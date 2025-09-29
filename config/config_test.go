package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Save original environment
	originalEnv := map[string]string{}
	envVars := []string{
		"PDF_INPUT_DIR", "OUTPUT_BASE_DIR", "MCP_SERVER_NAME", "MCP_SERVER_VERSION",
		"IMAGE_MAX_DPI", "IMAGE_FORMAT", "PRESERVE_ASPECT_RATIO", "DETECT_DIAGRAMS",
		"DIAGRAM_CONFIDENCE", "PLANTUML_STYLE", "PLANTUML_COLOR_SCHEME", "INCLUDE_TOC",
		"BASE_HEADER_LEVEL", "EXTRACT_TABLES", "EXTRACT_IMAGES", "LOG_LEVEL", "MCP_TRANSPORT",
	}

	for _, key := range envVars {
		originalEnv[key] = os.Getenv(key)
		os.Unsetenv(key)
	}

	defer func() {
		for key, value := range originalEnv {
			if value != "" {
				os.Setenv(key, value)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	t.Run("default configuration", func(t *testing.T) {
		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig() failed: %v", err)
		}
		if cfg.OutputBaseDir != "./output" {
			t.Errorf("OutputBaseDir './output', got '%s'", cfg.OutputBaseDir)
		}
		if cfg.ServerName != "pdf-to-markdown-server" {
			t.Errorf("ServerName 'pdf-to-markdown-server', got '%s'", cfg.ServerName)
		}
		if cfg.ServerVersion != "1.0.0" {
			t.Errorf("ServerVersion '1.0.0', got '%s'", cfg.ServerVersion)
		}
		if cfg.ImageMaxDPI != 300 {
			t.Errorf("ImageMaxDPI 300, got %d", cfg.ImageMaxDPI)
		}
		if cfg.ImageFormat != "png" {
			t.Errorf("ImageFormat 'png', got '%s'", cfg.ImageFormat)
		}
		if !cfg.PreserveAspectRatio {
			t.Error("PreserveAspectRatio true")
		}
		if cfg.DetectDiagrams {
			t.Error("DetectDiagrams false")
		}
		if cfg.DiagramConfidence != 0.7 {
			t.Errorf("DiagramConfidence 0.7, got %f", cfg.DiagramConfidence)
		}
		if cfg.PlantUMLStyle != "default" {
			t.Errorf("PlantUMLStyle 'default', got '%s'", cfg.PlantUMLStyle)
		}
		if cfg.PlantUMLColorScheme != "auto" {
			t.Errorf("PlantUMLColorScheme 'auto', got '%s'", cfg.PlantUMLColorScheme)
		}
		if !cfg.IncludeTOC {
			t.Error("IncludeTOC true")
		}
		if cfg.BaseHeaderLevel != 1 {
			t.Errorf("BaseHeaderLevel 1, got %d", cfg.BaseHeaderLevel)
		}
		if !cfg.ExtractTables {
			t.Error("ExtractTables true")
		}
		if !cfg.ExtractImages {
			t.Error("ExtractImages true")
		}
		if cfg.LogLevel != "info" {
			t.Errorf("LogLevel 'info', got '%s'", cfg.LogLevel)
		}
		if cfg.Transport != "stdio" {
			t.Errorf("Transport 'stdio', got '%s'", cfg.Transport)
		}
	})

	t.Run("custom configuration", func(t *testing.T) {
		os.Setenv("PDF_INPUT_DIR", "/custom/input")
		os.Setenv("OUTPUT_BASE_DIR", "/custom/output")
		os.Setenv("MCP_SERVER_NAME", "custom-server")
		os.Setenv("MCP_SERVER_VERSION", "2.0.0")
		os.Setenv("IMAGE_MAX_DPI", "600")
		os.Setenv("IMAGE_FORMAT", "jpg")
		os.Setenv("PRESERVE_ASPECT_RATIO", "false")
		os.Setenv("DETECT_DIAGRAMS", "true")
		os.Setenv("DIAGRAM_CONFIDENCE", "0.8")
		os.Setenv("PLANTUML_STYLE", "blueprint")
		os.Setenv("PLANTUML_COLOR_SCHEME", "mono")
		os.Setenv("INCLUDE_TOC", "false")
		os.Setenv("BASE_HEADER_LEVEL", "2")
		os.Setenv("EXTRACT_TABLES", "false")
		os.Setenv("EXTRACT_IMAGES", "false")
		os.Setenv("LOG_LEVEL", "debug")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig() failed: %v", err)
		}
		if cfg.PDFInputDir != "/custom/input" {
			t.Errorf("PDFInputDir '/custom/input', got '%s'", cfg.PDFInputDir)
		}
		if cfg.OutputBaseDir != "/custom/output" {
			t.Errorf("OutputBaseDir '/custom/output', got '%s'", cfg.OutputBaseDir)
		}
		if cfg.ServerName != "custom-server" {
			t.Errorf("ServerName 'custom-server', got '%s'", cfg.ServerName)
		}
		if cfg.ServerVersion != "2.0.0" {
			t.Errorf("ServerVersion '2.0.0', got '%s'", cfg.ServerVersion)
		}
		if cfg.ImageMaxDPI != 600 {
			t.Errorf("ImageMaxDPI 600, got %d", cfg.ImageMaxDPI)
		}
		if cfg.ImageFormat != "jpg" {
			t.Errorf("ImageFormat 'jpg', got '%s'", cfg.ImageFormat)
		}
		if cfg.PreserveAspectRatio {
			t.Error("PreserveAspectRatio false")
		}
		if !cfg.DetectDiagrams {
			t.Error("DetectDiagrams true")
		}
		if cfg.DiagramConfidence != 0.8 {
			t.Errorf("DiagramConfidence 0.8, got %f", cfg.DiagramConfidence)
		}
		if cfg.PlantUMLStyle != "blueprint" {
			t.Errorf("PlantUMLStyle 'blueprint', got '%s'", cfg.PlantUMLStyle)
		}
		if cfg.PlantUMLColorScheme != "mono" {
			t.Errorf("PlantUMLColorScheme 'mono', got '%s'", cfg.PlantUMLColorScheme)
		}
		if cfg.IncludeTOC {
			t.Error("IncludeTOC false")
		}
		if cfg.BaseHeaderLevel != 2 {
			t.Errorf("BaseHeaderLevel 2, got %d", cfg.BaseHeaderLevel)
		}
		if cfg.ExtractTables {
			t.Error("ExtractTables false")
		}
		if cfg.ExtractImages {
			t.Error("ExtractImages false")
		}
		if cfg.LogLevel != "debug" {
			t.Errorf("LogLevel 'debug', got '%s'", cfg.LogLevel)
		}
	})
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name        string
		cfg         Config
		expectError bool
		errorMsg    string
	}{
		{"valid configuration", Config{ImageMaxDPI: 300, ImageFormat: "png", DiagramConfidence: 0.7, BaseHeaderLevel: 1, LogLevel: "info", Transport: "stdio", PlantUMLStyle: "default", PlantUMLColorScheme: "auto"}, false, ""},
		{"invalid ImageMaxDPI - too low", Config{ImageMaxDPI: 50, ImageFormat: "png", DiagramConfidence: 0.7, BaseHeaderLevel: 1, LogLevel: "info", Transport: "stdio", PlantUMLStyle: "default", PlantUMLColorScheme: "auto"}, true, "IMAGE_MAX_DPI must be between 72 and 600"},
		{"invalid ImageMaxDPI - too high", Config{ImageMaxDPI: 800, ImageFormat: "png", DiagramConfidence: 0.7, BaseHeaderLevel: 1, LogLevel: "info", Transport: "stdio", PlantUMLStyle: "default", PlantUMLColorScheme: "auto"}, true, "IMAGE_MAX_DPI must be between 72 and 600"},
		{"invalid ImageFormat", Config{ImageMaxDPI: 300, ImageFormat: "gif", DiagramConfidence: 0.7, BaseHeaderLevel: 1, LogLevel: "info", Transport: "stdio", PlantUMLStyle: "default", PlantUMLColorScheme: "auto"}, true, "IMAGE_FORMAT must be 'png' or 'jpg'"},
		{"invalid DiagramConfidence - too low", Config{ImageMaxDPI: 300, ImageFormat: "png", DiagramConfidence: -0.1, BaseHeaderLevel: 1, LogLevel: "info", Transport: "stdio", PlantUMLStyle: "default", PlantUMLColorScheme: "auto"}, true, "DIAGRAM_CONFIDENCE must be between 0.0 and 1.0"},
		{"invalid DiagramConfidence - too high", Config{ImageMaxDPI: 300, ImageFormat: "png", DiagramConfidence: 1.1, BaseHeaderLevel: 1, LogLevel: "info", Transport: "stdio", PlantUMLStyle: "default", PlantUMLColorScheme: "auto"}, true, "DIAGRAM_CONFIDENCE must be between 0.0 and 1.0"},
		{"invalid BaseHeaderLevel - too low", Config{ImageMaxDPI: 300, ImageFormat: "png", DiagramConfidence: 0.7, BaseHeaderLevel: 0, LogLevel: "info", Transport: "stdio", PlantUMLStyle: "default", PlantUMLColorScheme: "auto"}, true, "BASE_HEADER_LEVEL must be between 1 and 6"},
		{"invalid BaseHeaderLevel - too high", Config{ImageMaxDPI: 300, ImageFormat: "png", DiagramConfidence: 0.7, BaseHeaderLevel: 7, LogLevel: "info", Transport: "stdio", PlantUMLStyle: "default", PlantUMLColorScheme: "auto"}, true, "BASE_HEADER_LEVEL must be between 1 and 6"},
		{"invalid LogLevel", Config{ImageMaxDPI: 300, ImageFormat: "png", DiagramConfidence: 0.7, BaseHeaderLevel: 1, LogLevel: "invalid", Transport: "stdio", PlantUMLStyle: "default", PlantUMLColorScheme: "auto"}, true, "LOG_LEVEL must be one of"},
		{"invalid Transport", Config{ImageMaxDPI: 300, ImageFormat: "png", DiagramConfidence: 0.7, BaseHeaderLevel: 1, LogLevel: "info", Transport: "tcp", PlantUMLStyle: "default", PlantUMLColorScheme: "auto"}, true, "MCP_TRANSPORT must be one of"},
		{"invalid PlantUMLStyle", Config{ImageMaxDPI: 300, ImageFormat: "png", DiagramConfidence: 0.7, BaseHeaderLevel: 1, LogLevel: "info", Transport: "stdio", PlantUMLStyle: "invalid", PlantUMLColorScheme: "auto"}, true, "PLANTUML_STYLE must be one of"},
		{"invalid PlantUMLColorScheme", Config{ImageMaxDPI: 300, ImageFormat: "png", DiagramConfidence: 0.7, BaseHeaderLevel: 1, LogLevel: "info", Transport: "stdio", PlantUMLStyle: "default", PlantUMLColorScheme: "invalid"}, true, "PLANTUML_COLOR_SCHEME must be one of"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.errorMsg)
				}
				if err != nil && tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}
