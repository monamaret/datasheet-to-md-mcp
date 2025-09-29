package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"

	"datasheet-to-md-mcp/cli"
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
	// If invoked with the 'config' subcommand, run the config CLI tool and exit
	if len(os.Args) > 1 && os.Args[1] == "config" {
		code := (&cli.ConfigCLI{}).Run(os.Args[2:])
		os.Exit(code)
	}

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
