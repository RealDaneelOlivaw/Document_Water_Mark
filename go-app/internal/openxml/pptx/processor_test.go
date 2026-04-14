package pptx

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadSlideFilesInOrderUsesPresentationOrder(t *testing.T) {
	tempDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tempDir, "ppt", "slides"), 0o755); err != nil {
		t.Fatalf("mkdir slides failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tempDir, "ppt", "_rels"), 0o755); err != nil {
		t.Fatalf("mkdir rels failed: %v", err)
	}

	slideXML := `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"/>`
	for _, name := range []string{"slide1.xml", "slide2.xml", "slide10.xml"} {
		if err := os.WriteFile(filepath.Join(tempDir, "ppt", "slides", name), []byte(slideXML), 0o644); err != nil {
			t.Fatalf("write %s failed: %v", name, err)
		}
	}

	presentationXML := `<?xml version="1.0" encoding="UTF-8"?>
<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:sldIdLst>
    <p:sldId id="256" r:id="rId10"/>
    <p:sldId id="257" r:id="rId2"/>
    <p:sldId id="258" r:id="rId1"/>
  </p:sldIdLst>
</p:presentation>`
	if err := os.WriteFile(filepath.Join(tempDir, "ppt", "presentation.xml"), []byte(presentationXML), 0o644); err != nil {
		t.Fatalf("write presentation.xml failed: %v", err)
	}

	presentationRels := `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide2.xml"/>
  <Relationship Id="rId10" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide10.xml"/>
</Relationships>`
	if err := os.WriteFile(filepath.Join(tempDir, "ppt", "_rels", "presentation.xml.rels"), []byte(presentationRels), 0o644); err != nil {
		t.Fatalf("write presentation.xml.rels failed: %v", err)
	}

	ordered, err := readSlideFilesInOrder(tempDir)
	if err != nil {
		t.Fatalf("readSlideFilesInOrder failed: %v", err)
	}

	names := make([]string, 0, len(ordered))
	for _, filePath := range ordered {
		names = append(names, filepath.Base(filePath))
	}

	expected := []string{"slide10.xml", "slide2.xml", "slide1.xml"}
	if !reflect.DeepEqual(names, expected) {
		t.Fatalf("expected %v, got %v", expected, names)
	}
}

func TestReadSlideFilesFallbackUsesNumericSort(t *testing.T) {
	tempDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tempDir, "ppt", "slides"), 0o755); err != nil {
		t.Fatalf("mkdir slides failed: %v", err)
	}
	slideXML := `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"/>`
	for _, name := range []string{"slide2.xml", "slide10.xml", "slide1.xml"} {
		if err := os.WriteFile(filepath.Join(tempDir, "ppt", "slides", name), []byte(slideXML), 0o644); err != nil {
			t.Fatalf("write %s failed: %v", name, err)
		}
	}

	ordered := readSlideFilesFallback(tempDir)
	names := make([]string, 0, len(ordered))
	for _, filePath := range ordered {
		names = append(names, filepath.Base(filePath))
	}

	expected := []string{"slide1.xml", "slide2.xml", "slide10.xml"}
	if !reflect.DeepEqual(names, expected) {
		t.Fatalf("expected %v, got %v", expected, names)
	}
}
