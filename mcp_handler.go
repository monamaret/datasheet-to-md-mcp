// Package main - MCP (Model Context Protocol) message handling and transport layer.
// This file implements the MCP protocol communication layer, handling stdio transport
// and processing MCP messages.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MCPHandler manages MCP protocol communication and message processing.
// It handles stdio transport and routes MCP messages to appropriate handlers.
type MCPHandler struct {
	converter *PDFConverter // PDF conversion engine for processing tool calls
	logger    *Logger       // Logger for tracking MCP operations
}

// MCPMessage represents a generic MCP protocol message that can be either a request or response.
// The JSON-RPC 2.0 format is used for all MCP communications.
type MCPMessage struct {
	JSONRPC string                 `json:"jsonrpc"`          // Always "2.0" for JSON-RPC 2.0
	ID      interface{}            `json:"id,omitempty"`     // Request/response identifier
	Method  string                 `json:"method,omitempty"` // Method name for requests
	Params  map[string]interface{} `json:"params,omitempty"` // Method parameters for requests
	Result  map[string]interface{} `json:"result,omitempty"` // Result data for responses
	Error   *MCPError              `json:"error,omitempty"`  // Error information for failed responses
}

// NewMCPHandler creates a new MCP message handler with the specified converter and logger.
// The handler is responsible for processing MCP protocol messages and coordinating
// with the PDF converter to fulfill tool requests.
//
// Parameters:
//   - converter: PDF converter instance for handling conversion requests
//   - logger: Logger instance for tracking MCP operations
//
// Returns:
//   - *MCPHandler: Initialized MCP handler ready to process messages
func NewMCPHandler(converter *PDFConverter, logger *Logger) *MCPHandler {
	return &MCPHandler{
		converter: converter,
		logger:    logger,
	}
}

// HandleStdio processes MCP messages using standard input/output communication.
// This is the most common transport method for MCP servers, allowing integration
// with AI coding assistants and other tools that spawn subprocess servers.
//
// The method reads JSON-RPC messages from stdin, processes them, and writes
// responses to stdout. Each message should be on a separate line.
//
// Returns:
//   - error: Communication or processing error, if any occurred
func (h *MCPHandler) HandleStdio() error {
	h.logger.Debug("Starting STDIO message handling")

	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	// Process messages line by line from stdin
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue // Skip empty lines
		}

		h.logger.Debug("Received message: %s", line)

		// Parse the incoming JSON-RPC message
		var message MCPMessage
		if err := json.Unmarshal([]byte(line), &message); err != nil {
			h.logger.Error("Failed to parse message: %v", err)

			// Send error response for malformed JSON
			errorResponse := MCPMessage{
				JSONRPC: "2.0",
				Error: &MCPError{
					Code:    -32700, // Parse error
					Message: "Parse error",
					Data:    err.Error(),
				},
			}
			encoder.Encode(errorResponse)
			continue
		}

		// Process the message and generate response
		response := h.processMessage(&message)

		// Send response back via stdout
		if err := encoder.Encode(response); err != nil {
			h.logger.Error("Failed to send response: %v", err)
		}
	}

	// Check for scanner errors
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading from stdin: %v", err)
	}

	return nil
}

// processMessage handles the core MCP message processing logic.
// It routes messages to appropriate handlers based on the method name and
// generates proper JSON-RPC responses.
//
// Supported MCP methods:
//   - initialize: Server capability negotiation
//   - tools/list: List available tools
//   - tools/call: Execute a tool (PDF conversion)
//
// Parameters:
//   - message: Parsed MCP message to process
//
// Returns:
//   - MCPMessage: Response message to send back to the client
func (h *MCPHandler) processMessage(message *MCPMessage) MCPMessage {
	// Ensure JSONRPC version is set
	if message.JSONRPC == "" {
		message.JSONRPC = "2.0"
	}

	response := MCPMessage{
		JSONRPC: "2.0",
		ID:      message.ID,
	}

	// Route message based on method
	switch message.Method {
	case "initialize":
		response.Result = h.handleInitialize(message.Params)
		h.logger.Info("Client initialized")

	case "tools/list":
		response.Result = h.handleToolsList()
		h.logger.Debug("Sent tools list")

	case "tools/call":
		result, err := h.handleToolsCall(message.Params)
		if err != nil {
			response.Error = &MCPError{
				Code:    -32603, // Internal error
				Message: err.Error(),
			}
			h.logger.Error("Tool call failed: %v", err)
		} else {
			response.Result = result
			h.logger.Info("Tool call completed successfully")
		}

	case "notifications/initialized":
		// Handle initialized notification (no response needed)
		h.logger.Debug("Received initialized notification")
		return MCPMessage{} // Return empty message for notifications

	default:
		response.Error = &MCPError{
			Code:    -32601, // Method not found
			Message: fmt.Sprintf("Method not found: %s", message.Method),
		}
		h.logger.Warn("Unknown method requested: %s", message.Method)
	}

	return response
}

// handleInitialize processes the MCP initialize request and returns server capabilities.
// This establishes the MCP session and informs the client about what the server can do.
//
// Parameters:
//   - params: Initialize request parameters from the client
//
// Returns:
//   - map[string]interface{}: Server capabilities and information
func (h *MCPHandler) handleInitialize(params map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]interface{}{
			"name":    "pdf-to-markdown-server",
			"version": "1.0.0",
		},
	}
}

// handleToolsList returns the list of available tools that this MCP server provides.
// Currently provides tools for PDF conversion: single file and directory batch processing.
//
// Returns:
//   - map[string]interface{}: Tools list with schemas and descriptions
func (h *MCPHandler) handleToolsList() map[string]interface{} {
	return map[string]interface{}{
		"tools": []map[string]interface{}{
			{
				"name":        "convert_pdf_to_markdown",
				"description": "Convert a single PDF file to Markdown format with extracted images",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"pdf_path": map[string]interface{}{
							"type":        "string",
							"description": "Path to the input PDF file",
						},
						"output_dir": map[string]interface{}{
							"type":        "string",
							"description": "Base output directory (optional, uses config default if not provided)",
						},
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
						"input_dir": map[string]interface{}{
							"type":        "string",
							"description": "Directory path containing PDF files to process",
						},
						"output_dir": map[string]interface{}{
							"type":        "string",
							"description": "Base output directory (optional, uses config default if not provided)",
						},
					},
					"required": []string{"input_dir"},
				},
			},
		},
	}
}

// handleToolsCall executes a tool call request, specifically handling PDF to Markdown conversion.
// It validates the parameters, performs the conversion, and returns the results.
//
// Parameters:
//   - params: Tool call parameters including tool name and arguments
//
// Returns:
//   - map[string]interface{}: Tool execution results
//   - error: Tool execution error, if any occurred
func (h *MCPHandler) handleToolsCall(params map[string]interface{}) (map[string]interface{}, error) {
	// Extract tool name
	toolName, ok := params["name"].(string)
	if !ok {
		return nil, fmt.Errorf("missing tool name")
	}

	if toolName != "convert_pdf_to_markdown" && toolName != "convert_pdfs_in_directory" {
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}

	// Extract tool arguments
	arguments, ok := params["arguments"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing tool arguments")
	}

	// Extract required parameters based on tool type
	var pdfPath string
	var inputDir string
	var outputDir string

	if toolName == "convert_pdf_to_markdown" {
		pdfPath, ok = arguments["pdf_path"].(string)
		if !ok {
			return nil, fmt.Errorf("missing required parameter: pdf_path")
		}
	} else if toolName == "convert_pdfs_in_directory" {
		inputDir, ok = arguments["input_dir"].(string)
		if !ok {
			return nil, fmt.Errorf("missing required parameter: input_dir")
		}
	}

	// Extract optional output directory
	outputDir = h.converter.config.OutputBaseDir
	if providedDir, exists := arguments["output_dir"].(string); exists {
		outputDir = providedDir
	}

	// Perform the conversion based on tool type
	if toolName == "convert_pdf_to_markdown" {
		h.logger.Info("Executing single PDF conversion: %s -> %s", pdfPath, outputDir)

		result, err := h.converter.ConvertPDF(pdfPath, outputDir)
		if err != nil {
			return nil, fmt.Errorf("conversion failed: %v", err)
		}

		// Format the response for single file conversion
		return map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": h.formatConversionResult(result),
				},
			},
		}, nil
	} else {
		h.logger.Info("Executing batch PDF conversion: %s -> %s", inputDir, outputDir)

		batchResult, err := h.converter.ConvertPDFsInDirectory(inputDir, outputDir)
		if err != nil {
			return nil, fmt.Errorf("batch conversion failed: %v", err)
		}

		// Format the response for batch conversion
		return map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": h.formatBatchConversionResult(batchResult),
				},
			},
		}, nil
	}
}

// formatConversionResult creates a formatted text description of the conversion results.
// This provides a human-readable summary of what was accomplished during the conversion.
//
// Parameters:
//   - result: Conversion result containing file paths and statistics
//
// Returns:
//   - string: Formatted description of the conversion results
func (h *MCPHandler) formatConversionResult(result *ConversionResult) string {
	return fmt.Sprintf(`PDF Conversion Completed Successfully

Output Directory: %s
Markdown File: %s
Pages Processed: %d
Images Extracted: %d

The PDF has been converted to Markdown format with all text content preserved and structured with appropriate headers. %s`,
		result.OutputDir,
		filepath.Base(result.MarkdownFile),
		result.PageCount,
		result.ImageCount,
		h.getImageExtractionNote(result.ImageCount),
	)
}

// formatBatchConversionResult creates a formatted text description of the batch conversion results.
// This provides a human-readable summary of what was accomplished during the batch conversion.
//
// Parameters:
//   - result: Batch conversion result containing file paths and statistics
//
// Returns:
//   - string: Formatted description of the batch conversion results
func (h *MCPHandler) formatBatchConversionResult(result *BatchConversionResult) string {
	var errorDetails string
	if result.FailureCount > 0 {
		errorDetails = fmt.Sprintf("\n\nErrors occurred during processing:\n")
		for _, err := range result.Errors {
			errorDetails += fmt.Sprintf("- %s: %s\n", filepath.Base(err.PDFPath), err.Error)
		}
	}

	return fmt.Sprintf(`Batch PDF Conversion Completed

Input Directory: %s
Output Directory: %s
PDF Files Found: %d
Successfully Converted: %d
Failed Conversions: %d
Total Pages Processed: %d
Total Images Extracted: %d

%s%s`,
		result.InputDir,
		result.OutputBaseDir,
		result.FileCount,
		result.SuccessCount,
		result.FailureCount,
		result.TotalPageCount,
		result.TotalImageCount,
		h.getImageExtractionNote(result.TotalImageCount),
		errorDetails,
	)
}

// getImageExtractionNote returns an appropriate note about image extraction based on the count.
// This provides additional context about the image extraction results.
//
// Parameters:
//   - imageCount: Number of images that were extracted
//
// Returns:
//   - string: Descriptive note about image extraction results
func (h *MCPHandler) getImageExtractionNote(imageCount int) string {
	if imageCount == 0 {
		return "No images were found in the PDF or image extraction was disabled."
	} else if imageCount == 1 {
		return "One image was extracted and saved as a PNG file."
	} else {
		return fmt.Sprintf("All %d images were extracted and saved as PNG files.", imageCount)
	}
}
