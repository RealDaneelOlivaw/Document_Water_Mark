package config

import (
	"reflect"
	"testing"
)

func TestDefaultConfigMatchesPlan(t *testing.T) {
	cfg := Default()
	if cfg.FontSizePt != 12 {
		t.Fatalf("expected default font size 12, got %d", cfg.FontSizePt)
	}
	if cfg.Opacity != 6 {
		t.Fatalf("expected default opacity 6, got %d", cfg.Opacity)
	}
	if cfg.GapXRatio != 0.2 {
		t.Fatalf("expected default gap x ratio 0.2, got %.2f", cfg.GapXRatio)
	}
	if cfg.GapYRatio != 1.5 {
		t.Fatalf("expected default gap y ratio 1.5, got %.2f", cfg.GapYRatio)
	}
	if cfg.MinAreaPct != 3 {
		t.Fatalf("expected default min area pct 3, got %d", cfg.MinAreaPct)
	}
	if cfg.DeleteOriginalEnabled {
		t.Fatal("expected delete original default to false")
	}
}

func TestValidateRejectsEmptyText(t *testing.T) {
	cfg := Default()
	cfg.Text = "   "
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for empty text")
	}
}

func TestParseExcludePagesSupportsValuesAndRanges(t *testing.T) {
	pages, err := ParseExcludePages("1, 3-5\uFF0C8")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	expected := []int{1, 3, 4, 5, 8}
	if !reflect.DeepEqual(pages, expected) {
		t.Fatalf("expected %v, got %v", expected, pages)
	}
}

func TestParseExcludePagesDedupAndSort(t *testing.T) {
	pages, err := ParseExcludePages("5,2,2,4-5")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	expected := []int{2, 4, 5}
	if !reflect.DeepEqual(pages, expected) {
		t.Fatalf("expected %v, got %v", expected, pages)
	}
}

func TestParseExcludePagesRejectsInvalidInput(t *testing.T) {
	cases := []string{
		"",
		"0",
		"-1",
		"3-1",
		"a",
		"1,,2",
	}
	for _, input := range cases {
		if _, err := ParseExcludePages(input); err == nil {
			t.Fatalf("expected parse error for %q", input)
		}
	}
}

func TestValidateExcludePagesAgainstTotal(t *testing.T) {
	if err := ValidateExcludePagesAgainstTotal([]int{1, 3, 5}, 5); err != nil {
		t.Fatalf("expected valid pages, got error: %v", err)
	}
	if err := ValidateExcludePagesAgainstTotal([]int{6}, 5); err == nil {
		t.Fatal("expected error when exclude page exceeds total")
	}
}

func TestValidateRequiresExcludePagesWhenEnabled(t *testing.T) {
	cfg := Default()
	cfg.ExcludePagesEnabled = true
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error when exclude pages is enabled but empty")
	}

	cfg.ExcludePages = []int{2, 1}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for unsorted exclude pages")
	}

	cfg.ExcludePages = []int{1, 2}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected validation success, got %v", err)
	}
}
