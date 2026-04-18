package processor

import (
	"errors"
	"strings"
	"testing"

	"pptwatermark/goapp/internal/config"
)

func TestApplyDeleteOriginalPolicyDisabled(t *testing.T) {
	result := &Result{
		InputPath:  `C:\docs\a.jpg`,
		OutputPath: `C:\docs\a_watermarked.jpg`,
		Success:    true,
	}
	called := false

	applyDeleteOriginalPolicy(result, config.WatermarkConfig{DeleteOriginalEnabled: false}, func(_ string) error {
		called = true
		return nil
	})

	if called {
		t.Fatal("expected delete callback not to be called when feature is disabled")
	}
	if result.Warning != "" {
		t.Fatalf("expected empty warning, got %q", result.Warning)
	}
}

func TestApplyDeleteOriginalPolicyDeleteSuccess(t *testing.T) {
	result := &Result{
		InputPath:  `C:\docs\a.jpg`,
		OutputPath: `C:\docs\a_watermarked.jpg`,
		Success:    true,
	}
	deletedPath := ""

	applyDeleteOriginalPolicy(result, config.WatermarkConfig{DeleteOriginalEnabled: true}, func(path string) error {
		deletedPath = path
		return nil
	})

	if deletedPath != result.InputPath {
		t.Fatalf("expected delete callback to receive %q, got %q", result.InputPath, deletedPath)
	}
	if result.Warning != "" {
		t.Fatalf("expected empty warning, got %q", result.Warning)
	}
	if !result.Success {
		t.Fatal("result success should remain true")
	}
}

func TestApplyDeleteOriginalPolicyDeleteFailureKeepsSuccessAndWarns(t *testing.T) {
	result := &Result{
		InputPath:  `C:\docs\a.docx`,
		OutputPath: `C:\docs\a_watermarked.docx`,
		Success:    true,
	}

	applyDeleteOriginalPolicy(result, config.WatermarkConfig{DeleteOriginalEnabled: true}, func(_ string) error {
		return errors.New("access denied")
	})

	if !result.Success {
		t.Fatal("result success should remain true when delete fails")
	}
	if !strings.Contains(result.Warning, "删除原文件失败") {
		t.Fatalf("expected warning to include delete failure message, got %q", result.Warning)
	}
}

func TestApplyDeleteOriginalPolicySkipsWhenNotSuccess(t *testing.T) {
	result := &Result{
		InputPath:  `C:\docs\a.pptx`,
		OutputPath: `C:\docs\a_watermarked.pptx`,
		Success:    false,
	}
	called := false

	applyDeleteOriginalPolicy(result, config.WatermarkConfig{DeleteOriginalEnabled: true}, func(_ string) error {
		called = true
		return nil
	})

	if called {
		t.Fatal("expected delete callback not to be called when processing failed")
	}
	if result.Warning != "" {
		t.Fatalf("expected empty warning, got %q", result.Warning)
	}
}

func TestApplyDeleteOriginalPolicySkipsWhenInputEqualsOutput(t *testing.T) {
	result := &Result{
		InputPath:  `C:\DOCS\A.JPG`,
		OutputPath: `c:\docs\a.jpg`,
		Success:    true,
	}
	called := false

	applyDeleteOriginalPolicy(result, config.WatermarkConfig{DeleteOriginalEnabled: true}, func(_ string) error {
		called = true
		return nil
	})

	if called {
		t.Fatal("expected delete callback not to be called when input and output are equal")
	}
	if result.Warning != "" {
		t.Fatalf("expected empty warning, got %q", result.Warning)
	}
}
