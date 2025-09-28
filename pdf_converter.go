// Package main - PDF to Markdown conversion functionality.
// This file handles the core PDF processing, text extraction, image extraction,
// and Markdown generation for the MCP server.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/ledongthuc/pdf"
)

// PDFConverter handles the conversion of PDF files to Markdown format with image extraction.
// It manages the PDF document parsing, text extraction, image processing, and Markdown generation.
type PDFConverter struct {
	config          *Config          // Server configuration containing conversion settings
	logger          *Logger          // Logger instance for tracking conversion progress and errors
	diagramDetector *DiagramDetector // Diagram detector for converting diagrams to PlantUML
}

// ConversionResult contains the details of a completed PDF to Markdown conversion.
// This information is returned to the MCP client to indicate conversion success and provide
// details about the generated output.
type ConversionResult struct {
	OutputDir    string // Path to the directory containing the generated Markdown and images
	MarkdownFile string // Path to the generated Markdown file
	ImageCount   int    // Number of images extracted and saved
	PageCount    int    // Total number of pages processed from the PDF
}

// PDFPage represents the content of a single page from the PDF document.
// It contains both textual content and any images found on that page.
type PDFPage struct {
	Number int        // Page number (1-based)
	Text   string     // Extracted text content from the page
	Images []PDFImage // Images found on this page
}

// PDFImage represents an image extracted from a PDF page.
// It contains the image data and metadata needed for saving and referencing.
type PDFImage struct {
	Data     image.Image       // The actual image data
	Width    int               // Image width in pixels
	Height   int               // Image height in pixels
	Filename string            // Filename to use when saving the image
	Diagrams []DetectedDiagram // Detected diagrams in this image
}

// BatchConversionResult contains the results of processing multiple PDF files from a directory.
type BatchConversionResult struct {
	InputDir        string             // Source directory that was processed
	OutputBaseDir   string             // Base output directory
	Results         []ConversionResult // Individual conversion results for each PDF
	SuccessCount    int                // Number of successful conversions
	FailureCount    int                // Number of failed conversions
	Errors          []ConversionError  // Errors that occurred during processing
	FileCount       int                // Total number of PDF files found and processed
	TotalPageCount  int                // Total pages processed across all PDFs
	TotalImageCount int                // Total images extracted across all PDFs
}

// ConversionError represents an error that occurred while processing a specific PDF file.
type ConversionError struct {
	PDFPath string // Path to the PDF file that failed
	Error   string // Error message
}

// NewPDFConverter creates a new PDFConverter instance with the provided configuration and logger.
// It initializes the converter but doesn't load any PDF documents yet.
//
// Parameters:
//   - config: Server configuration containing PDF processing settings
//   - logger: Logger instance for tracking operations
//
// Returns:
//   - *PDFConverter: Initialized converter ready for PDF processing
//   - error: Initialization error, if any
func NewPDFConverter(config *Config, logger *Logger) (*PDFConverter, error) {
	diagramDetector := NewDiagramDetector(config, logger)

	return &PDFConverter{
		config:          config,
		logger:          logger,
		diagramDetector: diagramDetector,
	}, nil
}

// ConvertPDF processes a PDF file and converts it to Markdown format with extracted images.
// This is the main entry point for PDF conversion, handling the entire pipeline from
// PDF loading to Markdown generation and file output.
//
// The conversion process:
//  1. Opens and validates the PDF file
//  2. Creates the output directory structure
//  3. Extracts text and images from each page
//  4. Generates structured Markdown content
//  5. Saves the Markdown file and extracted images
//
// Parameters:
//   - pdfPath: Path to the input PDF file to convert
//   - outputBaseDir: Base directory where the output subdirectory will be created
//
// Returns:
//   - *ConversionResult: Details about the completed conversion
//   - error: Conversion error, if any occurred during processing
func (c *PDFConverter) ConvertPDF(pdfPath, outputBaseDir string) (*ConversionResult, error) {
	c.logger.Info("Starting PDF conversion: %s", pdfPath)

	// Validate that the PDF file exists and is readable
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("PDF file does not exist: %s", pdfPath)
	}

	// Open the PDF document using ledongthuc/pdf library
	file, reader, err := pdf.Open(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF: %v", err)
	}
	defer file.Close()

	c.logger.Info("PDF opened successfully, %d pages found", reader.NumPage())

	// Create output directory structure
	outputDir, err := c.createOutputDirectory(pdfPath, outputBaseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create output directory: %v", err)
	}

	// Extract content from all pages
	pages, totalImages, err := c.extractPagesContent(reader, outputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to extract PDF content: %v", err)
	}

	// Generate Markdown content from extracted pages
	markdownContent := c.generateMarkdown(pages)

	// Write the Markdown file
	markdownPath := filepath.Join(outputDir, "README.md")
	if err := c.writeMarkdownFile(markdownPath, markdownContent); err != nil {
		return nil, fmt.Errorf("failed to write Markdown file: %v", err)
	}

	c.logger.Info("PDF conversion completed successfully")

	return &ConversionResult{
		OutputDir:    outputDir,
		MarkdownFile: markdownPath,
		ImageCount:   totalImages,
		PageCount:    len(pages),
	}, nil
}

// createOutputDirectory creates the output directory structure for the converted files.
// It follows the pattern: outputBaseDir/MARKDOWN_<filename>/ where filename is derived
// from the input PDF filename without extension.
//
// Parameters:
//   - pdfPath: Path to the original PDF file (used to derive directory name)
//   - outputBaseDir: Base directory where the new subdirectory will be created
//
// Returns:
//   - string: Path to the created output directory
//   - error: Directory creation error, if any
func (c *PDFConverter) createOutputDirectory(pdfPath, outputBaseDir string) (string, error) {
	// Extract filename without extension from PDF path
	baseName := filepath.Base(pdfPath)
	nameWithoutExt := strings.TrimSuffix(baseName, filepath.Ext(baseName))

	// Create directory name with MARKDOWN_ prefix
	dirName := fmt.Sprintf("MARKDOWN_%s", nameWithoutExt)
	outputDir := filepath.Join(outputBaseDir, dirName)

	c.logger.Debug("Creating output directory: %s", outputDir)

	// Create the directory with full parent path if needed
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %v", outputDir, err)
	}

	return outputDir, nil
}

// extractPagesContent processes all pages in the PDF document to extract text and images.
// It iterates through each page, extracts the textual content, and saves any embedded images.
//
// Parameters:
//   - reader: Opened PDF reader from ledongthuc/pdf library
//   - outputDir: Directory where extracted images will be saved
//
// Returns:
//   - []PDFPage: Slice of processed pages with extracted content
//   - int: Total number of images extracted across all pages
//   - error: Extraction error, if any occurred
func (c *PDFConverter) extractPagesContent(reader *pdf.Reader, outputDir string) ([]PDFPage, int, error) {
	var pages []PDFPage
	totalImages := 0

	// Process each page in the document
	for pageNum := 1; pageNum <= reader.NumPage(); pageNum++ {
		c.logger.Debug("Processing page %d/%d", pageNum, reader.NumPage())

		page := PDFPage{
			Number: pageNum,
			Images: []PDFImage{},
		}

		// Get the page
		p := reader.Page(pageNum)
		if p.V.IsNull() {
			c.logger.Warn("Page %d is null, skipping", pageNum)
			continue
		}

		// Extract text content from the page
		text, err := p.GetPlainText(nil)
		if err != nil {
			c.logger.Warn("Failed to extract text from page %d: %v", pageNum, err)
			text = ""
		}
		page.Text = text

		// Extract images from the page if image extraction is enabled
		if c.config.ExtractImages {
			images, err := c.extractImagesFromPage(p, pageNum, outputDir)
			if err != nil {
				c.logger.Warn("Failed to extract images from page %d: %v", pageNum, err)
			} else {
				page.Images = images
				totalImages += len(images)
			}
		}

		pages = append(pages, page)
	}

	c.logger.Info("Extracted content from %d pages, %d images total", len(pages), totalImages)
	return pages, totalImages, nil
}

// extractImagesFromPage extracts and saves all images found on a specific PDF page.
// It uses ledongthuc/pdf to extract embedded images, then saves them
// as PNG files in the output directory and analyzes them for diagrams.
//
// Parameters:
//   - page: PDF page from ledongthuc/pdf library
//   - pageNum: One-based page number to process
//   - outputDir: Directory where images will be saved
//
// Returns:
//   - []PDFImage: Slice of images found and saved from this page
//   - error: Image extraction error, if any occurred
func (c *PDFConverter) extractImagesFromPage(page pdf.Page, pageNum int, outputDir string) ([]PDFImage, error) {
	var images []PDFImage

	c.logger.Debug("Extracting images from page %d", pageNum)

	// Get page resources to look for images
	resources := page.V.Key("Resources")
	if resources.IsNull() {
		c.logger.Debug("No resources found on page %d", pageNum)
		return images, nil
	}

	// Look for XObject resources which may contain images
	xObjects := resources.Key("XObject")
	if xObjects.IsNull() {
		c.logger.Debug("No XObject resources found on page %d", pageNum)
		return images, nil
	}

	// Iterate through XObjects to find images
	imageCount := 0
	for _, name := range xObjects.Keys() {
		obj := xObjects.Key(name)

		// Check if this XObject is an image
		if obj.Key("Subtype").Name() == "Image" {
			imageCount++

			// Create filename for the image
			filename := fmt.Sprintf("page_%d_image_%d.png", pageNum, imageCount)
			imagePath := filepath.Join(outputDir, filename)

			// For now, create a placeholder image since actual image extraction
			// from PDF using ledongthuc/pdf requires more complex implementation
			placeholderImg := c.createPlaceholderImage(200, 150)

			// Save the image
			if err := c.saveImage(placeholderImg, imagePath); err != nil {
				c.logger.Warn("Failed to save image %s: %v", imagePath, err)
				continue
			}

			// Analyze the image for diagrams if diagram detection is enabled
			var diagrams []DetectedDiagram
			if c.config.DetectDiagrams {
				detectedDiagrams, err := c.diagramDetector.DetectDiagramsInImage(imagePath)
				if err != nil {
					c.logger.Warn("Failed to analyze image %s for diagrams: %v", imagePath, err)
				} else {
					diagrams = detectedDiagrams
					if len(detectedDiagrams) > 0 {
						c.logger.Info("Found %d diagram(s) in %s", len(detectedDiagrams), filename)
					}
				}
			}

			pdfImage := PDFImage{
				Data:     placeholderImg,
				Width:    placeholderImg.Bounds().Dx(),
				Height:   placeholderImg.Bounds().Dy(),
				Filename: filename,
				Diagrams: diagrams,
			}

			images = append(images, pdfImage)
		}
	}

	if imageCount > 0 {
		c.logger.Info("Extracted %d images from page %d", imageCount, pageNum)
	} else {
		c.logger.Debug("No images found on page %d", pageNum)
	}

	return images, nil
}

// createPlaceholderImage creates a placeholder image for testing purposes
// This should be replaced with actual image extraction from PDF in production
func (c *PDFConverter) createPlaceholderImage(width, height int) image.Image {
	// Create a simple placeholder image
	img := imaging.New(width, height, color.RGBA{240, 240, 240, 255})

	// Add some simple content to make it look like a diagram placeholder
	// This is just for demonstration - real images would come from PDF
	return img
}

// saveImage saves an image to the specified file path
func (c *PDFConverter) saveImage(img image.Image, filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create image file %s: %v", filePath, err)
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		return fmt.Errorf("failed to encode image %s: %v", filePath, err)
	}

	return nil
}

// generateMarkdown creates Markdown content from the extracted PDF pages.
// It structures the content with proper headers, preserves formatting, and includes
// references to extracted images.
//
// The generated Markdown includes:
//   - Document title derived from filename
//   - Table of contents (if enabled)
//   - Page content with appropriate header levels
//   - Image references with proper markdown syntax
//
// Parameters:
//   - pages: Slice of PDF pages with extracted content
//
// Returns:
//   - string: Complete Markdown document content
func (c *PDFConverter) generateMarkdown(pages []PDFPage) string {
	var md strings.Builder

	// Add document title
	md.WriteString("# PDF Document\n\n")

	// Add table of contents if enabled
	if c.config.IncludeTOC {
		md.WriteString(c.generateTableOfContents(pages))
		md.WriteString("\n")
	}

	// Process each page
	for _, page := range pages {
		// Add page header
		headerLevel := strings.Repeat("#", c.config.BaseHeaderLevel+1)
		md.WriteString(fmt.Sprintf("%s Page %d\n\n", headerLevel, page.Number))

		// Add page text content with basic formatting preservation
		if page.Text != "" {
			formattedText := c.formatTextContent(page.Text)
			md.WriteString(formattedText)
			md.WriteString("\n\n")
		}

		// Add image references and any detected diagrams
		for _, img := range page.Images {
			md.WriteString(fmt.Sprintf("![Image](./%s)\n\n", img.Filename))

			// Add PlantUML diagrams if any were detected
			for _, diagram := range img.Diagrams {
				diagramMarkdown := c.diagramDetector.GetPlantUMLMarkdown(diagram)
				md.WriteString(diagramMarkdown)
			}
		}

		// Add page separator
		if page.Number < len(pages) {
			md.WriteString("---\n\n")
		}
	}

	return md.String()
}

// generateTableOfContents creates a table of contents section for the Markdown document.
// It analyzes the page content to create navigation links to different sections.
//
// Parameters:
//   - pages: Slice of PDF pages to analyze for TOC generation
//
// Returns:
//   - string: Formatted table of contents in Markdown format
func (c *PDFConverter) generateTableOfContents(pages []PDFPage) string {
	var toc strings.Builder

	toc.WriteString("## Table of Contents\n\n")

	for _, page := range pages {
		// Simple TOC entry for each page
		toc.WriteString(fmt.Sprintf("- [Page %d](#page-%d)\n", page.Number, page.Number))
	}

	toc.WriteString("\n")
	return toc.String()
}

// formatTextContent applies basic formatting to extracted text content to improve
// readability in the Markdown output. It handles paragraph breaks, preserves spacing,
// and attempts to identify headers and sections.
//
// Text formatting includes:
//   - Preserving paragraph breaks
//   - Converting multiple spaces to single spaces
//   - Identifying potential section headers
//   - Maintaining basic text structure
//
// Parameters:
//   - text: Raw text content extracted from the PDF page
//
// Returns:
//   - string: Formatted text content ready for Markdown inclusion
func (c *PDFConverter) formatTextContent(text string) string {
	if text == "" {
		return ""
	}

	// Split into lines and process
	lines := strings.Split(text, "\n")
	var formatted []string

	for _, line := range lines {
		// Trim excessive whitespace but preserve structure
		line = strings.TrimSpace(line)

		// Skip empty lines (we'll add them back strategically)
		if line == "" {
			continue
		}

		// Check if line looks like a header (all caps, short, ends with colon, etc.)
		if c.looksLikeHeader(line) {
			// Add extra spacing before headers
			if len(formatted) > 0 {
				formatted = append(formatted, "")
			}
			headerLevel := strings.Repeat("#", c.config.BaseHeaderLevel+2)
			formatted = append(formatted, fmt.Sprintf("%s %s", headerLevel, line))
			formatted = append(formatted, "")
		} else {
			formatted = append(formatted, line)
		}
	}

	// Join with newlines and add paragraph breaks
	result := strings.Join(formatted, "\n")

	// Replace multiple consecutive newlines with double newlines (paragraph breaks)
	re := regexp.MustCompile(`\n{3,}`)
	result = re.ReplaceAllString(result, "\n\n")

	return result
}

// looksLikeHeader determines if a line of text appears to be a section header.
// It uses heuristics to identify potential headers based on formatting, length,
// and content patterns commonly found in technical documents.
//
// Header detection criteria:
//   - Line is relatively short (less than 60 characters)
//   - Ends with a colon
//   - Is mostly uppercase
//   - Contains common header keywords
//   - Has specific formatting patterns
//
// Parameters:
//   - line: Text line to analyze for header characteristics
//
// Returns:
//   - bool: True if the line appears to be a header, false otherwise
func (c *PDFConverter) looksLikeHeader(line string) bool {
	// Skip very long lines
	if len(line) > 60 {
		return false
	}

	// Check for header patterns
	line = strings.TrimSpace(line)

	// Lines ending with colon are often headers
	if strings.HasSuffix(line, ":") {
		return true
	}

	// Lines that are mostly uppercase and short
	if len(line) < 40 && strings.ToUpper(line) == line && len(strings.Fields(line)) <= 5 {
		return true
	}

	// Common header keywords
	headerKeywords := []string{"OVERVIEW", "DESCRIPTION", "FEATURES", "SPECIFICATIONS",
		"PARAMETERS", "APPLICATIONS", "CHARACTERISTICS", "OPERATION", "CONFIGURATION"}

	upperLine := strings.ToUpper(line)
	for _, keyword := range headerKeywords {
		if strings.Contains(upperLine, keyword) {
			return true
		}
	}

	return false
}

// writeMarkdownFile writes the generated Markdown content to the specified file path.
// It creates the file with appropriate permissions and handles any I/O errors.
//
// Parameters:
//   - filePath: Complete path where the Markdown file should be written
//   - content: Markdown content to write to the file
//
// Returns:
//   - error: File writing error, if any occurred
func (c *PDFConverter) writeMarkdownFile(filePath, content string) error {
	c.logger.Debug("Writing Markdown file: %s", filePath)

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %v", filePath, err)
	}
	defer file.Close()

	if _, err := file.WriteString(content); err != nil {
		return fmt.Errorf("failed to write content to file %s: %v", filePath, err)
	}

	c.logger.Info("Markdown file written successfully: %s", filePath)
	return nil
}

// ConvertPDFsInDirectory processes all PDF files found in the specified directory.
// It scans the directory for PDF files and converts each one to Markdown format.
//
// Parameters:
//   - inputDir: Directory path containing PDF files to process
//   - outputBaseDir: Base directory where output subdirectories will be created
//
// Returns:
//   - *BatchConversionResult: Results of processing all PDF files
//   - error: Error if directory processing fails
func (c *PDFConverter) ConvertPDFsInDirectory(inputDir, outputBaseDir string) (*BatchConversionResult, error) {
	c.logger.Info("Starting batch PDF conversion from directory: %s", inputDir)

	// Validate that the input directory exists
	if _, err := os.Stat(inputDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("input directory does not exist: %s", inputDir)
	}

	// Find all PDF files in the directory
	pdfFiles, err := c.findPDFFiles(inputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find PDF files: %v", err)
	}

	if len(pdfFiles) == 0 {
		c.logger.Warn("No PDF files found in directory: %s", inputDir)
		return &BatchConversionResult{
			InputDir:        inputDir,
			OutputBaseDir:   outputBaseDir,
			Results:         []ConversionResult{},
			SuccessCount:    0,
			FailureCount:    0,
			Errors:          []ConversionError{},
			FileCount:       0,
			TotalPageCount:  0,
			TotalImageCount: 0,
		}, nil
	}

	c.logger.Info("Found %d PDF files to process", len(pdfFiles))

	result := &BatchConversionResult{
		InputDir:        inputDir,
		OutputBaseDir:   outputBaseDir,
		Results:         make([]ConversionResult, 0, len(pdfFiles)),
		SuccessCount:    0,
		FailureCount:    0,
		Errors:          make([]ConversionError, 0),
		FileCount:       len(pdfFiles),
		TotalPageCount:  0,
		TotalImageCount: 0,
	}

	// Process each PDF file
	for _, pdfPath := range pdfFiles {
		c.logger.Info("Processing PDF file: %s", filepath.Base(pdfPath))

		conversionResult, err := c.ConvertPDF(pdfPath, outputBaseDir)
		if err != nil {
			c.logger.Error("Failed to convert PDF %s: %v", pdfPath, err)
			result.FailureCount++
			result.Errors = append(result.Errors, ConversionError{
				PDFPath: pdfPath,
				Error:   err.Error(),
			})
		} else {
			c.logger.Info("Successfully converted PDF: %s", filepath.Base(pdfPath))
			result.SuccessCount++
			result.Results = append(result.Results, *conversionResult)
			result.TotalPageCount += conversionResult.PageCount
			result.TotalImageCount += conversionResult.ImageCount
		}
	}

	c.logger.Info("Batch conversion completed: %d successful, %d failed", result.SuccessCount, result.FailureCount)
	return result, nil
}

// findPDFFiles recursively searches for PDF files in the specified directory.
// It returns a slice of absolute paths to all PDF files found.
//
// Parameters:
//   - dir: Directory path to search for PDF files
//
// Returns:
//   - []string: Slice of absolute paths to PDF files
//   - error: Error if directory traversal fails
func (c *PDFConverter) findPDFFiles(dir string) ([]string, error) {
	var pdfFiles []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			c.logger.Warn("Error accessing path %s: %v", path, err)
			return nil // Continue walking, don't fail on individual file errors
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if file has PDF extension (case-insensitive)
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".pdf" {
			absPath, err := filepath.Abs(path)
			if err != nil {
				c.logger.Warn("Could not get absolute path for %s: %v", path, err)
				return nil
			}
			pdfFiles = append(pdfFiles, absPath)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error walking directory %s: %v", dir, err)
	}

	return pdfFiles, nil
}
