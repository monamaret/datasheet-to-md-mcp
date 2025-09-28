# Configuration and Code Changes Summary

## Overview
Modified the PDF to Markdown MCP server to process multiple PDF files from a directory instead of requiring a specific single file path.

## Configuration Changes

### Environment Variables
- **Changed**: `PDF_INPUT_PATH` → `PDF_INPUT_DIR`
  - Old: Path to a single PDF file
  - New: Directory path containing PDF files to process

### Configuration Files
- Updated `pdf_md_mcp.env` to use `PDF_INPUT_DIR`
- Updated documentation and comments throughout codebase

## Code Changes

### New Functionality Added
1. **Directory Processing**: Added `ConvertPDFsInDirectory()` method to process all PDFs in a directory
2. **Batch Results**: New `BatchConversionResult` struct to track multiple conversions
3. **Error Handling**: Added `ConversionError` struct for individual file failures
4. **File Discovery**: Added `findPDFFiles()` method to recursively find PDF files

### MCP Tool Updates
1. **New Tool**: Added `convert_pdfs_in_directory` tool alongside existing `convert_pdf_to_markdown`
2. **Tool Parameters**:
   - `convert_pdf_to_markdown`: `pdf_path` (required), `output_dir` (optional)
   - `convert_pdfs_in_directory`: `input_dir` (required), `output_dir` (optional)

### Modified Files
- `config.go`: Updated Config struct and LoadConfig function
- `pdf_converter.go`: Added batch processing methods and data structures
- `mcp_handler.go`: Updated tool handlers and response formatting
- `main.go`: Updated tool handling logic
- `pdf_md_mcp.env`: Changed environment variable name
- `README.md`: Updated documentation for new functionality

## Features
- **Backward Compatibility**: Single file processing still works via `convert_pdf_to_markdown` tool
- **Recursive Search**: Finds PDF files in subdirectories
- **Error Resilience**: Continues processing other files if individual conversions fail
- **Detailed Reporting**: Comprehensive success/failure statistics
- **Flexible Configuration**: Can specify different input and output directories per call

## Usage Examples

### Single File (unchanged)
```json
{
  "name": "convert_pdf_to_markdown",
  "arguments": {
    "pdf_path": "/path/to/datasheet.pdf",
    "output_dir": "./output"
  }
}
```

### Directory Processing (new)
```json
{
  "name": "convert_pdfs_in_directory",
  "arguments": {
    "input_dir": "/path/to/datasheets/",
    "output_dir": "./output"
  }
}
```

## Output Structure
Each PDF file creates its own `MARKDOWN_<filename>/` directory containing:
- `README.md` - Generated Markdown content
- `image_*.png` - Extracted images (if any)

When processing a directory, multiple such directories are created in the output location.