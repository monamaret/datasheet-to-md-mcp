package pdfconv

import (
	"image/png"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jung-kurt/gofpdf"

	"datasheet-to-md-mcp/config"
	"datasheet-to-md-mcp/logger"
	"datasheet-to-md-mcp/uml"
)

func TestNewPDFConverter(t *testing.T) {
	cfg := &config.Config{IncludeTOC: true, BaseHeaderLevel: 1, ExtractImages: true, DetectDiagrams: true, DiagramConfidence: 0.6}
	logr := logger.NewLogger("debug")
	conv, err := NewPDFConverter(cfg, logr)
	if err != nil {
		t.Fatalf("NewPDFConverter() error = %v", err)
	}
	if conv == nil {
		t.Fatal("NewPDFConverter() returned nil")
	}
	if conv.config != cfg {
		t.Error("converter.config not set correctly")
	}
	if conv.logger != logr {
		t.Error("converter.logger not set correctly")
	}
	if conv.diagramDetector == nil {
		t.Error("converter.diagramDetector should be initialized")
	}
}

func TestCreateOutputDirectory(t *testing.T) {
	cfg := &config.Config{}
	logr := logger.NewLogger("error")
	conv, _ := NewPDFConverter(cfg, logr)
	tempBase := t.TempDir()
	pdfPath := filepath.Join(tempBase, "sample.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.4\n%"), 0644); err != nil {
		t.Fatalf("failed to create temp pdf: %v", err)
	}
	out, err := conv.createOutputDirectory(pdfPath, tempBase)
	if err != nil {
		t.Fatalf("createOutputDirectory() error = %v", err)
	}
	if !strings.Contains(out, "MARKDOWN_sample") {
		t.Errorf("expected output dir to contain MARKDOWN_sample, got %s", out)
	}
	if stat, err := os.Stat(out); err != nil || !stat.IsDir() {
		t.Errorf("expected output dir to exist, got err=%v isDir=%v", err, err == nil && stat.IsDir())
	}
}

func TestGenerateTableOfContents(t *testing.T) {
	cfg := &config.Config{}
	logr := logger.NewLogger("error")
	conv, _ := NewPDFConverter(cfg, logr)
	pages := []PDFPage{{Number: 1}, {Number: 2}, {Number: 3}}
	toc := conv.generateTableOfContents(pages)
	if !strings.HasPrefix(toc, "## Table of Contents") {
		t.Errorf("TOC should start with header, got: %s", toc)
	}
	for i := 1; i <= 3; i++ {
		anchor := filepath.ToSlash("#page-" + strconvI(i))
		if !strings.Contains(strings.ToLower(toc), anchor) {
			t.Errorf("TOC missing anchor for page %d: %s", i, toc)
		}
	}
}

func TestFormatTextContentAndHeaderDetection(t *testing.T) {
	cfg := &config.Config{BaseHeaderLevel: 1}
	logr := logger.NewLogger("error")
	conv, _ := NewPDFConverter(cfg, logr)
	text := "OVERVIEW:\n\nThis is an intro.\n\nFEATURES\n- item 1\n- item 2\n\nVery long line that should definitely not be a header because it exceeds sixty characters in length.\nShort UPPERCASE\n"
	formatted := conv.formatTextContent(text)
	if !strings.Contains(formatted, "### OVERVIEW:") {
		t.Errorf("expected OVERVIEW to be formatted as header, got: %s", formatted)
	}
	if !strings.Contains(formatted, "### FEATURES") {
		t.Errorf("expected FEATURES to be formatted as header, got: %s", formatted)
	}
	if strings.Contains(formatted, "### Very long line") {
		t.Error("unexpected header detected for long line")
	}
	if !strings.Contains(formatted, "Short UPPERCASE") {
		t.Error("expected regular line to remain present")
	}
}

func TestLooksLikeHeader(t *testing.T) {
	cfg := &config.Config{}
	logr := logger.NewLogger("error")
	conv, _ := NewPDFConverter(cfg, logr)
	cases := []struct {
		line string
		want bool
	}{
		{"OVERVIEW:", true}, {"FEATURES", true}, {"network topology", false}, {"THIS IS SHORT", true}, {"this is a lowercase line", false}, {"A very very very long line that should not be considered a header because it is too long and verbose", false},
	}
	for _, c := range cases {
		if got := conv.looksLikeHeader(c.line); got != c.want {
			t.Errorf("looksLikeHeader(%q) = %v, want %v", c.line, got, c.want)
		}
	}
}

func TestGenerateMarkdown_WithTOC_Images_Diagrams(t *testing.T) {
	cfg := &config.Config{IncludeTOC: true, BaseHeaderLevel: 1}
	logr := logger.NewLogger("error")
	conv, _ := NewPDFConverter(cfg, logr)

	pages := []PDFPage{
		{
			Number: 1,
			Text:   "OVERVIEW:\nSome text on page 1.",
			Images: []PDFImage{
				{
					Filename: "page_1_image_1.png",
					Diagrams: []uml.DetectedDiagram{{
						Type:       uml.FlowChart,
						Confidence: 0.9,
						PlantUML:   "@startuml\nstart\nstop\n@enduml\n",
						ImagePath:  "/tmp/page_1_image_1.png",
					}},
				},
			},
		},
		{Number: 2, Text: "Second page text."},
	}

	md := conv.generateMarkdown(pages)
	if !strings.HasPrefix(md, "# PDF Document\n\n") {
		t.Errorf("markdown should start with document title, got: %q", md[:min(40, len(md))])
	}
	if !strings.Contains(md, "## Table of Contents") {
		t.Error("expected TOC in markdown")
	}
	if !strings.Contains(md, "![Image](./page_1_image_1.png)") {
		t.Error("expected image reference in markdown")
	}
	// Page separator should appear between pages but not after the last one
	if strings.Count(md, "---\n\n") != 1 {
		t.Errorf("expected exactly one page separator, got %d", strings.Count(md, "---\n\n"))
	}
}

func TestWriteMarkdownFile(t *testing.T) {
	cfg := &config.Config{}
	logr := logger.NewLogger("error")
	conv, _ := NewPDFConverter(cfg, logr)
	t.Run("success", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "README.md")
		content := "# Title\n\nHello"
		if err := conv.writeMarkdownFile(path, content); err != nil {
			t.Fatalf("writeMarkdownFile() error = %v", err)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read writen file: %v", err)
		}
		if string(b) != content {
			t.Errorf("content mismatch: got %q want %q", string(b), content)
		}
	})
	t.Run("parent missing", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing", "README.md")
		err := conv.writeMarkdownFile(path, "content")
		if err == nil {
			t.Fatal("expected error when parent directory is missing")
		}
	})
}

func TestCreatePlaceholderAndSaveImage(t *testing.T) {
	cfg := &config.Config{}
	logr := logger.NewLogger("error")
	conv, _ := NewPDFConverter(cfg, logr)
	img := conv.createPlaceholderImage(123, 77)
	if img.Bounds().Dx() != 123 || img.Bounds().Dy() != 77 {
		t.Errorf("unexpected image size: %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
	path := filepath.Join(t.TempDir(), "img.png")
	if err := conv.saveImage(img, path); err != nil {
		t.Fatalf("saveImage() error = %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open saved image: %v", err)
	}
	defer f.Close()
	if _, err := png.Decode(f); err != nil {
		t.Fatalf("saved file is not a valid PNG: %v", err)
	}
}

func TestFindPDFFiles(t *testing.T) {
	cfg := &config.Config{}
	logr := logger.NewLogger("error")
	conv, _ := NewPDFConverter(cfg, logr)
	root := t.TempDir()
	paths := []string{filepath.Join(root, "a.PDF"), filepath.Join(root, "b.pdf"), filepath.Join(root, "c.txt"), filepath.Join(root, "sub", "d.pdf")}
	for _, p := range paths {
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if filepath.Ext(p) != ".txt" {
			if err := os.WriteFile(p, []byte("%PDF-1.4\n%"), 0644); err != nil {
				t.Fatalf("write pdf: %v", err)
			}
		} else {
			if err := os.WriteFile(p, []byte("hello"), 0644); err != nil {
				t.Fatalf("write txt: %v", err)
			}
		}
	}
	found, err := conv.findPDFFiles(root)
	if err != nil {
		t.Fatalf("findPDFFiles() error = %v", err)
	}
	if len(found) != 3 {
		t.Fatalf("expected 3 pdfs, got %d: %v", len(found), found)
	}
	for _, p := range found {
		if !strings.EqualFold(filepath.Ext(p), ".pdf") {
			t.Errorf("found non-pdf file: %s", p)
		}
		if !filepath.IsAbs(p) {
			t.Errorf("expected absolute path, got: %s", p)
		}
	}
}

func TestConvertPDF_NonExistentFile(t *testing.T) {
	cfg := &config.Config{IncludeTOC: true}
	logr := logger.NewLogger("error")
	conv, _ := NewPDFConverter(cfg, logr)
	_, err := conv.ConvertPDF("/path/does/not/exist.pdf", t.TempDir())
	if err == nil {
		t.Fatal("expected error for non-existent pdf")
	}
}

func TestConvertPDF_OutputBaseIsFileError(t *testing.T) {
	cfg := &config.Config{IncludeTOC: true}
	logr := logger.NewLogger("error")
	conv, _ := NewPDFConverter(cfg, logr)
	pdfPath := createTempValidPDF(t)
	baseFile := filepath.Join(t.TempDir(), "basefile")
	if err := os.WriteFile(baseFile, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create base file: %v", err)
	}
	_, err := conv.ConvertPDF(pdfPath, baseFile)
	if err == nil {
		t.Fatal("expected error when output base dir is a file")
	}
}

func TestConvertPDF_EndToEnd_WithRealPDF(t *testing.T) {
	pdfPath := createTempValidPDF(t)
	cfg := &config.Config{IncludeTOC: true, BaseHeaderLevel: 1, ExtractImages: true, DetectDiagrams: true, DiagramConfidence: 0.7}
	logr := logger.NewLogger("warn")
	conv, _ := NewPDFConverter(cfg, logr)
	outBase := t.TempDir()
	res, err := conv.ConvertPDF(pdfPath, outBase)
	if err != nil {
		t.Fatalf("ConvertPDF() error = %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result")
	}
	if res.PageCount < 1 {
		t.Errorf("expected at least 1 page, got %d", res.PageCount)
	}
	if !strings.Contains(res.OutputDir, "MARKDOWN_test") {
		t.Errorf("unexpected output dir: %s", res.OutputDir)
	}
	if _, err := os.Stat(res.MarkdownFile); err != nil {
		t.Fatalf("markdown file missing: %v", err)
	}
	content, err := os.ReadFile(res.MarkdownFile)
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}
	md := string(content)
	if !strings.HasPrefix(md, "# PDF Document") {
		t.Error("markdown should start with the document title")
	}
	if cfg.IncludeTOC && !strings.Contains(md, "## Table of Contents") {
		t.Error("expected TOC in markdown content")
	}
}

func TestConvertPDFsInDirectory(t *testing.T) {
	pdfSrc := createTempValidPDF(t)
	inDir := t.TempDir()
	copy1 := filepath.Join(inDir, "one.pdf")
	copy2 := filepath.Join(inDir, "nested", "two.PDF")
	if err := os.MkdirAll(filepath.Dir(copy2), 0755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	if err := copyFile(pdfSrc, copy1); err != nil {
		t.Fatalf("copy1: %v", err)
	}
	if err := copyFile(pdfSrc, copy2); err != nil {
		t.Fatalf("copy2: %v", err)
	}
	cfg := &config.Config{IncludeTOC: true, ExtractImages: true}
	logr := logger.NewLogger("warn")
	conv, _ := NewPDFConverter(cfg, logr)
	outBase := t.TempDir()
	batch, err := conv.ConvertPDFsInDirectory(inDir, outBase)
	if err != nil {
		t.Fatalf("ConvertPDFsInDirectory() error = %v", err)
	}
	if batch.FileCount != 2 {
		t.Errorf("expected FileCount=2, got %d", batch.FileCount)
	}
	if batch.SuccessCount != 2 || batch.FailureCount != 0 {
		t.Errorf("unexpected success/failure counts: %d/%d", batch.SuccessCount, batch.FailureCount)
	}
	if len(batch.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(batch.Results))
	}
	for _, r := range batch.Results {
		if _, err := os.Stat(r.MarkdownFile); err != nil {
			t.Errorf("markdown missing for result: %v", err)
		}
	}
}

func TestConvertPDFsInDirectory_NoPDFs(t *testing.T) {
	cfg := &config.Config{}
	logr := logger.NewLogger("error")
	conv, _ := NewPDFConverter(cfg, logr)
	inDir := t.TempDir()
	outBase := t.TempDir()
	batch, err := conv.ConvertPDFsInDirectory(inDir, outBase)
	if err != nil {
		t.Fatalf("ConvertPDFsInDirectory() error = %v", err)
	}
	if batch.FileCount != 0 || batch.SuccessCount != 0 || batch.FailureCount != 0 {
		t.Errorf("unexpected counts: %+v", batch)
	}
}

// Helpers
func createTempValidPDF(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	pdfPath := filepath.Join(dir, "test.pdf")
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(40, 10, "Hello, PDF")
	if err := pdf.OutputFileAndClose(pdfPath); err != nil {
		t.Fatalf("failed to create temp pdf: %v", err)
	}
	return pdfPath
}

func copyFile(src, dst string) error {
	srcF, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcF.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	dstF, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if runtime.GOOS != "windows" {
			_ = dstF.Sync()
		}
		_ = dstF.Close()
	}()
	_, err = io.Copy(dstF, srcF)
	return err
}

func strconvI(i int) string { return strconvItoa(i) }
func strconvItoa(i int) string {
	if i == 0 {
		return "0"
	}
	sign := ""
	if i < 0 {
		sign = "-"
		i = -i
	}
	var buf [32]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return sign + string(buf[pos:])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
