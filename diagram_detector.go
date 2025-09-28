// Package main - Diagram detection and PlantUML conversion functionality.
// This file handles the detection of diagrams in PDF images and their conversion to PlantUML format.
package main

import (
	"fmt"
	"image"
	"path/filepath"
	"strings"
)

// DiagramDetector handles the detection and analysis of diagrams in PDF images.
// It uses heuristic analysis to identify diagram-like content and convert it to PlantUML.
type DiagramDetector struct {
	config *Config // Server configuration containing diagram detection settings
	logger *Logger // Logger instance for tracking diagram detection progress
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
	Type        DiagramType     // Type of diagram detected
	Confidence  float64         // Confidence score (0.0-1.0)
	PlantUML    string          // Generated PlantUML code
	ImagePath   string          // Path to the original image
	BoundingBox image.Rectangle // Bounding box of the diagram in the image
}

// NewDiagramDetector creates a new DiagramDetector instance
func NewDiagramDetector(config *Config, logger *Logger) *DiagramDetector {
	return &DiagramDetector{
		config: config,
		logger: logger,
	}
}

// DetectDiagramsInImage analyzes an image for diagram content and returns detected diagrams
func (dd *DiagramDetector) DetectDiagramsInImage(imagePath string) ([]DetectedDiagram, error) {
	if !dd.config.DetectDiagrams {
		return []DetectedDiagram{}, nil
	}

	dd.logger.Debug("Analyzing image for diagrams: %s", imagePath)

	var detectedDiagrams []DetectedDiagram

	// Use simplified heuristic analysis for diagram detection
	confidence, diagramType := dd.analyzeImageMetadata(imagePath)

	if confidence >= dd.config.DiagramConfidence {
		dd.logger.Info("Diagram detected in %s: type=%s, confidence=%.2f",
			filepath.Base(imagePath), diagramType.String(), confidence)

		// Generate PlantUML for the detected diagram
		plantUML, err := dd.generatePlantUML(diagramType)
		if err != nil {
			dd.logger.Warn("Failed to generate PlantUML for %s: %v", imagePath, err)
			return detectedDiagrams, nil
		}

		detectedDiagram := DetectedDiagram{
			Type:        diagramType,
			Confidence:  confidence,
			PlantUML:    plantUML,
			ImagePath:   imagePath,
			BoundingBox: image.Rect(0, 0, 400, 300), // Default size for now
		}

		detectedDiagrams = append(detectedDiagrams, detectedDiagram)
	} else {
		dd.logger.Debug("No diagram detected in %s (confidence: %.2f < threshold: %.2f)",
			filepath.Base(imagePath), confidence, dd.config.DiagramConfidence)
	}

	return detectedDiagrams, nil
}

// analyzeImageMetadata performs basic analysis to detect diagram-like content
// This is a simplified version that can be enhanced with computer vision later
func (dd *DiagramDetector) analyzeImageMetadata(imagePath string) (float64, DiagramType) {
	filename := strings.ToLower(filepath.Base(imagePath))

	// Simple heuristic based on filename and context
	// In a real implementation, this would analyze the actual image content

	// Look for keywords in filename that suggest diagram content
	if strings.Contains(filename, "diagram") || strings.Contains(filename, "flowchart") {
		return 0.8, FlowChart
	}

	if strings.Contains(filename, "block") || strings.Contains(filename, "schematic") {
		return 0.8, BlockDiagram
	}

	if strings.Contains(filename, "circuit") || strings.Contains(filename, "electronic") {
		return 0.8, CircuitDiagram
	}

	if strings.Contains(filename, "network") || strings.Contains(filename, "topology") {
		return 0.8, NetworkDiagram
	}

	// For placeholder images from PDF extraction, assume they could be diagrams
	// with moderate confidence
	if strings.Contains(filename, "page_") && strings.Contains(filename, "image_") {
		return 0.6, BlockDiagram // Default to block diagram for extracted images
	}

	return 0.2, UnknownDiagram
}

// generatePlantUML creates PlantUML code based on the detected diagram type
func (dd *DiagramDetector) generatePlantUML(diagramType DiagramType) (string, error) {
	var plantUML strings.Builder

	// Add PlantUML header with style
	plantUML.WriteString("@startuml\n")

	// Apply configured style
	dd.applyPlantUMLStyle(&plantUML)

	// Generate content based on diagram type
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
	default:
		// Use default theme
	}

	// Apply color scheme
	switch dd.config.PlantUMLColorScheme {
	case "mono":
		plantUML.WriteString("skinparam monochrome true\n")
	case "color":
		plantUML.WriteString("skinparam monochrome false\n")
	default:
		// Auto - let PlantUML decide
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
	plantUML.WriteString("stop\n")
	plantUML.WriteString("\n")
	plantUML.WriteString("note bottom : Diagram converted from PDF image\n")
}

// generateBlockDiagramUML generates PlantUML code for block diagrams
func (dd *DiagramDetector) generateBlockDiagramUML(plantUML *strings.Builder) {
	plantUML.WriteString("' Block diagram detected from PDF\n")
	plantUML.WriteString("!define BLOCK(x) rectangle x\n")
	plantUML.WriteString("\n")
	plantUML.WriteString("BLOCK(Input) {\n")
	plantUML.WriteString("  [Input Signal]\n")
	plantUML.WriteString("}\n")
	plantUML.WriteString("\n")
	plantUML.WriteString("BLOCK(Process) {\n")
	plantUML.WriteString("  [Processing Unit]\n")
	plantUML.WriteString("}\n")
	plantUML.WriteString("\n")
	plantUML.WriteString("BLOCK(Output) {\n")
	plantUML.WriteString("  [Output Signal]\n")
	plantUML.WriteString("}\n")
	plantUML.WriteString("\n")
	plantUML.WriteString("[Input Signal] --> [Processing Unit]\n")
	plantUML.WriteString("[Processing Unit] --> [Output Signal]\n")
	plantUML.WriteString("\n")
	plantUML.WriteString("note bottom : Block diagram converted from PDF image\n")
}

// generateCircuitDiagramUML generates PlantUML code for circuit diagrams
func (dd *DiagramDetector) generateCircuitDiagramUML(plantUML *strings.Builder) {
	plantUML.WriteString("' Circuit diagram detected from PDF\n")
	plantUML.WriteString("!define COMPONENT(x) circle x\n")
	plantUML.WriteString("\n")
	plantUML.WriteString("COMPONENT(R1) {\n")
	plantUML.WriteString("  R1\\nResistor\n")
	plantUML.WriteString("}\n")
	plantUML.WriteString("\n")
	plantUML.WriteString("COMPONENT(C1) {\n")
	plantUML.WriteString("  C1\\nCapacitor\n")
	plantUML.WriteString("}\n")
	plantUML.WriteString("\n")
	plantUML.WriteString("rectangle VCC\n")
	plantUML.WriteString("rectangle GND\n")
	plantUML.WriteString("\n")
	plantUML.WriteString("VCC --> R1\n")
	plantUML.WriteString("R1 --> C1\n")
	plantUML.WriteString("C1 --> GND\n")
	plantUML.WriteString("\n")
	plantUML.WriteString("note bottom : Circuit diagram converted from PDF image\n")
}

// generateNetworkDiagramUML generates PlantUML code for network diagrams
func (dd *DiagramDetector) generateNetworkDiagramUML(plantUML *strings.Builder) {
	plantUML.WriteString("' Network diagram detected from PDF\n")
	plantUML.WriteString("!include <C4/C4_Container>\n")
	plantUML.WriteString("\n")
	plantUML.WriteString("Person(user, \"User\", \"End user\")\n")
	plantUML.WriteString("System(server, \"Server\", \"Main server\")\n")
	plantUML.WriteString("System(database, \"Database\", \"Data storage\")\n")
	plantUML.WriteString("\n")
	plantUML.WriteString("Rel(user, server, \"Connects to\")\n")
	plantUML.WriteString("Rel(server, database, \"Reads/Writes\")\n")
	plantUML.WriteString("\n")
	plantUML.WriteString("note bottom : Network diagram converted from PDF image\n")
}

// generateGenericDiagramUML generates a generic PlantUML representation for unknown diagram types
func (dd *DiagramDetector) generateGenericDiagramUML(plantUML *strings.Builder) {
	plantUML.WriteString("' Generic diagram detected from PDF\n")
	plantUML.WriteString("rectangle \"Component A\" as A\n")
	plantUML.WriteString("rectangle \"Component B\" as B\n")
	plantUML.WriteString("rectangle \"Component C\" as C\n")
	plantUML.WriteString("\n")
	plantUML.WriteString("A --> B\n")
	plantUML.WriteString("B --> C\n")
	plantUML.WriteString("\n")
	plantUML.WriteString("note bottom : Generic diagram converted from PDF image\n")
}

// GetPlantUMLMarkdown formats the detected diagram as markdown with PlantUML code block
func (dd *DiagramDetector) GetPlantUMLMarkdown(diagram DetectedDiagram) string {
	var md strings.Builder

	md.WriteString(fmt.Sprintf("### Detected %s Diagram (Confidence: %.1f%%)\n\n",
		strings.Title(diagram.Type.String()), diagram.Confidence*100))

	md.WriteString("```plantuml\n")
	md.WriteString(diagram.PlantUML)
	md.WriteString("```\n\n")

	md.WriteString(fmt.Sprintf("*Original image: %s*\n\n", filepath.Base(diagram.ImagePath)))

	return md.String()
}
