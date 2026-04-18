package processor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"pptwatermark/goapp/internal/config"
	"pptwatermark/goapp/internal/openxml/common"
	"pptwatermark/goapp/internal/openxml/docx"
	"pptwatermark/goapp/internal/openxml/pptx"
	"pptwatermark/goapp/internal/picture"
)

type Result struct {
	InputPath  string
	OutputPath string
	Success    bool
	Stats      common.ProcessStats
	Err        error
	Warning    string
}

func ProcessFile(inputPath string, cfg config.WatermarkConfig) Result {
	result := Result{InputPath: inputPath}
	outputPath := common.UniqueOutputPath(inputPath)
	result.OutputPath = outputPath

	ext := strings.ToLower(filepath.Ext(inputPath))
	switch ext {
	case ".pptx":
		stats, err := pptx.New().ProcessFile(inputPath, outputPath, cfg)
		result.Stats = stats
		result.Err = err
	case ".docx":
		stats, err := docx.New().ProcessFile(inputPath, outputPath, cfg)
		result.Stats = stats
		result.Err = err
	default:
		if picture.IsSupportedExtension(ext) {
			stats, err := picture.New().ProcessFile(inputPath, outputPath, cfg)
			result.Stats = stats
			result.Err = err
		} else {
			result.Err = fmt.Errorf("unsupported file type: %s", filepath.Ext(inputPath))
		}
	}
	result.Success = result.Err == nil
	applyDeleteOriginalPolicy(&result, cfg, nil)
	return result
}

func applyDeleteOriginalPolicy(result *Result, cfg config.WatermarkConfig, removeFile func(string) error) {
	if result == nil || !cfg.DeleteOriginalEnabled || !result.Success {
		return
	}

	inputPath := filepath.Clean(result.InputPath)
	outputPath := filepath.Clean(result.OutputPath)
	if strings.EqualFold(inputPath, outputPath) {
		return
	}

	if removeFile == nil {
		removeFile = os.Remove
	}
	if err := removeFile(result.InputPath); err != nil {
		result.Warning = fmt.Sprintf("删除原文件失败: %v", err)
	}
}
