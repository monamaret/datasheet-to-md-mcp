// Package main implements an MCP (Model Context Protocol) server that converts PDF files to Markdown format.
// The server extracts text content, images, and formatting from PDF files and generates structured Markdown
// output with proper headers, sections, and embedded images saved as PNG files.
package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/joho/godotenv"

	"datasheet-to-md-mcp/config"
	"datasheet-to-md-mcp/logger"
	"datasheet-to-md-mcp/mcp"
	"datasheet-to-md-mcp/pdfconv"
)

// MCPServer represents the main MCP server instance that handles PDF to Markdown conversion
type MCPServer struct {
	config    *config.Config
	converter *pdfconv.PDFConverter
	logger    *logger.Logger
}

// main is the entry point of the PDF to Markdown MCP server.
// It initializes the configuration, sets up logging, creates the server instance,
// and starts the MCP protocol handler to listen for incoming requests.
func main() {
	// Load environment variables from pdf_md_mcp.env file if it exists
	if err := godotenv.Load("pdf_md_mcp.env"); err != nil {
		// If the file doesn't exist, continue with system environment variables
		log.Printf("Warning: Could not load pdf_md_mcp.env file: %v", err)
	}

	// Initialize configuration from environment variables
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger with configured log level
	logr := logger.NewLogger(cfg.LogLevel)

	// Create PDF converter instance with the loaded configuration
	converter, err := pdfconv.NewPDFConverter(cfg, logr)
	if err != nil {
		logr.Fatal("Failed to create PDF converter: %v", err)
	}

	// Create the main MCP server instance
	server := &MCPServer{
		config:    cfg,
		converter: converter,
		logger:    logr,
	}

	logr.Info("Starting PDF to Markdown MCP server v%s", cfg.ServerVersion)

	// Start the MCP protocol handler based on the configured transport
	if err := server.Start(); err != nil {
		logr.Fatal("Server failed to start: %v", err)
	}
}

// Start initializes and starts the MCP server based on the configured transport method.
// It handles stdio transport only, processing incoming MCP messages.
func (s *MCPServer) Start() error {
	s.logger.Info("MCP Server starting with transport: %s", s.config.Transport)

	switch s.config.Transport {
	case "stdio":
		return s.startStdioTransport()
	default:
		return fmt.Errorf("unsupported transport type: %s (only stdio is supported)", s.config.Transport)
	}
}

// startStdioTransport starts the MCP server using standard input/output communication.
// This is the most common transport method for MCP servers, allowing them to be
// integrated with AI coding assistants and other tools that spawn subprocess servers.
func (s *MCPServer) startStdioTransport() error {
	s.logger.Info("Starting STDIO transport")

	// Create MCP message handler
	handler := mcp.NewMCPHandler(s.converter, s.logger)

	// Process messages from stdin and write responses to stdout
	return handler.HandleStdio()
}

// MCPRequest represents an incoming MCP protocol message with method and parameters
type MCPRequest struct {
	ID     interface{}            `json:"id"`
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// MCPResponse represents an outgoing MCP protocol response with result or error
type MCPResponse struct {
	ID     interface{}            `json:"id"`
	Result map[string]interface{} `json:"result,omitempty"`
	Error  *MCPError              `json:"error,omitempty"`
}

// MCPError represents an error response in the MCP protocol
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// handleMCPRequest processes an incoming MCP request and returns the appropriate response.
// It handles different MCP methods like "initialize", "tools/list", "tools/call", etc.
func (s *MCPServer) handleMCPRequest(request *MCPRequest) *MCPResponse {
	s.logger.Debug("Handling MCP request: %s", request.Method)

	response := &MCPResponse{ID: request.ID}

	switch request.Method {
	case "initialize":
		response.Result = s.handleInitialize(request.Params)
	case "tools/list":
		response.Result = s.handleToolsList()
	case "tools/call":
		result, err := s.handleToolsCall(request.Params)
		if err != nil {
			response.Error = &MCPError{Code: -32603, Message: err.Error()}
		} else {
			response.Result = result
		}
	default:
		response.Error = &MCPError{Code: -32601, Message: fmt.Sprintf("Method not found: %s", request.Method)}
	}

	return response
}

// handleInitialize processes the MCP initialize request and returns server capabilities.
func (s *MCPServer) handleInitialize(params map[string]interface{}) map[string]interface{} {
	s.logger.Info("Client initializing connection")

	return map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
		"serverInfo":      map[string]interface{}{"name": s.config.ServerName, "version": s.config.ServerVersion},
	}
}

// handleToolsList returns the list of available tools that this MCP server provides.
func (s *MCPServer) handleToolsList() map[string]interface{} {
	return map[string]interface{}{
		"tools": []map[string]interface{}{
			{
				"name":        "convert_pdf_to_markdown",
				"description": "Convert a single PDF file to Markdown format with extracted images",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"pdf_path":   map[string]interface{}{"type": "string", "description": "Path to the input PDF file"},
						"output_dir": map[string]interface{}{"type": "string", "description": "Base output directory (optional, uses config default if not provided)"},
					},
					"required": []string{"pdf_path"},
				},
			},
			{
				"name":        "convert_pdfs_in_directory",
				"description": "Convert all PDF files in a directory to Markdown format with extracted images",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"input_dir":  map[string]interface{}{"type": "string", "description": "Directory path containing PDF files to process"},
						"output_dir": map[string]interface{}{"type": "string", "description": "Base output directory (optional, uses config default if not provided)"},
					},
					"required": []string{"input_dir"},
				},
			},
		},
	}
}

// handleToolsCall executes a tool call request, specifically handling PDF to Markdown conversion.
func (s *MCPServer) handleToolsCall(params map[string]interface{}) (map[string]interface{}, error) {
	toolName, ok := params["name"].(string)
	if !ok {
		return nil, fmt.Errorf("missing tool name")
	}
	if toolName != "convert_pdf_to_markdown" && toolName != "convert_pdfs_in_directory" {
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
	arguments, ok := params["arguments"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing tool arguments")
	}

	switch toolName {
	case "convert_pdf_to_markdown":
		pdfPath, ok := arguments["pdf_path"].(string)
		if !ok {
			return nil, fmt.Errorf("missing required parameter: pdf_path")
		}
		outputDir := s.config.OutputBaseDir
		if providedDir, exists := arguments["output_dir"].(string); exists {
			outputDir = providedDir
		}
		s.logger.Info("Converting PDF to Markdown: %s", pdfPath)
		result, err := s.converter.ConvertPDF(pdfPath, outputDir)
		if err != nil {
			return nil, fmt.Errorf("conversion failed: %v", err)
		}
		return map[string]interface{}{"content": []map[string]interface{}{{"type": "text", "text": fmt.Sprintf("Successfully converted PDF to Markdown:\n\nOutput directory: %s\nMarkdown file: %s\nExtracted images: %d\nTotal pages processed: %d", result.OutputDir, result.MarkdownFile, result.ImageCount, result.PageCount)}}}, nil
	case "convert_pdfs_in_directory":
		inputDir, ok := arguments["input_dir"].(string)
		if !ok {
			return nil, fmt.Errorf("missing required parameter: input_dir")
		}
		outputDir := s.config.OutputBaseDir
		if providedDir, exists := arguments["output_dir"].(string); exists {
			outputDir = providedDir
		}
		s.logger.Info("Converting PDFs in directory to Markdown: %s", inputDir)
		batchResult, err := s.converter.ConvertPDFsInDirectory(inputDir, outputDir)
		if err != nil {
			return nil, fmt.Errorf("directory conversion failed: %v", err)
		}
		var summary strings.Builder
		summary.WriteString(fmt.Sprintf("Batch PDF Conversion Completed\n\n"))
		summary.WriteString(fmt.Sprintf("Input Directory: %s\n", batchResult.InputDir))
		summary.WriteString(fmt.Sprintf("Output Directory: %s\n", batchResult.OutputBaseDir))
		summary.WriteString(fmt.Sprintf("PDF Files Found: %d\n", batchResult.FileCount))
		summary.WriteString(fmt.Sprintf("Successfully Converted: %d\n", batchResult.SuccessCount))
		summary.WriteString(fmt.Sprintf("Failed Conversions: %d\n", batchResult.FailureCount))
		summary.WriteString(fmt.Sprintf("Total Pages Processed: %d\n", batchResult.TotalPageCount))
		summary.WriteString(fmt.Sprintf("Total Images Extracted: %d\n\n", batchResult.TotalImageCount))
		if len(batchResult.Results) > 0 {
			summary.WriteString("Converted Files:\n")
			for _, result := range batchResult.Results {
				summary.WriteString(fmt.Sprintf("- %s: %d pages, %d images\n", result.MarkdownFile, result.PageCount, result.ImageCount))
			}
		}
		if batchResult.FailureCount > 0 {
			summary.WriteString("\nErrors:\n")
			for _, err := range batchResult.Errors {
				summary.WriteString(fmt.Sprintf("- %s: %s\n", err.PDFPath, err.Error))
			}
		}
		return map[string]interface{}{"content": []map[string]interface{}{{"type": "text", "text": summary.String()}}}, nil
	}
	return nil, fmt.Errorf("unexpected tool name: %s", toolName)
}
