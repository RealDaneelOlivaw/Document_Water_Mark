package processor

import (
	"fmt"
	"path/filepath"
	"strings"

	"pptwatermark/goapp/internal/config"
	"pptwatermark/goapp/internal/openxml/common"
	"pptwatermark/goapp/internal/openxml/docx"
	"pptwatermark/goapp/internal/openxml/pptx"
)

type Result struct {
	InputPath  string
	OutputPath string
	Success    bool
	Stats      common.ProcessStats
	Err        error
}

func ProcessFile(inputPath string, cfg config.WatermarkConfig) Result {
	result := Result{InputPath: inputPath}
	outputPath := common.UniqueOutputPath(inputPath)
	result.OutputPath = outputPath

	switch strings.ToLower(filepath.Ext(inputPath)) {
	case ".pptx":
		stats, err := pptx.New().ProcessFile(inputPath, outputPath, cfg)
		result.Stats = stats
		result.Err = err
	case ".docx":
		stats, err := docx.New().ProcessFile(inputPath, outputPath, cfg)
		result.Stats = stats
		result.Err = err
	default:
		result.Err = fmt.Errorf("unsupported file type: %s", filepath.Ext(inputPath))
	}
	result.Success = result.Err == nil
	return result
}
