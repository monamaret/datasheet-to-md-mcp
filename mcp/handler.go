// Package mcp - MCP (Model Context Protocol) message handling and transport layer.
// This file implements the MCP protocol communication layer, handling stdio transport
// and processing MCP messages.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"datasheet-to-md-mcp/logger"
	"datasheet-to-md-mcp/pdfconv"
)

// MCPHandler manages MCP protocol communication and message processing.
// It handles stdio transport and routes MCP messages to appropriate handlers.
type MCPHandler struct {
	converter *pdfconv.PDFConverter // PDF conversion engine for processing tool calls
	logger    *logger.Logger        // Logger for tracking MCP operations
}

// MCPMessage represents a generic MCP protocol message that can be either a request or response.
// The JSON-RPC 2.0 format is used for all MCP communications.
type MCPMessage struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id,omitempty"`
	Method  string                 `json:"method,omitempty"`
	Params  map[string]interface{} `json:"params,omitempty"`
	Result  map[string]interface{} `json:"result,omitempty"`
	Error   *MCPError              `json:"error,omitempty"`
}

// MCPError represents an error response in the MCP protocol
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// NewMCPHandler creates a new MCP message handler with the specified converter and logger.
func NewMCPHandler(converter *pdfconv.PDFConverter, logger *logger.Logger) *MCPHandler {
	return &MCPHandler{
		converter: converter,
		logger:    logger,
	}
}

// HandleStdio processes MCP messages using standard input/output communication.
func (h *MCPHandler) HandleStdio() error {
	h.logger.Debug("Starting STDIO message handling")

	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		h.logger.Debug("Received message: %s", line)

		var message MCPMessage
		if err := json.Unmarshal([]byte(line), &message); err != nil {
			h.logger.Error("Failed to parse message: %v", err)
			errorResponse := MCPMessage{JSONRPC: "2.0", Error: &MCPError{Code: -32700, Message: "Parse error", Data: err.Error()}}
			_ = encoder.Encode(errorResponse)
			continue
		}

		response := h.processMessage(&message)

		if err := encoder.Encode(response); err != nil {
			h.logger.Error("Failed to send response: %v", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading from stdin: %v", err)
	}

	return nil
}

// processMessage handles the core MCP message processing logic.
func (h *MCPHandler) processMessage(message *MCPMessage) MCPMessage {
	if message.JSONRPC == "" {
		message.JSONRPC = "2.0"
	}

	response := MCPMessage{JSONRPC: "2.0", ID: message.ID}

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
			response.Error = &MCPError{Code: -32603, Message: err.Error()}
			h.logger.Error("Tool call failed: %v", err)
		} else {
			response.Result = result
			h.logger.Info("Tool call completed successfully")
		}

	case "notifications/initialized":
		return MCPMessage{}

	default:
		response.Error = &MCPError{Code: -32601, Message: fmt.Sprintf("Method not found: %s", message.Method)}
		h.logger.Warn("Unknown method requested: %s", message.Method)
	}

	return response
}

// handleInitialize processes the MCP initialize request and returns server capabilities.
func (h *MCPHandler) handleInitialize(params map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
		"serverInfo":      map[string]interface{}{"name": "pdf-to-markdown-server", "version": "1.0.0"},
	}
}

// handleToolsList returns the list of available tools.
func (h *MCPHandler) handleToolsList() map[string]interface{} {
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

// handleToolsCall executes a tool call request.
func (h *MCPHandler) handleToolsCall(params map[string]interface{}) (map[string]interface{}, error) {
	toolName, ok := params["name"].(string)
	if !ok {
		return nil, fmt.Errorf("missing tool name")
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
		outputDir := h.converter.Config().OutputBaseDir
		if providedDir, exists := arguments["output_dir"].(string); exists {
			outputDir = providedDir
		}
		h.logger.Info("Executing single PDF conversion: %s -> %s", pdfPath, outputDir)
		result, err := h.converter.ConvertPDF(pdfPath, outputDir)
		if err != nil {
			return nil, fmt.Errorf("conversion failed: %v", err)
		}
		return map[string]interface{}{"content": []map[string]interface{}{{"type": "text", "text": h.formatConversionResult(result)}}}, nil

	case "convert_pdfs_in_directory":
		inputDir, ok := arguments["input_dir"].(string)
		if !ok {
			return nil, fmt.Errorf("missing required parameter: input_dir")
		}
		outputDir := h.converter.Config().OutputBaseDir
		if providedDir, exists := arguments["output_dir"].(string); exists {
			outputDir = providedDir
		}
		h.logger.Info("Executing batch PDF conversion: %s -> %s", inputDir, outputDir)
		batchResult, err := h.converter.ConvertPDFsInDirectory(inputDir, outputDir)
		if err != nil {
			return nil, fmt.Errorf("batch conversion failed: %v", err)
		}
		return map[string]interface{}{"content": []map[string]interface{}{{"type": "text", "text": h.formatBatchConversionResult(batchResult)}}}, nil
	}

	return nil, fmt.Errorf("unexpected tool name: %s", toolName)
}

// formatConversionResult creates a formatted text description of the conversion results.
func (h *MCPHandler) formatConversionResult(result *pdfconv.ConversionResult) string {
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
func (h *MCPHandler) formatBatchConversionResult(result *pdfconv.BatchConversionResult) string {
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
func (h *MCPHandler) getImageExtractionNote(imageCount int) string {
	if imageCount == 0 {
		return "No images were found in the PDF or image extraction was disabled."
	} else if imageCount == 1 {
		return "One image was extracted and saved as a PNG file."
	}
	return fmt.Sprintf("All %d images were extracted and saved as PNG files.", imageCount)
}
