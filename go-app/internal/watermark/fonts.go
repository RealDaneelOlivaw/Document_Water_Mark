package watermark

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

var GUIFontChoices = []string{
	"\u5fae\u8f6f\u96c5\u9ed1",
	"\u5b8b\u4f53",
	"\u9ed1\u4f53",
	"\u6977\u4f53",
	"\u4eff\u5b8b",
	"Arial",
	"Calibri",
	"Cambria",
	"Times New Roman",
	"Consolas",
}

var fontNameCandidates = map[string][]string{
	"\u5fae\u8f6f\u96c5\u9ed1": {"msyh.ttc", "msyh.ttf", "simhei.ttf", "arial.ttf"},
	"\u5b8b\u4f53":             {"simsun.ttc", "simsun.ttf", "simhei.ttf"},
	"\u9ed1\u4f53":             {"simhei.ttf", "msyh.ttc", "arial.ttf"},
	"\u6977\u4f53":             {"simkai.ttf", "kaiti.ttf", "simhei.ttf"},
	"\u4eff\u5b8b":             {"simfang.ttf", "fangsong.ttf", "simhei.ttf"},
	"Arial":                    {"arial.ttf", "calibri.ttf"},
	"Calibri":                  {"calibri.ttf", "arial.ttf"},
	"Cambria":                  {"cambria.ttc", "cambria.ttf", "arial.ttf"},
	"Times New Roman":          {"times.ttf", "timesbd.ttf", "arial.ttf"},
	"Consolas":                 {"consola.ttf", "arial.ttf"},
}

var fallbackFontNames = []string{"\u5fae\u8f6f\u96c5\u9ed1", "\u9ed1\u4f53", "Arial"}

func LoadFace(fontName string, sizePx float64) (font.Face, error) {
	candidates := BuildFontCandidates(fontName)
	var failures []string
	for _, candidate := range candidates {
		data, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		parsed, err := opentype.Parse(data)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", candidate, err))
			continue
		}
		face, err := opentype.NewFace(parsed, &opentype.FaceOptions{
			Size:    sizePx,
			DPI:     72,
			Hinting: font.HintingFull,
		})
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", candidate, err))
			continue
		}
		return face, nil
	}
	if len(failures) == 0 {
		return nil, fmt.Errorf("no usable fonts found for %q", fontName)
	}
	return nil, fmt.Errorf("failed to load fonts for %q: %s", fontName, strings.Join(failures, "; "))
}

func BuildFontCandidates(fontName string) []string {
	fontDir := filepath.Join(os.Getenv("WINDIR"), "Fonts")
	if fontDir == "Fonts" {
		fontDir = `C:\Windows\Fonts`
	}

	requests := append([]string{fontName}, fallbackFontNames...)
	seen := map[string]struct{}{}
	var results []string

	for _, request := range requests {
		names, ok := fontNameCandidates[request]
		if !ok {
			names = []string{request}
		}
		for _, name := range names {
			for _, expanded := range expandFontCandidate(fontDir, name) {
				key := strings.ToLower(expanded)
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				results = append(results, expanded)
			}
		}
	}
	return results
}

func expandFontCandidate(fontDir, candidate string) []string {
	if filepath.IsAbs(candidate) {
		return []string{candidate}
	}
	values := []string{candidate, filepath.Join(fontDir, candidate)}
	ext := strings.ToLower(filepath.Ext(candidate))
	if ext != "" {
		return values
	}
	for _, suffix := range []string{".ttf", ".ttc", ".otf"} {
		values = append(values, filepath.Join(fontDir, candidate+suffix))
		values = append(values, filepath.Join(fontDir, strings.ToLower(candidate)+suffix))
	}
	return values
}
