// Package config - Configuration management for the PDF to Markdown MCP server.
// This file handles loading and validating configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all configuration settings for the PDF to Markdown MCP server.
// These settings control server behavior, PDF processing options, output formatting,
// and MCP transport configuration.
type Config struct {
	// PDF Input/Output Settings
	PDFInputDir   string // Directory containing PDF files to process
	OutputBaseDir string // Base directory where MARKDOWN_<filename> subdirectories will be created

	// Server Settings
	ServerName    string // Name of the MCP server for identification
	ServerVersion string // Version of the MCP server

	// PDF Processing Settings
	ImageMaxDPI         int    // Maximum DPI for extracted images (higher = better quality, larger files)
	ImageFormat         string // Format for extracted images (png, jpg)
	PreserveAspectRatio bool   // Whether to maintain original image aspect ratios

	// Diagram Detection and PlantUML Settings
	DetectDiagrams      bool    // Whether to detect diagrams in PDFs and convert to PlantUML
	DiagramConfidence   float64 // Minimum confidence threshold for diagram detection (0.0-1.0)
	PlantUMLStyle       string  // PlantUML diagram style (default, blueprint, modern)
	PlantUMLColorScheme string  // PlantUML color scheme (mono, color, auto)

	// Markdown Generation Settings
	IncludeTOC      bool // Whether to generate a table of contents in the markdown
	BaseHeaderLevel int  // Starting header level for sections (1-6, where 1 = #, 2 = ##, etc.)
	ExtractTables   bool // Whether to attempt table extraction and conversion
	ExtractImages   bool // Whether to extract and save images from the PDF

	// Logging Configuration
	LogLevel string // Logging verbosity level (debug, info, warn, error)

	// MCP Transport Settings
	Transport string // Transport method for MCP communication (stdio only)
}

// LoadConfig creates a new Config instance by reading values from environment variables.
// It applies sensible defaults for any missing configuration values and validates
// that required settings are present.
//
// Environment variables read:
//   - PDF_INPUT_DIR: Directory containing PDF files to process
//   - OUTPUT_BASE_DIR: Base output directory
//   - MCP_SERVER_NAME: Server identification name
//   - MCP_SERVER_VERSION: Server version
//   - IMAGE_MAX_DPI: Maximum image resolution
//   - IMAGE_FORMAT: Image output format
//   - PRESERVE_ASPECT_RATIO: Maintain image aspect ratios
//   - DETECT_DIAGRAMS: Enable diagram detection
//   - DIAGRAM_CONFIDENCE: Minimum confidence for diagram detection
//   - PLANTUML_STYLE: PlantUML diagram style
//   - PLANTUML_COLOR_SCHEME: PlantUML color scheme
//   - INCLUDE_TOC: Generate table of contents
//   - BASE_HEADER_LEVEL: Starting header level for sections
//   - EXTRACT_TABLES: Enable table extraction
//   - EXTRACT_IMAGES: Enable image extraction
//   - LOG_LEVEL: Logging verbosity
//   - MCP_TRANSPORT: Transport method
//
// Returns:
//   - *Config: Populated configuration struct
//   - error: Configuration validation error, if any
func LoadConfig() (*Config, error) {
	config := &Config{
		// Set default values first
		PDFInputDir:         getEnvWithDefault("PDF_INPUT_DIR", ""),
		OutputBaseDir:       getEnvWithDefault("OUTPUT_BASE_DIR", "./output"),
		ServerName:          getEnvWithDefault("MCP_SERVER_NAME", "pdf-to-markdown-server"),
		ServerVersion:       getEnvWithDefault("MCP_SERVER_VERSION", "1.0.0"),
		ImageMaxDPI:         getEnvIntWithDefault("IMAGE_MAX_DPI", 300),
		ImageFormat:         getEnvWithDefault("IMAGE_FORMAT", "png"),
		PreserveAspectRatio: getEnvBoolWithDefault("PRESERVE_ASPECT_RATIO", true),
		DetectDiagrams:      getEnvBoolWithDefault("DETECT_DIAGRAMS", false),
		DiagramConfidence:   getEnvFloat64WithDefault("DIAGRAM_CONFIDENCE", 0.7),
		PlantUMLStyle:       getEnvWithDefault("PLANTUML_STYLE", "default"),
		PlantUMLColorScheme: getEnvWithDefault("PLANTUML_COLOR_SCHEME", "auto"),
		IncludeTOC:          getEnvBoolWithDefault("INCLUDE_TOC", true),
		BaseHeaderLevel:     getEnvIntWithDefault("BASE_HEADER_LEVEL", 1),
		ExtractTables:       getEnvBoolWithDefault("EXTRACT_TABLES", true),
		ExtractImages:       getEnvBoolWithDefault("EXTRACT_IMAGES", true),
		LogLevel:            getEnvWithDefault("LOG_LEVEL", "info"),
		Transport:           getEnvWithDefault("MCP_TRANSPORT", "stdio"),
	}

	// Validate configuration values
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %v", err)
	}

	return config, nil
}

// Validate checks that all configuration values are valid and within acceptable ranges.
// It ensures that critical settings like paths exist and numeric values are within bounds.
//
// Validation rules:
//   - ImageMaxDPI must be between 72 and 600 DPI
//   - ImageFormat must be "png" or "jpg"
//   - DiagramConfidence must be between 0.0 and 1.0
//   - BaseHeaderLevel must be between 1 and 6
//   - LogLevel must be one of: debug, info, warn, error
//   - Transport must be one of: stdio
//
// Returns:
//   - error: Validation error describing the first invalid setting found, or nil if valid
func (c *Config) Validate() error {
	// Validate image DPI range
	if c.ImageMaxDPI < 72 || c.ImageMaxDPI > 600 {
		return fmt.Errorf("IMAGE_MAX_DPI must be between 72 and 600, got %d", c.ImageMaxDPI)
	}

	// Validate image format
	if c.ImageFormat != "png" && c.ImageFormat != "jpg" {
		return fmt.Errorf("IMAGE_FORMAT must be 'png' or 'jpg', got '%s'", c.ImageFormat)
	}

	// Validate diagram confidence range
	if c.DiagramConfidence < 0.0 || c.DiagramConfidence > 1.0 {
		return fmt.Errorf("DIAGRAM_CONFIDENCE must be between 0.0 and 1.0, got %f", c.DiagramConfidence)
	}

	// Validate header level range
	if c.BaseHeaderLevel < 1 || c.BaseHeaderLevel > 6 {
		return fmt.Errorf("BASE_HEADER_LEVEL must be between 1 and 6, got %d", c.BaseHeaderLevel)
	}

	// Validate log level
	validLogLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLogLevels, c.LogLevel) {
		return fmt.Errorf("LOG_LEVEL must be one of %v, got '%s'", validLogLevels, c.LogLevel)
	}

	// Validate transport method
	validTransports := []string{"stdio"}
	if !contains(validTransports, c.Transport) {
		return fmt.Errorf("MCP_TRANSPORT must be one of %v, got '%s'", validTransports, c.Transport)
	}

	// Validate PlantUML style
	validStyles := []string{"default", "blueprint", "modern"}
	if !contains(validStyles, c.PlantUMLStyle) {
		return fmt.Errorf("PLANTUML_STYLE must be one of %v, got '%s'", validStyles, c.PlantUMLStyle)
	}

	// Validate PlantUML color scheme
	validColorSchemes := []string{"mono", "color", "auto"}
	if !contains(validColorSchemes, c.PlantUMLColorScheme) {
		return fmt.Errorf("PLANTUML_COLOR_SCHEME must be one of %v, got '%s'", validColorSchemes, c.PlantUMLColorScheme)
	}

	return nil
}

// getEnvWithDefault retrieves an environment variable value or returns a default if not set.
// This is a helper function for loading string configuration values.
//
// Parameters:
//   - key: Environment variable name to look up
//   - defaultValue: Value to return if environment variable is not set or empty
//
// Returns:
//   - string: Environment variable value or default value
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvIntWithDefault retrieves an environment variable as an integer or returns a default.
// If the environment variable cannot be parsed as an integer, the default value is returned.
//
// Parameters:
//   - key: Environment variable name to look up
//   - defaultValue: Integer value to return if environment variable is not set or invalid
//
// Returns:
//   - int: Parsed integer value or default value
func getEnvIntWithDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvFloat64WithDefault retrieves an environment variable as a float64 or returns a default.
// If the environment variable cannot be parsed as a float64, the default value is returned.
//
// Parameters:
//   - key: Environment variable name to look up
//   - defaultValue: Float64 value to return if environment variable is not set or invalid
//
// Returns:
//   - float64: Parsed float64 value or default value
func getEnvFloat64WithDefault(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

// getEnvBoolWithDefault retrieves an environment variable as a boolean or returns a default.
// Accepts various boolean representations: "true", "1", "yes", "on" for true;
// "false", "0", "no", "off" for false. Case-insensitive.
//
// Parameters:
//   - key: Environment variable name to look up
//   - defaultValue: Boolean value to return if environment variable is not set or invalid
//
// Returns:
//   - bool: Parsed boolean value or default value
func getEnvBoolWithDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		value = strings.ToLower(strings.TrimSpace(value))
		switch value {
		case "true", "1", "yes", "on":
			return true
		case "false", "0", "no", "off":
			return false
		}
	}
	return defaultValue
}

// contains checks if a string slice contains a specific string value.
// This is a utility function used for validating configuration values against allowed lists.
//
// Parameters:
//   - slice: String slice to search in
//   - item: String value to search for
//
// Returns:
//   - bool: True if the item is found in the slice, false otherwise
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
