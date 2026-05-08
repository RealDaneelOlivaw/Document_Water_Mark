package processor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"pptwatermark/goapp/internal/compat/office"
	"pptwatermark/goapp/internal/config"
	"pptwatermark/goapp/internal/openxml/common"
	"pptwatermark/goapp/internal/openxml/docx"
	"pptwatermark/goapp/internal/openxml/pptx"
	"pptwatermark/goapp/internal/openxml/xlsx"
	"pptwatermark/goapp/internal/pdf"
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
	case ".ppt":
		stats, err := processLegacyPPT(inputPath, outputPath, cfg)
		result.Stats = stats
		result.Err = err
	case ".docx":
		stats, err := docx.New().ProcessFile(inputPath, outputPath, cfg)
		result.Stats = stats
		result.Err = err
	case ".doc":
		stats, err := processLegacyDOC(inputPath, outputPath, cfg)
		result.Stats = stats
		result.Err = err
	case ".xlsx":
		if cfg.ExcludePagesEnabled {
			appendWarning(&result, "Excel 文件没有稳定页码语义，已忽略排除页码设置")
		}
		stats, err := xlsx.New().ProcessFile(inputPath, outputPath, cfgWithoutExcludePages(cfg))
		result.Stats = stats
		result.Err = err
	case ".xls":
		if cfg.ExcludePagesEnabled {
			appendWarning(&result, "Excel 文件没有稳定页码语义，已忽略排除页码设置")
		}
		stats, err := processLegacyXLS(inputPath, outputPath, cfgWithoutExcludePages(cfg))
		result.Stats = stats
		result.Err = err
	case ".pdf":
		stats, err := pdf.New().ProcessFile(inputPath, outputPath, cfg)
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

func processLegacyPPT(inputPath, outputPath string, cfg config.WatermarkConfig) (common.ProcessStats, error) {
	convertedPath, cleanup, err := office.ConvertPPTToPPTX(inputPath)
	if err != nil {
		return common.ProcessStats{}, err
	}
	defer cleanup()

	tempDir, err := os.MkdirTemp("", "ppt_watermark_ppt_out_")
	if err != nil {
		return common.ProcessStats{}, err
	}
	defer os.RemoveAll(tempDir)

	intermediatePath := filepath.Join(tempDir, "watermarked.pptx")
	stats, err := pptx.New().ProcessFile(convertedPath, intermediatePath, cfg)
	if err != nil {
		return stats, err
	}
	if err := office.ConvertPPTXToPPT(intermediatePath, outputPath); err != nil {
		return stats, err
	}
	return stats, nil
}

func processLegacyDOC(inputPath, outputPath string, cfg config.WatermarkConfig) (common.ProcessStats, error) {
	convertedPath, cleanup, err := office.ConvertDOCToDOCX(inputPath)
	if err != nil {
		return common.ProcessStats{}, err
	}
	defer cleanup()

	tempDir, err := os.MkdirTemp("", "ppt_watermark_doc_out_")
	if err != nil {
		return common.ProcessStats{}, err
	}
	defer os.RemoveAll(tempDir)

	intermediatePath := filepath.Join(tempDir, "watermarked.docx")
	stats, err := docx.New().ProcessFile(convertedPath, intermediatePath, cfg)
	if err != nil {
		return stats, err
	}
	if err := office.ConvertDOCXToDOC(intermediatePath, outputPath); err != nil {
		return stats, err
	}
	return stats, nil
}

func processLegacyXLS(inputPath, outputPath string, cfg config.WatermarkConfig) (common.ProcessStats, error) {
	convertedPath, cleanup, err := office.ConvertXLSToXLSX(inputPath)
	if err != nil {
		return common.ProcessStats{}, err
	}
	defer cleanup()

	tempDir, err := os.MkdirTemp("", "ppt_watermark_xls_out_")
	if err != nil {
		return common.ProcessStats{}, err
	}
	defer os.RemoveAll(tempDir)

	intermediatePath := filepath.Join(tempDir, "watermarked.xlsx")
	stats, err := xlsx.New().ProcessFile(convertedPath, intermediatePath, cfg)
	if err != nil {
		return stats, err
	}
	if err := office.ConvertXLSXToXLS(intermediatePath, outputPath); err != nil {
		return stats, err
	}
	return stats, nil
}

func cfgWithoutExcludePages(cfg config.WatermarkConfig) config.WatermarkConfig {
	cfg.ExcludePagesEnabled = false
	cfg.ExcludePages = nil
	return cfg
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
		appendWarning(result, fmt.Sprintf("删除原文件失败: %v", err))
	}
}

func appendWarning(result *Result, warning string) {
	if result == nil || strings.TrimSpace(warning) == "" {
		return
	}
	if strings.TrimSpace(result.Warning) == "" {
		result.Warning = warning
		return
	}
	result.Warning += "; " + warning
}
