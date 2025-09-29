// Package pdfconv - PDF to Markdown conversion functionality.
// This file handles the core PDF processing, text extraction, image extraction,
// and Markdown generation for the MCP server.
package pdfconv

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/ledongthuc/pdf"

	"datasheet-to-md-mcp/config"
	"datasheet-to-md-mcp/logger"
	"datasheet-to-md-mcp/uml"
)

// Constants for image processing limits
const (
	MaxImageWidth       = 10000   // Maximum allowed image width in pixels
	MaxImageHeight      = 10000   // Maximum allowed image height in pixels
	MaxImagePixels      = 4000000 // Maximum total pixels (width * height)
	DefaultImageWidth   = 200     // Default width for placeholder images
	DefaultImageHeight  = 150     // Default height for placeholder images
	DefaultHeaderLength = 60      // Maximum length for header detection
	MaxHeaderWords      = 5       // Maximum words in a detected header
	ShortHeaderLength   = 40      // Length threshold for short headers
)

// PDFConverter handles the conversion of PDF files to Markdown format with image extraction.
// It manages the PDF document parsing, text extraction, image processing, and Markdown generation.
type PDFConverter struct {
	config          *config.Config       // Server configuration containing conversion settings
	logger          *logger.Logger       // Logger instance for tracking conversion progress and errors
	diagramDetector *uml.DiagramDetector // Diagram detector for converting diagrams to PlantUML
}

// Config returns the underlying config for convenience
func (c *PDFConverter) Config() *config.Config { return c.config }

// ConversionResult contains the details of a completed PDF to Markdown conversion.
type ConversionResult struct {
	OutputDir    string
	MarkdownFile string
	ImageCount   int
	PageCount    int
}

// PDFPage represents the content of a single page from the PDF document.
type PDFPage struct {
	Number int
	Text   string
	Images []PDFImage
}

// PDFImage represents an image extracted from a PDF page.
type PDFImage struct {
	Data     image.Image
	Width    int
	Height   int
	Filename string
	Diagrams []uml.DetectedDiagram
}

// BatchConversionResult contains the results of processing multiple PDF files from a directory.
type BatchConversionResult struct {
	InputDir        string
	OutputBaseDir   string
	Results         []ConversionResult
	SuccessCount    int
	FailureCount    int
	Errors          []ConversionError
	FileCount       int
	TotalPageCount  int
	TotalImageCount int
}

// ConversionError represents an error that occurred while processing a specific PDF file.
type ConversionError struct {
	PDFPath string
	Error   string
}

// NewPDFConverter creates a new PDFConverter instance with the provided configuration and logger.
func NewPDFConverter(cfg *config.Config, log *logger.Logger) (*PDFConverter, error) {
	diagramDetector := uml.NewDiagramDetector(cfg, log)
	return &PDFConverter{config: cfg, logger: log, diagramDetector: diagramDetector}, nil
}

// ConvertPDF processes a PDF file and converts it to Markdown format with extracted images.
func (c *PDFConverter) ConvertPDF(pdfPath, outputBaseDir string) (*ConversionResult, error) {
	c.logger.Info("Starting PDF conversion: %s", pdfPath)

	// Validate input parameters
	if strings.TrimSpace(pdfPath) == "" {
		return nil, fmt.Errorf("PDF path cannot be empty")
	}
	if strings.TrimSpace(outputBaseDir) == "" {
		return nil, fmt.Errorf("output base directory cannot be empty")
	}

	// Clean and validate paths
	pdfPath = filepath.Clean(pdfPath)
	outputBaseDir = filepath.Clean(outputBaseDir)

	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("PDF file does not exist: %s", pdfPath)
	}

	// Check if it's actually a file, not a directory
	if fileInfo, err := os.Stat(pdfPath); err == nil && fileInfo.IsDir() {
		return nil, fmt.Errorf("path is a directory, not a file: %s", pdfPath)
	}

	// Validate file extension
	if !strings.HasSuffix(strings.ToLower(pdfPath), ".pdf") {
		return nil, fmt.Errorf("file does not have a .pdf extension: %s", pdfPath)
	}

	file, reader, err := pdf.Open(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF: %v", err)
	}
	defer file.Close()

	c.logger.Info("PDF opened successfully, %d pages found", reader.NumPage())

	outputDir, err := c.createOutputDirectory(pdfPath, outputBaseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create output directory: %v", err)
	}

	pages, totalImages, err := c.extractPagesContent(reader, outputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to extract PDF content: %v", err)
	}

	markdownContent := c.generateMarkdown(pages)

	markdownPath := filepath.Join(outputDir, "README.md")
	if err := c.writeMarkdownFile(markdownPath, markdownContent); err != nil {
		return nil, fmt.Errorf("failed to write Markdown file: %v", err)
	}

	c.logger.Info("PDF conversion completed successfully")

	return &ConversionResult{OutputDir: outputDir, MarkdownFile: markdownPath, ImageCount: totalImages, PageCount: len(pages)}, nil
}

func (c *PDFConverter) createOutputDirectory(pdfPath, outputBaseDir string) (string, error) {
	baseName := filepath.Base(pdfPath)
	nameWithoutExt := strings.TrimSuffix(baseName, filepath.Ext(baseName))
	dirName := fmt.Sprintf("MARKDOWN_%s", nameWithoutExt)
	outputDir := filepath.Join(outputBaseDir, dirName)
	c.logger.Debug("Creating output directory: %s", outputDir)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %v", outputDir, err)
	}
	return outputDir, nil
}

func (c *PDFConverter) extractPagesContent(reader *pdf.Reader, outputDir string) ([]PDFPage, int, error) {
	var pages []PDFPage
	totalImages := 0
	for pageNum := 1; pageNum <= reader.NumPage(); pageNum++ {
		c.logger.Debug("Processing page %d/%d", pageNum, reader.NumPage())
		page := PDFPage{Number: pageNum, Images: []PDFImage{}}
		p := reader.Page(pageNum)
		if p.V.IsNull() {
			c.logger.Warn("Page %d is null, skipping", pageNum)
			continue
		}
		text, err := p.GetPlainText(nil)
		if err != nil {
			c.logger.Warn("Failed to extract text from page %d: %v", pageNum, err)
			text = ""
		}
		page.Text = text

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

func (c *PDFConverter) extractImagesFromPage(page pdf.Page, pageNum int, outputDir string) ([]PDFImage, error) {
	var images []PDFImage
	c.logger.Debug("Extracting images from page %d", pageNum)

	// Safely check for resources
	resources := page.V.Key("Resources")
	if resources.IsNull() {
		c.logger.Debug("No resources found on page %d", pageNum)
		return images, nil
	}

	xObjects := resources.Key("XObject")
	if xObjects.IsNull() {
		c.logger.Debug("No XObject resources found on page %d", pageNum)
		return images, nil
	}

	imageCount := 0
	objKeys := xObjects.Keys()

	for _, name := range objKeys {
		// Skip if name is empty or invalid
		if name == "" {
			continue
		}

		// Process each image object with error recovery
		func() {
			defer func() {
				if r := recover(); r != nil {
					c.logger.Warn("Recovered from panic while processing image %s on page %d: %v", name, pageNum, r)
				}
			}()

			obj := xObjects.Key(name)
			if obj.IsNull() || obj.Key("Subtype").Name() != "Image" {
				return // Exit anonymous function only - this is correct
			}

			imageCount++
			filename := fmt.Sprintf("page_%d_image_%d.png", pageNum, imageCount)
			imagePath := filepath.Join(outputDir, filename)

			// Extract actual image data from PDF
			img, err := c.extractImageFromXObject(obj)
			if err != nil {
				c.logger.Warn("Failed to extract image data for %s: %v, using placeholder", filename, err)
				img = c.createPlaceholderImage(DefaultImageWidth, DefaultImageHeight)
			}

			if err := c.saveImage(img, imagePath); err != nil {
				c.logger.Warn("Failed to save image %s: %v", imagePath, err)
				return // Exit anonymous function only - this is correct, continue processing other images
			}

			var diagrams []uml.DetectedDiagram
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
				Data:     img,
				Width:    img.Bounds().Dx(),
				Height:   img.Bounds().Dy(),
				Filename: filename,
				Diagrams: diagrams,
			}
			images = append(images, pdfImage)
		}()
	}
	if imageCount > 0 {
		c.logger.Info("Extracted %d images from page %d", imageCount, pageNum)
	} else {
		c.logger.Debug("No images found on page %d", pageNum)
	}
	return images, nil
}

func (c *PDFConverter) extractImageFromXObject(obj pdf.Value) (image.Image, error) {
	// Get the image stream data
	reader := obj.Reader()
	if reader == nil {
		return nil, fmt.Errorf("failed to get reader from XObject")
	}
	defer reader.Close()

	// Read the stream data
	stream, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read stream from XObject: %v", err)
	}

	// Get image properties with safe defaults
	width := int(obj.Key("Width").Int64())
	height := int(obj.Key("Height").Int64())

	// Validate dimensions
	if width <= 0 || height <= 0 || width > MaxImageWidth || height > MaxImageHeight {
		c.logger.Warn("Invalid or excessive image dimensions: %dx%d, using placeholder", width, height)
		return c.createPlaceholderImage(DefaultImageWidth, DefaultImageHeight), nil
	}

	colorSpace := obj.Key("ColorSpace").Name()
	bitsPerComponent := int(obj.Key("BitsPerComponent").Int64())

	// Validate bits per component
	if bitsPerComponent <= 0 {
		bitsPerComponent = 8 // Default fallback
	}

	c.logger.Debug("Extracting image: %dx%d, colorspace: %s, bits: %d", width, height, colorSpace, bitsPerComponent)

	// Check for image filters
	filter := obj.Key("Filter")
	if !filter.IsNull() {
		filterName := filter.Name()
		switch filterName {
		case "DCTDecode":
			// This is a JPEG image, try to decode it directly
			img, err := jpeg.Decode(bytes.NewReader(stream))
			if err != nil {
				c.logger.Debug("Failed to decode JPEG directly: %v", err)
				return c.decodeRawImageData(stream, width, height, colorSpace, bitsPerComponent)
			}
			return img, nil
		case "FlateDecode":
			// This is compressed data, the stream should already be decompressed by the PDF library
			return c.decodeRawImageData(stream, width, height, colorSpace, bitsPerComponent)
		case "CCITTFaxDecode":
			// CCITT Fax encoding, typically used for black and white images
			c.logger.Debug("CCITT Fax encoded image detected, using placeholder")
			return c.createPlaceholderImage(width, height), nil
		default:
			c.logger.Debug("Unknown filter %s, attempting raw decode", filterName)
			return c.decodeRawImageData(stream, width, height, colorSpace, bitsPerComponent)
		}
	}

	// No filter specified, decode as raw image data
	return c.decodeRawImageData(stream, width, height, colorSpace, bitsPerComponent)
}

func (c *PDFConverter) decodeRawImageData(data []byte, width, height int, colorSpace string, bitsPerComponent int) (image.Image, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid image dimensions: %dx%d", width, height)
	}

	// Prevent excessive memory allocation
	if width*height > MaxImagePixels {
		c.logger.Warn("Image too large (%dx%d = %d pixels), using placeholder", width, height, width*height)
		return c.createPlaceholderImage(width, height), nil
	}

	// Handle ColorSpace as an array (e.g., [/ICCBased ...])
	if colorSpace == "" {
		colorSpace = "DeviceRGB" // Default fallback
	}

	// Create an RGBA image
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Calculate bytes per pixel based on color space and bits per component
	var bytesPerPixel int
	switch colorSpace {
	case "DeviceRGB", "RGB":
		bytesPerPixel = 3
	case "DeviceGray", "Gray":
		bytesPerPixel = 1
	case "DeviceCMYK", "CMYK":
		bytesPerPixel = 4
	default:
		// Handle ICCBased and other complex color spaces
		if strings.Contains(colorSpace, "ICCBased") || strings.Contains(colorSpace, "CalRGB") {
			c.logger.Debug("ICC/CalRGB color space detected, assuming RGB")
			bytesPerPixel = 3
		} else {
			c.logger.Debug("Unknown color space %s, assuming RGB", colorSpace)
			bytesPerPixel = 3
		}
	}

	// Handle different bits per component
	if bitsPerComponent != 8 && bitsPerComponent != 1 {
		c.logger.Debug("Unsupported bits per component: %d, using placeholder", bitsPerComponent)
		return c.createPlaceholderImage(width, height), nil
	}

	// Special handling for 1-bit images (black and white)
	if bitsPerComponent == 1 {
		return c.decode1BitImage(data, width, height), nil
	}

	expectedDataSize := width * height * bytesPerPixel
	if len(data) < expectedDataSize {
		c.logger.Debug("Insufficient image data: got %d bytes, expected %d, using smaller dimensions or placeholder", len(data), expectedDataSize)
		// Try to extract what we can
		if len(data) < width*height {
			return c.createPlaceholderImage(width, height), nil
		}
	}

	// Convert pixel data based on color space
	dataIndex := 0
	for y := 0; y < height && dataIndex < len(data); y++ {
		for x := 0; x < width && dataIndex < len(data); x++ {
			var r, g, b uint8 = 0, 0, 0

			switch colorSpace {
			case "DeviceGray", "Gray":
				if dataIndex < len(data) {
					gray := data[dataIndex]
					r, g, b = gray, gray, gray
					dataIndex++
				}
			case "DeviceRGB", "RGB":
				if dataIndex+2 < len(data) {
					r = data[dataIndex]
					g = data[dataIndex+1]
					b = data[dataIndex+2]
					dataIndex += 3
				} else {
					// Incomplete pixel data
					break
				}
			case "DeviceCMYK", "CMYK":
				if dataIndex+3 < len(data) {
					// Simple CMYK to RGB conversion
					cVal := float64(data[dataIndex]) / 255.0
					mVal := float64(data[dataIndex+1]) / 255.0
					yVal := float64(data[dataIndex+2]) / 255.0
					kVal := float64(data[dataIndex+3]) / 255.0

					r = uint8(255 * (1 - cVal) * (1 - kVal))
					g = uint8(255 * (1 - mVal) * (1 - kVal))
					b = uint8(255 * (1 - yVal) * (1 - kVal))
					dataIndex += 4
				} else {
					// Incomplete pixel data
					break
				}
			default:
				// Assume RGB for unknown color spaces
				if dataIndex+2 < len(data) {
					r = data[dataIndex]
					g = data[dataIndex+1]
					b = data[dataIndex+2]
					dataIndex += 3
				} else if dataIndex < len(data) {
					// Single channel fallback
					gray := data[dataIndex]
					r, g, b = gray, gray, gray
					dataIndex++
				}
			}

			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	return img, nil
}

func (c *PDFConverter) decode1BitImage(data []byte, width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	bitIndex := 0
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			byteIndex := bitIndex / 8
			bitPosition := 7 - (bitIndex % 8)

			if byteIndex >= len(data) {
				return img
			}

			bit := (data[byteIndex] >> bitPosition) & 1
			var colorVal uint8
			if bit == 1 {
				colorVal = 0 // Black for 1
			} else {
				colorVal = 255 // White for 0
			}

			img.Set(x, y, color.RGBA{R: colorVal, G: colorVal, B: colorVal, A: 255})
			bitIndex++
		}

		// Align to byte boundary at end of each row if needed
		if bitIndex%8 != 0 {
			bitIndex = ((bitIndex / 8) + 1) * 8
		}
	}

	return img
}

func (c *PDFConverter) createPlaceholderImage(width, height int) image.Image {
	img := imaging.New(width, height, color.RGBA{240, 240, 240, 255})
	return img
}

func (c *PDFConverter) saveImage(img image.Image, filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create image file %s: %v", filePath, err)
	}
	defer file.Close()

	// Always save as PNG for consistency and quality
	if err := png.Encode(file, img); err != nil {
		return fmt.Errorf("failed to encode image %s: %v", filePath, err)
	}

	c.logger.Debug("Successfully saved image: %s", filePath)
	return nil
}

func (c *PDFConverter) generateMarkdown(pages []PDFPage) string {
	var md strings.Builder
	md.WriteString("# PDF Document\n\n")
	if c.config.IncludeTOC {
		md.WriteString(c.generateTableOfContents(pages))
		md.WriteString("\n")
	}
	for _, page := range pages {
		headerLevel := strings.Repeat("#", c.config.BaseHeaderLevel+1)
		md.WriteString(fmt.Sprintf("%s Page %d\n\n", headerLevel, page.Number))
		if page.Text != "" {
			formattedText := c.formatTextContent(page.Text)
			md.WriteString(formattedText)
			md.WriteString("\n\n")
		}
		for _, img := range page.Images {
			md.WriteString(fmt.Sprintf("![Image](./%s)\n\n", img.Filename))
			for _, diagram := range img.Diagrams {
				diagramMarkdown := c.diagramDetector.GetPlantUMLMarkdown(diagram)
				md.WriteString(diagramMarkdown)
			}
		}
		if page.Number < len(pages) {
			md.WriteString("---\n\n")
		}
	}
	return md.String()
}

func (c *PDFConverter) generateTableOfContents(pages []PDFPage) string {
	var toc strings.Builder
	toc.WriteString("## Table of Contents\n\n")
	for _, page := range pages {
		toc.WriteString(fmt.Sprintf("- [Page %d](#page-%d)\n", page.Number, page.Number))
	}
	toc.WriteString("\n")
	return toc.String()
}

func (c *PDFConverter) formatTextContent(text string) string {
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	var formatted []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if c.looksLikeHeader(line) {
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
	result := strings.Join(formatted, "\n")
	re := regexp.MustCompile(`\n{3,}`)
	result = re.ReplaceAllString(result, "\n\n")
	return result
}

func (c *PDFConverter) looksLikeHeader(line string) bool {
	if len(line) > DefaultHeaderLength {
		return false
	}
	line = strings.TrimSpace(line)
	if strings.HasSuffix(line, ":") {
		return true
	}
	if len(line) < ShortHeaderLength && strings.ToUpper(line) == line && len(strings.Fields(line)) <= MaxHeaderWords {
		return true
	}
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

func (c *PDFConverter) ConvertPDFsInDirectory(inputDir, outputBaseDir string) (*BatchConversionResult, error) {
	c.logger.Info("Starting batch PDF conversion from directory: %s", inputDir)
	if _, err := os.Stat(inputDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("input directory does not exist: %s", inputDir)
	}
	pdfFiles, err := c.findPDFFiles(inputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find PDF files: %v", err)
	}
	if len(pdfFiles) == 0 {
		c.logger.Warn("No PDF files found in directory: %s", inputDir)
		return &BatchConversionResult{InputDir: inputDir, OutputBaseDir: outputBaseDir, Results: []ConversionResult{}, SuccessCount: 0, FailureCount: 0, Errors: []ConversionError{}, FileCount: 0, TotalPageCount: 0, TotalImageCount: 0}, nil
	}
	c.logger.Info("Found %d PDF files to process", len(pdfFiles))
	result := &BatchConversionResult{
		InputDir:      inputDir,
		OutputBaseDir: outputBaseDir,
		Results:       make([]ConversionResult, 0, len(pdfFiles)),
		Errors:        make([]ConversionError, 0), // Pre-allocate to avoid nil slice issues
	}

	for i, pdfPath := range pdfFiles {
		c.logger.Info("Processing PDF file (%d/%d): %s", i+1, len(pdfFiles), filepath.Base(pdfPath))
		conversionResult, err := c.ConvertPDF(pdfPath, outputBaseDir)
		if err != nil {
			c.logger.Error("Failed to convert PDF %s: %v", pdfPath, err)
			result.FailureCount++
			result.Errors = append(result.Errors, ConversionError{PDFPath: pdfPath, Error: err.Error()})
		} else {
			c.logger.Info("Successfully converted PDF: %s", filepath.Base(pdfPath))
			result.SuccessCount++
			result.Results = append(result.Results, *conversionResult)
			result.TotalPageCount += conversionResult.PageCount
			result.TotalImageCount += conversionResult.ImageCount
		}
	}
	result.FileCount = len(pdfFiles)
	c.logger.Info("Batch conversion completed: %d successful, %d failed", result.SuccessCount, result.FailureCount)
	return result, nil
}

func (c *PDFConverter) findPDFFiles(dir string) ([]string, error) {
	var pdfFiles []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			c.logger.Warn("Error accessing path %s: %v", path, err)
			return nil
		}
		if info.IsDir() {
			return nil
		}
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
