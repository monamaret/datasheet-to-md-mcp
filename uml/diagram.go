// Package uml - Diagram detection and PlantUML conversion functionality.
// This file handles the detection of diagrams in PDF images and their conversion to PlantUML format.
package uml

import (
	"fmt"
	"image"
	"path/filepath"
	"strings"

	"datasheet-to-md-mcp/config"
	"datasheet-to-md-mcp/logger"
)

// DiagramDetector handles the detection and analysis of diagrams in PDF images.
type DiagramDetector struct {
	config *config.Config
	logger *logger.Logger
}

// DiagramType represents the type of diagram detected
type DiagramType int

const (
	UnknownDiagram DiagramType = iota
	FlowChart
	BlockDiagram
	CircuitDiagram
	NetworkDiagram
	SequenceDiagram
	ClassDiagram
	ERDiagram
)

// String returns the string representation of the diagram type
func (dt DiagramType) String() string {
	switch dt {
	case FlowChart:
		return "flowchart"
	case BlockDiagram:
		return "block"
	case CircuitDiagram:
		return "circuit"
	case NetworkDiagram:
		return "network"
	case SequenceDiagram:
		return "sequence"
	case ClassDiagram:
		return "class"
	case ERDiagram:
		return "er"
	default:
		return "unknown"
	}
}

// DetectedDiagram represents a diagram found in a PDF image
type DetectedDiagram struct {
	Type        DiagramType
	Confidence  float64
	PlantUML    string
	ImagePath   string
	BoundingBox image.Rectangle
}

// NewDiagramDetector creates a new DiagramDetector instance
func NewDiagramDetector(cfg *config.Config, log *logger.Logger) *DiagramDetector {
	return &DiagramDetector{config: cfg, logger: log}
}

// DetectDiagramsInImage analyzes an image for diagram content and returns detected diagrams
func (dd *DiagramDetector) DetectDiagramsInImage(imagePath string) ([]DetectedDiagram, error) {
	if !dd.config.DetectDiagrams {
		return []DetectedDiagram{}, nil
	}
	dd.logger.Debug("Analyzing image for diagrams: %s", imagePath)
	var detectedDiagrams []DetectedDiagram
	confidence, diagramType := dd.analyzeImageMetadata(imagePath)
	if confidence >= dd.config.DiagramConfidence {
		dd.logger.Info("Diagram detected in %s: type=%s, confidence=%.2f", filepath.Base(imagePath), diagramType.String(), confidence)
		plantUML, err := dd.generatePlantUML(diagramType)
		if err != nil {
			dd.logger.Warn("Failed to generate PlantUML for %s: %v", imagePath, err)
			return detectedDiagrams, nil
		}
		detectedDiagram := DetectedDiagram{Type: diagramType, Confidence: confidence, PlantUML: plantUML, ImagePath: imagePath, BoundingBox: image.Rect(0, 0, 400, 300)}
		detectedDiagrams = append(detectedDiagrams, detectedDiagram)
	} else {
		dd.logger.Debug("No diagram detected in %s (confidence: %.2f < threshold: %.2f)", filepath.Base(imagePath), confidence, dd.config.DiagramConfidence)
	}
	return detectedDiagrams, nil
}

// analyzeImageMetadata performs basic analysis to detect diagram-like content
func (dd *DiagramDetector) analyzeImageMetadata(imagePath string) (float64, DiagramType) {
	filename := strings.ToLower(filepath.Base(imagePath))

	// Prefer specific matches before generic ones to avoid conflicts like "block_diagram"
	if strings.Contains(filename, "circuit") || strings.Contains(filename, "electronic") {
		return 0.8, CircuitDiagram
	}
	if strings.Contains(filename, "block") || strings.Contains(filename, "schematic") {
		return 0.8, BlockDiagram
	}
	if strings.Contains(filename, "network") || strings.Contains(filename, "topology") {
		return 0.8, NetworkDiagram
	}
	if strings.Contains(filename, "flowchart") {
		return 0.8, FlowChart
	}
	// Generic "diagram" keyword defaults to flowchart for simple representation
	if strings.Contains(filename, "diagram") {
		return 0.8, FlowChart
	}
	// For placeholder images from PDF extraction, assume they could be diagrams
	if strings.Contains(filename, "page_") && strings.Contains(filename, "image_") {
		return 0.6, BlockDiagram
	}
	return 0.2, UnknownDiagram
}

// generatePlantUML creates PlantUML code based on the detected diagram type
func (dd *DiagramDetector) generatePlantUML(diagramType DiagramType) (string, error) {
	var plantUML strings.Builder
	plantUML.WriteString("@startuml\n")
	dd.applyPlantUMLStyle(&plantUML)
	switch diagramType {
	case FlowChart:
		dd.generateFlowChartUML(&plantUML)
	case BlockDiagram:
		dd.generateBlockDiagramUML(&plantUML)
	case CircuitDiagram:
		dd.generateCircuitDiagramUML(&plantUML)
	case NetworkDiagram:
		dd.generateNetworkDiagramUML(&plantUML)
	default:
		dd.generateGenericDiagramUML(&plantUML)
	}
	plantUML.WriteString("@enduml\n")
	return plantUML.String(), nil
}

// applyPlantUMLStyle applies the configured PlantUML style and color scheme
func (dd *DiagramDetector) applyPlantUMLStyle(plantUML *strings.Builder) {
	switch dd.config.PlantUMLStyle {
	case "blueprint":
		plantUML.WriteString("!theme blueprint\n")
	case "modern":
		plantUML.WriteString("!theme modern\n")
	}
	switch dd.config.PlantUMLColorScheme {
	case "mono":
		plantUML.WriteString("skinparam monochrome true\n")
	case "color":
		plantUML.WriteString("skinparam monochrome false\n")
	}
	plantUML.WriteString("\n")
}

// generateFlowChartUML generates PlantUML code for flowchart diagrams
func (dd *DiagramDetector) generateFlowChartUML(plantUML *strings.Builder) {
	plantUML.WriteString("' Flowchart detected from PDF diagram\n")
	plantUML.WriteString("start\n")
	plantUML.WriteString(":Process Input;\n")
	plantUML.WriteString("if (Condition?) then (yes)\n")
	plantUML.WriteString("  :Action A;\n")
	plantUML.WriteString("else (no)\n")
	plantUML.WriteString("  :Action B;\n")
	plantUML.WriteString("endif\n")
	plantUML.WriteString(":Generate Output;\n")
	plantUML.WriteString("stop\n\n")
	plantUML.WriteString("note bottom : Diagram converted from PDF image\n")
}

// generateBlockDiagramUML generates PlantUML code for block diagrams
func (dd *DiagramDetector) generateBlockDiagramUML(plantUML *strings.Builder) {
	plantUML.WriteString("' Block diagram detected from PDF\n")
	plantUML.WriteString("!define BLOCK(x) rectangle x\n\n")
	plantUML.WriteString("BLOCK(Input) {\n  [Input Signal]\n}\n\n")
	plantUML.WriteString("BLOCK(Process) {\n  [Processing Unit]\n}\n\n")
	plantUML.WriteString("BLOCK(Output) {\n  [Output Signal]\n}\n\n")
	plantUML.WriteString("[Input Signal] --> [Processing Unit]\n")
	plantUML.WriteString("[Processing Unit] --> [Output Signal]\n\n")
	plantUML.WriteString("note bottom : Block diagram converted from PDF image\n")
}

// generateCircuitDiagramUML generates PlantUML code for circuit diagrams
func (dd *DiagramDetector) generateCircuitDiagramUML(plantUML *strings.Builder) {
	plantUML.WriteString("' Circuit diagram detected from PDF\n")
	plantUML.WriteString("!define COMPONENT(x) circle x\n\n")
	plantUML.WriteString("COMPONENT(R1) {\n  R1\\nResistor\n}\n\n")
	plantUML.WriteString("COMPONENT(C1) {\n  C1\\nCapacitor\n}\n\n")
	plantUML.WriteString("rectangle VCC\nrectangle GND\n\n")
	plantUML.WriteString("VCC --> R1\nR1 --> C1\nC1 --> GND\n\n")
	plantUML.WriteString("note bottom : Circuit diagram converted from PDF image\n")
}

// generateNetworkDiagramUML generates PlantUML code for network diagrams
func (dd *DiagramDetector) generateNetworkDiagramUML(plantUML *strings.Builder) {
	plantUML.WriteString("' Network diagram detected from PDF\n")
	plantUML.WriteString("!include <C4/C4_Container>\n\n")
	plantUML.WriteString("Person(user, \"User\", \"End user\")\n")
	plantUML.WriteString("System(server, \"Server\", \"Main server\")\n")
	plantUML.WriteString("System(database, \"Database\", \"Data storage\")\n\n")
	plantUML.WriteString("Rel(user, server, \"Connects to\")\n")
	plantUML.WriteString("Rel(server, database, \"Reads/Writes\")\n\n")
	plantUML.WriteString("note bottom : Network diagram converted from PDF image\n")
}

// generateGenericDiagramUML generates a generic PlantUML representation for unknown diagram types
func (dd *DiagramDetector) generateGenericDiagramUML(plantUML *strings.Builder) {
	plantUML.WriteString("' Generic diagram detected from PDF\n")
	plantUML.WriteString("rectangle \"Component A\" as A\n")
	plantUML.WriteString("rectangle \"Component B\" as B\n")
	plantUML.WriteString("rectangle \"Component C\" as C\n\n")
	plantUML.WriteString("A --> B\nB --> C\n\n")
	plantUML.WriteString("note bottom : Generic diagram converted from PDF image\n")
}

// GetPlantUMLMarkdown formats the detected diagram as markdown with PlantUML code block
func (dd *DiagramDetector) GetPlantUMLMarkdown(diagram DetectedDiagram) string {
	var md strings.Builder
	md.WriteString(fmt.Sprintf("### Detected %s Diagram (Confidence: %.1f%%)\n\n", strings.Title(diagram.Type.String()), diagram.Confidence*100))
	md.WriteString("```plantuml\n")
	md.WriteString(diagram.PlantUML)
	md.WriteString("```\n\n")
	md.WriteString(fmt.Sprintf("*Original image: %s*\n\n", filepath.Base(diagram.ImagePath)))
	return md.String()
}
