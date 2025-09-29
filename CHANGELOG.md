# Changelog

All notable changes to this project will be documented in this file.

## v1.0.0 - 2025-09-29

Initial stable release of Datasheet (PDF) to Markdown MCP Server.

### Added
- PDF to Markdown conversion with structured headers and formatting
- Diagram detection with automatic PlantUML generation (optional)
- Batch directory processing for multiple PDFs
- Image extraction and export (PNG), with configurable DPI and aspect ratio
- MCP server over stdio for integration with AI coding assistants
- Environment-driven configuration via .env
- Structured output directory with MARKDOWN_ prefix per input file
- Optional Table of Contents generation
- Comprehensive, configurable logging and error handling

### Platforms
- Windows (amd64)
- macOS (Intel amd64 and Apple Silicon arm64)
- Raspberry Pi/Linux (arm/arm64)

### Tooling/CI
- GitHub Actions workflow to build cross-platform binaries and create releases on tags
- Makefile targets for build, test, lint, and multi-platform builds

### Notes
- See README.md for installation, configuration, and usage details
- Default configuration example provided in pdf_md_mcp.env
