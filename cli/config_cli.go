// Package cli provides a small command-line interface for managing the server configuration file (.env style)
// without external CLI libraries. It supports generating a new config, showing the current config, and
// updating individual settings. The help text is derived from the project's README documentation.
package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/joho/godotenv"

	"datasheet-to-md-mcp/config"
)

// ConfigCLI implements a minimal CLI for configuration file management.
type ConfigCLI struct{}

// Run executes the config CLI with the provided arguments.
//
// Subcommands:
//   - generate [-f <file>]            Create a new config file with default values (non-destructive if exists unless --force)
//   - create <file>                   Create a new config file at the specified path (required)
//   - show [-f <file>] [--format <env|json>]   Print the resolved config from the file
//   - set [-f <file>] KEY VALUE        Update or add a setting in the config file
//   - get [-f <file>] KEY              Print a specific setting value from the config file
//   - list-keys                        Print available keys with descriptions and defaults
//   - help                             Show usage
func (c *ConfigCLI) Run(args []string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		c.printHelp()
		return 0
	}

	sub := args[0]
	switch sub {
	case "generate":
		file, force := parseFileFlag(args[1:])
		if err := c.generate(file, force); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		fmt.Fprintf(os.Stdout, "Config created at %s\n", file)
		return 0
	case "create":
		file, force, err := parseCreateArgs(args[1:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		if err := c.create(file, force); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		fmt.Fprintf(os.Stdout, "Config created at %s\n", file)
		return 0
	case "show":
		file := parseOnlyFileFlag(args[1:])
		format := parseFormatFlag(args[1:], "env")
		if err := c.show(file, format); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		return 0
	case "set":
		file, _ := parseFileFlag(args[1:])
		key, val, err := parseKeyValue(args[1:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		if err := c.set(file, key, val); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		fmt.Fprintf(os.Stdout, "Updated %s in %s\n", key, file)
		return 0
	case "get":
		file := parseOnlyFileFlag(args[1:])
		key, _, err := parseKeyValue(args[1:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		if err := c.get(file, key); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		return 0
	case "list-keys":
		c.listKeys()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", sub)
		c.printHelp()
		return 1
	}
}

func (c *ConfigCLI) printHelp() {
	fmt.Print(`pdf-md-mcp config - Manage server configuration (.env)

Usage:
  pdf-md-mcp config help
  pdf-md-mcp config generate [-f <file>] [--force]
  pdf-md-mcp config create <file> [--force]
  pdf-md-mcp config show [-f <file>] [--format env|json]
  pdf-md-mcp config set [-f <file>] KEY VALUE
  pdf-md-mcp config get [-f <file>] KEY
  pdf-md-mcp config list-keys

Description:
  Manage the environment-based configuration used by the PDF→Markdown MCP server.
  This CLI helps you generate a new config file, inspect current values, and update
  individual settings. Configuration keys and defaults are aligned with the project's documentation.

Commands:
  generate    Create a new config file with optional -f flag (defaults to .env)
  create      Create a new config file at the specified path (path required)
  show        Display configuration from file
  set         Update a configuration value
  get         Read a single configuration value
  list-keys   Show all available configuration keys

Examples:
  # Create a new .env file using defaults (will not overwrite existing file)
  pdf-md-mcp config generate -f .env

  # Create a new config file at a specific path (path is required)
  pdf-md-mcp config create /path/to/config.env

  # Create and overwrite if exists
  pdf-md-mcp config create /path/to/config.env --force

  # Print the resolved config from a file as .env lines
  pdf-md-mcp config show -f .env

  # Print the resolved config as JSON
  pdf-md-mcp config show -f .env --format json

  # Update a value
  pdf-md-mcp config set -f .env IMAGE_MAX_DPI 300

  # Read a single value
  pdf-md-mcp config get -f .env LOG_LEVEL

  # See available keys with descriptions
  pdf-md-mcp config list-keys
`)
}

// knownKeys defines supported configuration keys, their descriptions (from README), and default values.
// This allows generating and validating config files without external libraries.
var knownKeys = []struct {
	Key         string
	Description string
	Default     string
}{
	{"PDF_INPUT_DIR", "Directory containing PDF files to process", ""},
	{"OUTPUT_BASE_DIR", "Base output directory", "./output"},
	{"MCP_SERVER_NAME", "Server identification name", "pdf-to-markdown-server"},
	{"MCP_SERVER_VERSION", "Server version", "1.0.0"},
	{"IMAGE_MAX_DPI", "Maximum image resolution (72-600)", "300"},
	{"IMAGE_FORMAT", "Image output format (png/jpg)", "png"},
	{"PRESERVE_ASPECT_RATIO", "Maintain image aspect ratios", "true"},
	{"DETECT_DIAGRAMS", "Enable diagram detection and PlantUML generation", "false"},
	{"DIAGRAM_CONFIDENCE", "Minimum confidence for diagram detection (0.0-1.0)", "0.7"},
	{"PLANTUML_STYLE", "PlantUML diagram style (default/blueprint/modern)", "default"},
	{"PLANTUML_COLOR_SCHEME", "PlantUML color scheme (mono/color/auto)", "auto"},
	{"INCLUDE_TOC", "Generate table of contents", "true"},
	{"BASE_HEADER_LEVEL", "Starting header level (1-6)", "1"},
	{"EXTRACT_TABLES", "Enable table extraction", "true"},
	{"EXTRACT_IMAGES", "Enable image extraction", "true"},
	{"LOG_LEVEL", "Logging verbosity (debug/info/warn/error)", "info"},
	{"MCP_TRANSPORT", "Transport method for MCP communication (stdio)", "stdio"},
}

func defaultFilePath() string {
	return ".env"
}

// generate creates a new env file with default values and helpful comments.
func (c *ConfigCLI) generate(path string, force bool) error {
	if path == "" {
		path = defaultFilePath()
	}
	if _, err := os.Stat(path); err == nil && !force {
		return fmt.Errorf("file already exists: %s (use --force to overwrite)", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	defer w.Flush()

	// Use ConfigExample to get the formatted configuration content
	content := config.ConfigExample()
	_, err = w.WriteString(content)
	return err
}

// create creates a new env file at the specified path with default values.
func (c *ConfigCLI) create(path string, force bool) error {
	if path == "" {
		return errors.New("file path required for create command")
	}
	if _, err := os.Stat(path); err == nil && !force {
		return fmt.Errorf("file already exists: %s (use --force to overwrite)", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	defer w.Flush()

	// Use ConfigExample to get the formatted configuration content
	content := config.ConfigExample()
	_, err = w.WriteString(content)
	return err
}

// show loads the config file and prints the resolved configuration using the project's validation.
func (c *ConfigCLI) show(path string, format string) error {
	if path == "" {
		path = defaultFilePath()
	}

	// Read file into env map (do not modify actual process environment yet)
	envMap, err := godotenv.Read(path)
	if err != nil {
		return fmt.Errorf("failed to read config file '%s': %w", path, err)
	}

	// Temporarily apply to environment to reuse config.LoadConfig validation
	restore := snapshotEnv()
	defer restore()
	for k, v := range envMap {
		os.Setenv(k, v)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	switch strings.ToLower(format) {
	case "json":
		b, _ := json.MarshalIndent(cfg, "", "  ")
		fmt.Println(string(b))
	default: // env-like output
		pairs := c.configToEnvPairs(cfg)
		sort.Strings(pairs)
		for _, line := range pairs {
			fmt.Println(line)
		}
	}
	return nil
}

// set updates or adds a key=value in the env file.
func (c *ConfigCLI) set(path, key, value string) error {
	if path == "" {
		path = defaultFilePath()
	}
	if key == "" {
		return errors.New("missing KEY")
	}

	// Load existing env (if file missing, start fresh)
	envMap := map[string]string{}
	if _, err := os.Stat(path); err == nil {
		m, err := godotenv.Read(path)
		if err != nil {
			return fmt.Errorf("failed to read existing config: %w", err)
		}
		for k, v := range m {
			envMap[k] = v
		}
	}

	// Validate known keys if possible
	if isKnownKey(key) {
		if err := validateValue(key, value); err != nil {
			return err
		}
	}

	envMap[key] = value
	return writeEnvFile(path, envMap)
}

// get prints a single value from the env file.
func (c *ConfigCLI) get(path, key string) error {
	if path == "" {
		path = defaultFilePath()
	}
	if key == "" {
		return errors.New("missing KEY")
	}
	m, err := godotenv.Read(path)
	if err != nil {
		return fmt.Errorf("failed to read config file '%s': %w", path, err)
	}
	if v, ok := m[key]; ok {
		fmt.Println(v)
		return nil
	}
	return fmt.Errorf("key not found: %s", key)
}

func (c *ConfigCLI) listKeys() {
	fmt.Println("Available configuration keys:")
	for _, item := range knownKeys {
		fmt.Printf("- %s: %s (default: %s)\n", item.Key, item.Description, item.Default)
	}
}

// Helpers

func parseFileFlag(args []string) (string, bool) {
	file := defaultFilePath()
	force := false
	for i := 0; i < len(args); i++ {
		if args[i] == "-f" || strings.HasPrefix(args[i], "--file=") {
			if args[i] == "-f" && i+1 < len(args) {
				file = args[i+1]
				i++
				continue
			}
			if strings.HasPrefix(args[i], "--file=") {
				file = strings.TrimPrefix(args[i], "--file=")
				continue
			}
		}
		if args[i] == "--force" {
			force = true
		}
	}
	return file, force
}

func parseCreateArgs(args []string) (string, bool, error) {
	if len(args) == 0 {
		return "", false, errors.New("file path required for create command")
	}
	file := args[0]
	force := false
	for i := 1; i < len(args); i++ {
		if args[i] == "--force" {
			force = true
		}
	}
	return file, force, nil
}

func parseOnlyFileFlag(args []string) string {
	file := defaultFilePath()
	for i := 0; i < len(args); i++ {
		if args[i] == "-f" && i+1 < len(args) {
			file = args[i+1]
			i++
			continue
		}
		if strings.HasPrefix(args[i], "--file=") {
			file = strings.TrimPrefix(args[i], "--file=")
		}
	}
	return file
}

func parseFormatFlag(args []string, def string) string {
	format := def
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "--format=") {
			format = strings.TrimPrefix(args[i], "--format=")
		}
	}
	return format
}

func parseKeyValue(args []string) (string, string, error) {
	// Find key and value after removing flags (-f, --file, --format)
	filtered := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		s := args[i]
		if s == "-f" {
			i++ // skip value
			continue
		}
		if strings.HasPrefix(s, "--file=") || strings.HasPrefix(s, "--format=") {
			continue
		}
		filtered = append(filtered, s)
	}
	if len(filtered) < 2 {
		return "", "", errors.New("usage: ... KEY VALUE")
	}
	return filtered[0], strings.Join(filtered[1:], " "), nil
}

func isKnownKey(key string) bool {
	for _, k := range knownKeys {
		if k.Key == key {
			return true
		}
	}
	return false
}

func validateValue(key, value string) error {
	switch key {
	case "IMAGE_MAX_DPI":
		v, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("%s must be an integer: %v", key, err)
		}
		if v < 72 || v > 600 {
			return fmt.Errorf("%s must be between 72 and 600", key)
		}
	case "IMAGE_FORMAT":
		vv := strings.ToLower(value)
		if vv != "png" && vv != "jpg" {
			return fmt.Errorf("%s must be 'png' or 'jpg'", key)
		}
	case "DIAGRAM_CONFIDENCE":
		f, err := strconv.ParseFloat(value, 64)
		if err != nil || f < 0.0 || f > 1.0 {
			return fmt.Errorf("%s must be a number between 0.0 and 1.0", key)
		}
	case "BASE_HEADER_LEVEL":
		v, err := strconv.Atoi(value)
		if err != nil || v < 1 || v > 6 {
			return fmt.Errorf("%s must be an integer between 1 and 6", key)
		}
	case "LOG_LEVEL":
		vv := strings.ToLower(value)
		if !inSet(vv, []string{"debug", "info", "warn", "error"}) {
			return fmt.Errorf("%s must be one of: debug, info, warn, error", key)
		}
	case "MCP_TRANSPORT":
		vv := strings.ToLower(value)
		if !inSet(vv, []string{"stdio"}) {
			return fmt.Errorf("%s must be: stdio", key)
		}
	case "PLANTUML_STYLE":
		vv := strings.ToLower(value)
		if !inSet(vv, []string{"default", "blueprint", "modern"}) {
			return fmt.Errorf("%s must be one of: default, blueprint, modern", key)
		}
	case "PLANTUML_COLOR_SCHEME":
		vv := strings.ToLower(value)
		if !inSet(vv, []string{"mono", "color", "auto"}) {
			return fmt.Errorf("%s must be one of: mono, color, auto", key)
		}
	}
	return nil
}

func inSet(v string, set []string) bool {
	for _, s := range set {
		if s == v {
			return true
		}
	}
	return false
}

func writeEnvFile(path string, env map[string]string) error {
	// Write keys in a stable order: known keys first, then any extras sorted
	order := make([]string, 0, len(knownKeys))
	for _, k := range knownKeys {
		order = append(order, k.Key)
	}
	extras := make([]string, 0)
	for k := range env {
		if !isKnownKey(k) {
			extras = append(extras, k)
		}
	}
	sort.Strings(extras)

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to open config for writing: %w", err)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	defer w.Flush()

	w.WriteString("# Generated by pdf-md-mcp config CLI\n")
	w.WriteString("# Edit values as needed.\n\n")

	for _, k := range order {
		if v, ok := env[k]; ok {
			w.WriteString(fmt.Sprintf("%s=%s\n", k, v))
		}
	}
	if len(extras) > 0 {
		w.WriteString("\n# Additional settings\n")
		for _, k := range extras {
			w.WriteString(fmt.Sprintf("%s=%s\n", k, env[k]))
		}
	}
	return nil
}

func (c *ConfigCLI) configToEnvPairs(cfg *config.Config) []string {
	pairs := []string{
		fmt.Sprintf("PDF_INPUT_DIR=%s", cfg.PDFInputDir),
		fmt.Sprintf("OUTPUT_BASE_DIR=%s", cfg.OutputBaseDir),
		fmt.Sprintf("MCP_SERVER_NAME=%s", cfg.ServerName),
		fmt.Sprintf("MCP_SERVER_VERSION=%s", cfg.ServerVersion),
		fmt.Sprintf("IMAGE_MAX_DPI=%d", cfg.ImageMaxDPI),
		fmt.Sprintf("IMAGE_FORMAT=%s", cfg.ImageFormat),
		fmt.Sprintf("PRESERVE_ASPECT_RATIO=%t", cfg.PreserveAspectRatio),
		fmt.Sprintf("DETECT_DIAGRAMS=%t", cfg.DetectDiagrams),
		fmt.Sprintf("DIAGRAM_CONFIDENCE=%g", cfg.DiagramConfidence),
		fmt.Sprintf("PLANTUML_STYLE=%s", cfg.PlantUMLStyle),
		fmt.Sprintf("PLANTUML_COLOR_SCHEME=%s", cfg.PlantUMLColorScheme),
		fmt.Sprintf("INCLUDE_TOC=%t", cfg.IncludeTOC),
		fmt.Sprintf("BASE_HEADER_LEVEL=%d", cfg.BaseHeaderLevel),
		fmt.Sprintf("EXTRACT_TABLES=%t", cfg.ExtractTables),
		fmt.Sprintf("EXTRACT_IMAGES=%t", cfg.ExtractImages),
		fmt.Sprintf("LOG_LEVEL=%s", cfg.LogLevel),
		fmt.Sprintf("MCP_TRANSPORT=%s", cfg.Transport),
	}
	return pairs
}

// snapshotEnv captures current environment variables and returns a restore function.
func snapshotEnv() func() {
	orig := os.Environ()
	return func() {
		// Clear and restore
		for _, kv := range os.Environ() {
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) == 2 {
				os.Unsetenv(parts[0])
			}
		}
		for _, kv := range orig {
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) == 2 {
				os.Setenv(parts[0], parts[1])
			}
		}
	}
}
