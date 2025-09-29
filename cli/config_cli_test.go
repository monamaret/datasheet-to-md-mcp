package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	cfgpkg "datasheet-to-md-mcp/config"
)

// Helpers to capture stdout/stderr during tests
func captureStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	r.Close()
	return buf.String()
}

func captureStderr(fn func()) string {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	fn()

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	r.Close()
	return buf.String()
}

func TestConfigCLIGenerate(t *testing.T) {
	c := &ConfigCLI{}
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	// First generate should succeed
	if err := c.generate(path, false); err != nil {
		t.Fatalf("generate() failed: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading generated file failed: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "PDF to Markdown MCP Server Configuration") {
		t.Errorf("generated file missing header comment")
	}
	if !strings.Contains(content, "IMAGE_MAX_DPI=300") {
		t.Errorf("generated file missing default key=value for IMAGE_MAX_DPI")
	}

	// Second generate without force should error
	if err := c.generate(path, false); err == nil {
		t.Errorf("expected error when generating over existing file without --force")
	}

	// With force should overwrite
	if err := c.generate(path, true); err != nil {
		t.Fatalf("generate() with force failed: %v", err)
	}
}

func TestConfigCLISetAndGet(t *testing.T) {
	c := &ConfigCLI{}
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	// Set a valid known key
	if err := c.set(path, "IMAGE_MAX_DPI", "400"); err != nil {
		t.Fatalf("set valid key failed: %v", err)
	}

	// Setting an invalid value for a known key should error
	if err := c.set(path, "IMAGE_MAX_DPI", "1000"); err == nil {
		t.Errorf("expected validation error when setting invalid IMAGE_MAX_DPI")
	}

	// Set an unknown key should be allowed
	if err := c.set(path, "CUSTOM_VAR", "some value"); err != nil {
		t.Fatalf("set unknown key should succeed, got: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read env after set failed: %v", err)
	}
	if !strings.Contains(string(data), "IMAGE_MAX_DPI=400") {
		t.Errorf("env file should contain updated IMAGE_MAX_DPI, got:\n%s", string(data))
	}
	if !strings.Contains(string(data), "CUSTOM_VAR=some value") {
		t.Errorf("env file should contain CUSTOM_VAR, got:\n%s", string(data))
	}

	// get should print the value
	out := captureStdout(func() {
		if err := c.get(path, "IMAGE_MAX_DPI"); err != nil {
			t.Fatalf("get failed: %v", err)
		}
	})
	if strings.TrimSpace(out) != "400" {
		t.Errorf("expected get to print 400, got '%s'", strings.TrimSpace(out))
	}

	// get missing key should error
	if err := c.get(path, "MISSING_KEY"); err == nil {
		t.Errorf("expected error for missing key")
	}
}

func TestConfigCLIShowJSON(t *testing.T) {
	c := &ConfigCLI{}
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	// Prepare a minimal config file
	content := strings.Join([]string{
		"OUTPUT_BASE_DIR=/tmp/out",
		"IMAGE_MAX_DPI=350",
		"IMAGE_FORMAT=jpg",
		"PRESERVE_ASPECT_RATIO=false",
		"DETECT_DIAGRAMS=true",
		"DIAGRAM_CONFIDENCE=0.55",
		"PLANTUML_STYLE=modern",
		"PLANTUML_COLOR_SCHEME=color",
		"INCLUDE_TOC=false",
		"BASE_HEADER_LEVEL=3",
		"EXTRACT_TABLES=false",
		"EXTRACT_IMAGES=false",
		"LOG_LEVEL=debug",
		"MCP_TRANSPORT=stdio",
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed writing env file: %v", err)
	}

	out := captureStdout(func() {
		if err := c.show(path, "json"); err != nil {
			t.Fatalf("show json failed: %v", err)
		}
	})

	var cfg cfgpkg.Config
	if err := json.Unmarshal([]byte(out), &cfg); err != nil {
		t.Fatalf("failed to parse show json output: %v\nOutput: %s", err, out)
	}

	if cfg.OutputBaseDir != "/tmp/out" {
		t.Errorf("OutputBaseDir '/tmp/out', got '%s'", cfg.OutputBaseDir)
	}
	if cfg.ImageMaxDPI != 350 {
		t.Errorf("ImageMaxDPI 350, got %d", cfg.ImageMaxDPI)
	}
	if cfg.ImageFormat != "jpg" {
		t.Errorf("ImageFormat 'jpg', got '%s'", cfg.ImageFormat)
	}
	if cfg.PreserveAspectRatio {
		t.Errorf("PreserveAspectRatio false expected")
	}
	if !cfg.DetectDiagrams {
		t.Errorf("DetectDiagrams true expected")
	}
	if cfg.DiagramConfidence != 0.55 {
		t.Errorf("DiagramConfidence 0.55, got %v", cfg.DiagramConfidence)
	}
	if cfg.PlantUMLStyle != "modern" {
		t.Errorf("PlantUMLStyle 'modern', got '%s'", cfg.PlantUMLStyle)
	}
	if cfg.PlantUMLColorScheme != "color" {
		t.Errorf("PlantUMLColorScheme 'color', got '%s'", cfg.PlantUMLColorScheme)
	}
	if cfg.IncludeTOC {
		t.Errorf("IncludeTOC false expected")
	}
	if cfg.BaseHeaderLevel != 3 {
		t.Errorf("BaseHeaderLevel 3, got %d", cfg.BaseHeaderLevel)
	}
	if cfg.ExtractTables {
		t.Errorf("ExtractTables false expected")
	}
	if cfg.ExtractImages {
		t.Errorf("ExtractImages false expected")
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel 'debug', got '%s'", cfg.LogLevel)
	}
}

func TestConfigCLIShowEnvFormat(t *testing.T) {
	c := &ConfigCLI{}
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	// Prepare a partial config file; omitted values will use defaults
	content := strings.Join([]string{
		"OUTPUT_BASE_DIR=/tmp/out2",
		"IMAGE_MAX_DPI=280",
		"BASE_HEADER_LEVEL=2",
		"LOG_LEVEL=warn",
		"MCP_TRANSPORT=stdio",
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed writing env file: %v", err)
	}

	out := captureStdout(func() {
		if err := c.show(path, "env"); err != nil {
			t.Fatalf("show env failed: %v", err)
		}
	})

	if !strings.Contains(out, "OUTPUT_BASE_DIR=/tmp/out2") {
		t.Errorf("env output missing expected OUTPUT_BASE_DIR line. Output:\n%s", out)
	}
	if !strings.Contains(out, "IMAGE_MAX_DPI=280") {
		t.Errorf("env output missing expected IMAGE_MAX_DPI line. Output:\n%s", out)
	}
	if !strings.Contains(out, "BASE_HEADER_LEVEL=2") {
		t.Errorf("env output missing expected BASE_HEADER_LEVEL line. Output:\n%s", out)
	}
	if !strings.Contains(out, "LOG_LEVEL=warn") {
		t.Errorf("env output missing expected LOG_LEVEL line. Output:\n%s", out)
	}

	// Ensure lines are sorted lexicographically as implemented
	lines := make([]string, 0)
	for _, ln := range strings.Split(out, "\n") {
		ln = strings.TrimSpace(ln)
		if ln != "" {
			lines = append(lines, ln)
		}
	}
	sorted := make([]string, len(lines))
	copy(sorted, lines)
	sort.Strings(sorted)
	for i := range lines {
		if lines[i] != sorted[i] {
			t.Fatalf("env output lines are not sorted; expected %q at position %d, got %q. Full output:\n%s", sorted[i], i, lines[i], out)
		}
	}
}

func TestConfigCLIMissingFileErrors(t *testing.T) {
	c := &ConfigCLI{}
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.env")

	if err := c.show(missing, "json"); err == nil {
		t.Errorf("expected error when showing config from missing file")
	}

	if err := c.get(missing, "ANY"); err == nil {
		t.Errorf("expected error when getting key from missing file")
	}
}

func TestConfigCLIListKeys(t *testing.T) {
	c := &ConfigCLI{}
	out := captureStdout(func() { c.listKeys() })
	if !strings.Contains(out, "Available configuration keys:") {
		t.Errorf("listKeys header missing")
	}
	for _, item := range knownKeys {
		if !strings.Contains(out, item.Key) {
			t.Errorf("listKeys missing key %s", item.Key)
		}
	}
}

func TestRunHelpAndUnknown(t *testing.T) {
	c := &ConfigCLI{}
	out := captureStdout(func() { c.Run([]string{"help"}) })
	if !strings.Contains(out, "Usage:") {
		t.Errorf("help output should contain Usage:")
	}

	code := c.Run([]string{"not-a-command"})
	if code != 1 {
		t.Errorf("expected exit code 1 for unknown command, got %d", code)
	}
	errOut := captureStderr(func() { _ = c.Run([]string{"not-a-command"}) })
	if !strings.Contains(errOut, "Unknown command:") {
		t.Errorf("unknown command should print error to stderr, got: %s", errOut)
	}
}

func TestParseHelpers(t *testing.T) {
	file, force := parseFileFlag([]string{"-f", "cfg.env", "--force"})
	if file != "cfg.env" || !force {
		t.Errorf("parseFileFlag expected (cfg.env, true), got (%s, %v)", file, force)
	}

	fileOnly := parseOnlyFileFlag([]string{"--file=abc.env"})
	if fileOnly != "abc.env" {
		t.Errorf("parseOnlyFileFlag expected abc.env, got %s", fileOnly)
	}

	format := parseFormatFlag([]string{"--format=json"}, "env")
	if format != "json" {
		t.Errorf("parseFormatFlag expected json, got %s", format)
	}

	key, val, err := parseKeyValue([]string{"-f", "x.env", "KEY", "some value"})
	if err != nil {
		t.Fatalf("parseKeyValue unexpected error: %v", err)
	}
	if key != "KEY" || val != "some value" {
		t.Errorf("parseKeyValue expected (KEY, 'some value'), got (%s, %s)", key, val)
	}

	_, _, err = parseKeyValue([]string{"ONLYKEY"})
	if err == nil {
		t.Errorf("parseKeyValue expected error when KEY is provided without VALUE")
	}
}

func TestValidateValue(t *testing.T) {
	if err := validateValue("IMAGE_MAX_DPI", "71"); err == nil {
		t.Errorf("expected error for IMAGE_MAX_DPI below range")
	}
	if err := validateValue("IMAGE_FORMAT", "bmp"); err == nil {
		t.Errorf("expected error for unsupported IMAGE_FORMAT")
	}
	if err := validateValue("DIAGRAM_CONFIDENCE", "1.5"); err == nil {
		t.Errorf("expected error for DIAGRAM_CONFIDENCE out of range")
	}
	if err := validateValue("BASE_HEADER_LEVEL", "0"); err == nil {
		t.Errorf("expected error for BASE_HEADER_LEVEL out of range")
	}
	if err := validateValue("LOG_LEVEL", "verbose"); err == nil {
		t.Errorf("expected error for invalid LOG_LEVEL")
	}
	if err := validateValue("MCP_TRANSPORT", "tcp"); err == nil {
		t.Errorf("expected error for invalid MCP_TRANSPORT")
	}
	if err := validateValue("PLANTUML_STYLE", "fancy"); err == nil {
		t.Errorf("expected error for invalid PLANTUML_STYLE")
	}
	if err := validateValue("PLANTUML_COLOR_SCHEME", "vivid"); err == nil {
		t.Errorf("expected error for invalid PLANTUML_COLOR_SCHEME")
	}
}

func TestConfigToEnvPairs(t *testing.T) {
	c := &ConfigCLI{}
	cfg := &cfgpkg.Config{
		PDFInputDir:         "/in",
		OutputBaseDir:       "/out",
		ServerName:          "srv",
		ServerVersion:       "v1",
		ImageMaxDPI:         200,
		ImageFormat:         "png",
		PreserveAspectRatio: true,
		DetectDiagrams:      true,
		DiagramConfidence:   0.9,
		PlantUMLStyle:       "default",
		PlantUMLColorScheme: "auto",
		IncludeTOC:          true,
		BaseHeaderLevel:     2,
		ExtractTables:       false,
		ExtractImages:       true,
		LogLevel:            "info",
		Transport:           "stdio",
	}
	pairs := c.configToEnvPairs(cfg)
	joined := strings.Join(pairs, "\n")
	checks := []string{
		"PDF_INPUT_DIR=/in",
		"OUTPUT_BASE_DIR=/out",
		"MCP_SERVER_NAME=srv",
		"IMAGE_MAX_DPI=200",
		"INCLUDE_TOC=true",
		"EXTRACT_TABLES=false",
	}
	for _, want := range checks {
		if !strings.Contains(joined, want) {
			t.Errorf("configToEnvPairs missing '%s' in: \n%s", want, joined)
		}
	}
}

func TestConfigCLIShowInvalidValues(t *testing.T) {
	c := &ConfigCLI{}
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	// Invalid IMAGE_MAX_DPI should cause validation error
	content := strings.Join([]string{
		"IMAGE_MAX_DPI=1000", // > 600 invalid
		"MCP_TRANSPORT=stdio",
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed writing env file: %v", err)
	}
	if err := c.show(path, "env"); err == nil {
		t.Errorf("expected error from show with invalid IMAGE_MAX_DPI")
	}

	// Overwrite with invalid IMAGE_FORMAT
	content = strings.Join([]string{
		"IMAGE_FORMAT=gif", // unsupported
		"MCP_TRANSPORT=stdio",
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed writing env file: %v", err)
	}
	if err := c.show(path, "json"); err == nil {
		t.Errorf("expected error from show with invalid IMAGE_FORMAT")
	}
}
