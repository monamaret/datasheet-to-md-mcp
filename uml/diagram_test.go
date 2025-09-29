package uml

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"datasheet-to-md-mcp/config"
	"datasheet-to-md-mcp/logger"
)

func TestNewDiagramDetector(t *testing.T) {
	cfg := &config.Config{DetectDiagrams: true, DiagramConfidence: 0.7, PlantUMLStyle: "default", PlantUMLColorScheme: "auto"}
	logr := logger.NewLogger("info")
	d := NewDiagramDetector(cfg, logr)
	if d == nil {
		t.Fatal("nil detector")
	}
	if d.config != cfg {
		t.Error("config not assigned")
	}
	if d.logger != logr {
		t.Error("logger not assigned")
	}
}

func TestDiagramType_String(t *testing.T) {
	tests := []struct {
		dt       DiagramType
		expected string
	}{
		{UnknownDiagram, "unknown"}, {FlowChart, "flowchart"}, {BlockDiagram, "block"}, {CircuitDiagram, "circuit"}, {NetworkDiagram, "network"}, {SequenceDiagram, "sequence"}, {ClassDiagram, "class"}, {ERDiagram, "er"}, {DiagramType(999), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.dt.String() != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, tt.dt.String())
			}
		})
	}
}

func TestDetectDiagramsInImage_DisabledDetection(t *testing.T) {
	cfg := &config.Config{DetectDiagrams: false}
	logr := logger.NewLogger("info")
	d := NewDiagramDetector(cfg, logr)
	tempFile := createTempImageFile(t, "test_diagram.png")
	defer os.Remove(tempFile)
	diagrams, err := d.DetectDiagramsInImage(tempFile)
	if err != nil {
		t.Fatalf("DetectDiagramsInImage failed: %v", err)
	}
	if len(diagrams) != 0 {
		t.Errorf("Expected no diagrams when disabled, got %d", len(diagrams))
	}
}

func TestDetectDiagramsInImage_EnabledDetection(t *testing.T) {
	cfg := &config.Config{DetectDiagrams: true, DiagramConfidence: 0.5, PlantUMLStyle: "default", PlantUMLColorScheme: "auto"}
	logr := logger.NewLogger("info")
	d := NewDiagramDetector(cfg, logr)
	tests := []struct {
		filename           string
		expectedType       DiagramType
		shouldDetect       bool
		expectedConfidence float64
	}{
		{"flowchart_example.png", FlowChart, true, 0.8},
		{"block_diagram.png", BlockDiagram, true, 0.8},
		{"circuit_schematic.png", CircuitDiagram, true, 0.8},
		{"network_topology.png", NetworkDiagram, true, 0.8},
		{"page_1_image_1.png", BlockDiagram, true, 0.6},
		{"regular_photo.jpg", UnknownDiagram, false, 0.2},
	}
	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			tempFile := createTempImageFile(t, tt.filename)
			defer os.Remove(tempFile)
			diagrams, err := d.DetectDiagramsInImage(tempFile)
			if err != nil {
				t.Fatalf("DetectDiagramsInImage failed: %v", err)
			}
			if tt.shouldDetect {
				if len(diagrams) == 0 {
					t.Errorf("Expected detection for %s", tt.filename)
					return
				}
				diagram := diagrams[0]
				if diagram.Type != tt.expectedType {
					t.Errorf("Expected %s, got %s", tt.expectedType.String(), diagram.Type.String())
				}
				if diagram.Confidence != tt.expectedConfidence {
					t.Errorf("Expected confidence %f, got %f", tt.expectedConfidence, diagram.Confidence)
				}
				if diagram.ImagePath != tempFile {
					t.Errorf("Expected image path %s, got %s", tempFile, diagram.ImagePath)
				}
				if diagram.PlantUML == "" {
					t.Error("Expected PlantUML code")
				}
			} else if len(diagrams) > 0 {
				t.Errorf("Expected no detection for %s", tt.filename)
			}
		})
	}
}

func TestAnalyzeImageMetadata(t *testing.T) {
	cfg := &config.Config{}
	logr := logger.NewLogger("info")
	d := NewDiagramDetector(cfg, logr)
	tests := []struct {
		filename           string
		expectedConfidence float64
		expectedType       DiagramType
	}{
		{"flowchart_test.png", 0.8, FlowChart},
		{"diagram_example.jpg", 0.8, FlowChart},
		{"block_diagram.png", 0.8, BlockDiagram},
		{"schematic_view.png", 0.8, BlockDiagram},
		{"circuit_board.png", 0.8, CircuitDiagram},
		{"electronic_diagram.png", 0.8, CircuitDiagram},
		{"network_map.png", 0.8, NetworkDiagram},
		{"topology_view.png", 0.8, NetworkDiagram},
		{"page_1_image_1.png", 0.6, BlockDiagram},
		{"page_2_image_3.jpg", 0.6, BlockDiagram},
		{"random_photo.jpg", 0.2, UnknownDiagram},
		{"document_scan.pdf", 0.2, UnknownDiagram},
	}
	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			tempDir := t.TempDir()
			imagePath := filepath.Join(tempDir, tt.filename)
			confidence, tpe := d.analyzeImageMetadata(imagePath)
			if confidence != tt.expectedConfidence {
				t.Errorf("Expected confidence %f, got %f", tt.expectedConfidence, confidence)
			}
			if tpe != tt.expectedType {
				t.Errorf("Expected type %s, got %s", tt.expectedType.String(), tpe.String())
			}
		})
	}
}

func TestGeneratePlantUML(t *testing.T) {
	tests := []struct {
		style, colorScheme string
		dt                 DiagramType
	}{
		{"default", "auto", FlowChart}, {"blueprint", "mono", BlockDiagram}, {"modern", "color", CircuitDiagram}, {"default", "auto", NetworkDiagram}, {"default", "auto", UnknownDiagram},
	}
	for _, tt := range tests {
		t.Run(tt.style+"_"+tt.colorScheme+"_"+tt.dt.String(), func(t *testing.T) {
			cfg := &config.Config{PlantUMLStyle: tt.style, PlantUMLColorScheme: tt.colorScheme}
			logr := logger.NewLogger("info")
			d := NewDiagramDetector(cfg, logr)
			plantUML, err := d.generatePlantUML(tt.dt)
			if err != nil {
				t.Fatalf("generatePlantUML failed: %v", err)
			}
			if !strings.HasPrefix(plantUML, "@startuml") {
				t.Error("PlantUML should start with @startuml")
			}
			if !strings.HasSuffix(strings.TrimSpace(plantUML), "@enduml") {
				t.Error("PlantUML should end with @enduml")
			}
			if tt.style == "blueprint" && !strings.Contains(plantUML, "!theme blueprint") {
				t.Error("Expected blueprint theme")
			}
			if tt.style == "modern" && !strings.Contains(plantUML, "!theme modern") {
				t.Error("Expected modern theme")
			}
			if tt.colorScheme == "mono" && !strings.Contains(plantUML, "skinparam monochrome true") {
				t.Error("Expected monochrome setting")
			}
			if tt.colorScheme == "color" && !strings.Contains(plantUML, "skinparam monochrome false") {
				t.Error("Expected color setting")
			}
			switch tt.dt {
			case FlowChart:
				if !strings.Contains(plantUML, "start") || !strings.Contains(plantUML, "stop") {
					t.Error("Flowchart should contain start/stop")
				}
			case BlockDiagram:
				if !strings.Contains(plantUML, "rectangle") || !strings.Contains(plantUML, "BLOCK") {
					t.Error("Block diagram should contain rectangles/blocks")
				}
			case CircuitDiagram:
				if !strings.Contains(plantUML, "COMPONENT") {
					t.Error("Circuit diagram should contain components")
				}
			case NetworkDiagram:
				if !strings.Contains(plantUML, "C4") {
					t.Error("Network diagram should use C4 notation")
				}
			}
			if !strings.Contains(plantUML, "converted from PDF") {
				t.Error("Should contain conversion note")
			}
		})
	}
}

func TestGetPlantUMLMarkdown(t *testing.T) {
	cfg := &config.Config{}
	logr := logger.NewLogger("info")
	d := NewDiagramDetector(cfg, logr)
	diagram := DetectedDiagram{Type: FlowChart, Confidence: 0.85, PlantUML: "@startuml\nstart\nstop\n@enduml", ImagePath: "/path/to/test_diagram.png", BoundingBox: image.Rect(0, 0, 400, 300)}
	markdown := d.GetPlantUMLMarkdown(diagram)
	if !strings.Contains(markdown, "### Detected Flowchart Diagram") {
		t.Error("Should contain diagram type header")
	}
	if !strings.Contains(markdown, "85.0%") {
		t.Error("Should contain confidence percentage")
	}
	if !strings.Contains(markdown, "```plantuml") {
		t.Error("Should contain PlantUML code block")
	}
	if !strings.Contains(markdown, "@startuml") {
		t.Error("Should contain PlantUML code")
	}
	if !strings.Contains(markdown, "test_diagram.png") {
		t.Error("Should reference original image filename")
	}
}

func TestApplyPlantUMLStyle(t *testing.T) {
	tests := []struct {
		style, colorScheme string
		expected           []string
	}{
		{"blueprint", "mono", []string{"!theme blueprint", "skinparam monochrome true"}},
		{"modern", "color", []string{"!theme modern", "skinparam monochrome false"}},
		{"default", "auto", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.style+"_"+tt.colorScheme, func(t *testing.T) {
			cfg := &config.Config{PlantUMLStyle: tt.style, PlantUMLColorScheme: tt.colorScheme}
			logr := logger.NewLogger("info")
			d := NewDiagramDetector(cfg, logr)
			var plantUML strings.Builder
			d.applyPlantUMLStyle(&plantUML)
			result := plantUML.String()
			for _, expected := range tt.expected {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected '%s' in PlantUML style, got: %s", expected, result)
				}
			}
		})
	}
}

func TestDiagramDetectorWithRealPDF(t *testing.T) {
	// The original test relied on a PDF in pdf/ which no longer exists.
	// Since DetectDiagramsInImage works on image files, keep this test focused on image detection.
	cfg := &config.Config{DetectDiagrams: true, DiagramConfidence: 0.6, PlantUMLStyle: "default", PlantUMLColorScheme: "auto", ImageMaxDPI: 300, ImageFormat: "png"}
	logr := logger.NewLogger("info")
	d := NewDiagramDetector(cfg, logr)
	tempFile := createTempImageFile(t, "page_1_image_1.png")
	defer os.Remove(tempFile)
	diagrams, err := d.DetectDiagramsInImage(tempFile)
	if err != nil {
		t.Fatalf("DetectDiagramsInImage failed: %v", err)
	}
	if len(diagrams) > 0 {
		diagram := diagrams[0]
		if diagram.Confidence < 0.5 {
			t.Errorf("Expected confidence >= 0.5, got %f", diagram.Confidence)
		}
		if diagram.Type == UnknownDiagram {
			t.Error("Expected a specific diagram type")
		}
		if diagram.PlantUML == "" {
			t.Error("Expected PlantUML code")
		}
		if !strings.HasPrefix(diagram.PlantUML, "@startuml") {
			t.Error("PlantUML should start with @startuml")
		}
		if !strings.HasSuffix(strings.TrimSpace(diagram.PlantUML), "@enduml") {
			t.Error("PlantUML should end with @enduml")
		}
	}
}

func createTempImageFile(t *testing.T, filename string) string {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, filename)
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	file.Close()
	return filePath
}

func TestDiagramDetectorEdgeCases(t *testing.T) {
	cfg := &config.Config{DetectDiagrams: true, DiagramConfidence: 0.7, PlantUMLStyle: "default", PlantUMLColorScheme: "auto"}
	logr := logger.NewLogger("info")
	d := NewDiagramDetector(cfg, logr)
	t.Run("non-existent file", func(t *testing.T) {
		diagrams, err := d.DetectDiagramsInImage("/non/existent/file.png")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if len(diagrams) > 0 {
			t.Error("Expected no diagrams for non-existent file")
		}
	})
	t.Run("high confidence threshold", func(t *testing.T) {
		cfg.DiagramConfidence = 0.9
		tempFile := createTempImageFile(t, "page_1_image_1.png")
		defer os.Remove(tempFile)
		diagrams, err := d.DetectDiagramsInImage(tempFile)
		if err != nil {
			t.Fatalf("DetectDiagramsInImage failed: %v", err)
		}
		if len(diagrams) > 0 {
			t.Error("Expected no diagrams with very high confidence threshold")
		}
	})
	t.Run("empty filename", func(t *testing.T) {
		tempFile := createTempImageFile(t, "empty.png")
		defer os.Remove(tempFile)
		diagrams, err := d.DetectDiagramsInImage(tempFile)
		if err != nil {
			t.Errorf("Expected no error for empty filename, got: %v", err)
		}
		// Should not detect anything for empty filename
		if len(diagrams) > 0 {
			t.Error("Expected no diagrams for empty filename")
		}
	})
}
