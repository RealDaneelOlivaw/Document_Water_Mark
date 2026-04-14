package office

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const normalizeScript = `
$ErrorActionPreference = 'Stop'
$source = $env:PPT_WATERMARK_DOCX_SOURCE
$target = $env:PPT_WATERMARK_DOCX_TARGET
$attempts = @(
    @{ Name = 'Word'; ProgId = 'Word.Application'; SaveMethod = 'SaveAs'; Format = 16 },
    @{ Name = 'WPS'; ProgId = 'Kwps.Application'; SaveMethod = 'SaveAs2'; Format = 16 },
    @{ Name = 'WPS'; ProgId = 'KWPS.Application'; SaveMethod = 'SaveAs2'; Format = 16 }
)
$failures = New-Object System.Collections.Generic.List[string]
foreach ($attempt in $attempts) {
    $app = $null
    $doc = $null
    try {
        $app = New-Object -ComObject $attempt.ProgId
        $app.Visible = $false
        if ($app.PSObject.Properties.Name -contains 'DisplayAlerts') { $app.DisplayAlerts = 0 }
        $doc = $app.Documents.Open($source, $false, $true)
        if ($attempt.SaveMethod -eq 'SaveAs2' -and ($doc.PSObject.Methods.Name -contains 'SaveAs2')) {
            $doc.SaveAs2($target, $attempt.Format)
        } else {
            $doc.SaveAs([ref]$target, [ref]$attempt.Format)
        }
        Write-Output ('OK|' + $attempt.Name + '|' + $attempt.ProgId)
        exit 0
    } catch {
        $failures.Add($attempt.ProgId + ': ' + $_.Exception.Message)
    } finally {
        if ($doc -ne $null) { try { $doc.Close([ref]$false) } catch {} }
        if ($app -ne $null) { try { $app.Quit() } catch {} }
    }
}
Write-Output ('ERROR|' + ($failures -join ' || '))
exit 1
`

const pageMapScript = `
$ErrorActionPreference = 'Stop'
$source = $env:PPT_WATERMARK_DOCX_SOURCE
$markerPrefix = $env:PPT_WATERMARK_MARKER_PREFIX
$attempts = @(
    @{ Name = 'Word'; ProgId = 'Word.Application' },
    @{ Name = 'WPS'; ProgId = 'Kwps.Application' },
    @{ Name = 'WPS'; ProgId = 'KWPS.Application' }
)
$failures = New-Object System.Collections.Generic.List[string]
$wdMainTextStory = 1
$wdStatisticPages = 2
$wdActiveEndAdjustedPageNumber = 1

foreach ($attempt in $attempts) {
    $app = $null
    $doc = $null
    try {
        $app = New-Object -ComObject $attempt.ProgId
        $app.Visible = $false
        if ($app.PSObject.Properties.Name -contains 'DisplayAlerts') { $app.DisplayAlerts = 0 }
        $doc = $app.Documents.Open($source, $false, $true)

        $totalPages = 0
        try { $totalPages = [int]$doc.ComputeStatistics($wdStatisticPages) } catch { $totalPages = 0 }
        Write-Output ('TOTAL|' + $totalPages)

        foreach ($inlineShape in $doc.InlineShapes) {
            try {
                if ($inlineShape.Range.StoryType -ne $wdMainTextStory) { continue }
            } catch { continue }
            $marker = ''
            try { $marker = [string]$inlineShape.AlternativeText } catch { $marker = '' }
            if ([string]::IsNullOrWhiteSpace($marker)) {
                try { $marker = [string]$inlineShape.Title } catch { $marker = '' }
            }
            if ([string]::IsNullOrWhiteSpace($marker) -or -not $marker.StartsWith($markerPrefix)) {
                continue
            }
            $page = 0
            try { $page = [int]$inlineShape.Range.Information($wdActiveEndAdjustedPageNumber) } catch { $page = 0 }
            Write-Output ('PAGE|' + $marker + '|' + $page)
        }

        foreach ($shape in $doc.Shapes) {
            try {
                if ($shape.Anchor.StoryType -ne $wdMainTextStory) { continue }
            } catch { continue }
            $marker = ''
            try { $marker = [string]$shape.AlternativeText } catch { $marker = '' }
            if ([string]::IsNullOrWhiteSpace($marker)) {
                try { $marker = [string]$shape.Title } catch { $marker = '' }
            }
            if ([string]::IsNullOrWhiteSpace($marker) -or -not $marker.StartsWith($markerPrefix)) {
                continue
            }
            $page = 0
            try { $page = [int]$shape.Anchor.Information($wdActiveEndAdjustedPageNumber) } catch { $page = 0 }
            Write-Output ('PAGE|' + $marker + '|' + $page)
        }

        exit 0
    } catch {
        $failures.Add($attempt.ProgId + ': ' + $_.Exception.Message)
    } finally {
        if ($doc -ne $null) { try { $doc.Close([ref]$false) } catch {} }
        if ($app -ne $null) { try { $app.Quit() } catch {} }
    }
}

Write-Output ('ERROR|' + ($failures -join ' || '))
exit 1
`

func NormalizeDOCX(inputPath string) (string, func(), error) {
	if runtime.GOOS != "windows" {
		return "", func() {}, fmt.Errorf("office compatibility fallback is only available on Windows")
	}
	tempDir, err := os.MkdirTemp("", "ppt_watermark_docx_*")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }
	targetPath := filepath.Join(tempDir, filepath.Base(strings.TrimSuffix(inputPath, filepath.Ext(inputPath)))+"_normalized.docx")

	cmd := exec.Command("powershell", "-NoProfile", "-STA", "-ExecutionPolicy", "Bypass", "-Command", normalizeScript)
	cmd.Env = append(os.Environ(),
		"PPT_WATERMARK_DOCX_SOURCE="+inputPath,
		"PPT_WATERMARK_DOCX_TARGET="+targetPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("Word/WPS compatibility fallback failed: %s", strings.TrimSpace(string(output)))
	}
	return targetPath, cleanup, nil
}

func ExtractDOCXBodyImagePages(inputPath, markerPrefix string) (int, map[string]int, error) {
	if runtime.GOOS != "windows" {
		return 0, nil, fmt.Errorf("docx page mapping is only available on Windows")
	}

	cmd := exec.Command("powershell", "-NoProfile", "-STA", "-ExecutionPolicy", "Bypass", "-Command", pageMapScript)
	cmd.Env = append(os.Environ(),
		"PPT_WATERMARK_DOCX_SOURCE="+inputPath,
		"PPT_WATERMARK_MARKER_PREFIX="+markerPrefix,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, nil, fmt.Errorf("Word/WPS page mapping failed: %s", strings.TrimSpace(string(output)))
	}

	totalPages := 0
	pageMap := map[string]int{}
	lines := strings.Split(strings.ReplaceAll(string(output), "\r\n", "\n"), "\n")
	for _, line := range lines {
		text := strings.TrimSpace(line)
		if text == "" {
			continue
		}
		if strings.HasPrefix(text, "TOTAL|") {
			value := strings.TrimPrefix(text, "TOTAL|")
			pages, parseErr := strconv.Atoi(value)
			if parseErr != nil {
				return 0, nil, fmt.Errorf("invalid total pages output: %q", text)
			}
			totalPages = pages
			continue
		}
		if strings.HasPrefix(text, "PAGE|") {
			parts := strings.SplitN(text, "|", 3)
			if len(parts) != 3 {
				return 0, nil, fmt.Errorf("invalid page map output: %q", text)
			}
			page, parseErr := strconv.Atoi(parts[2])
			if parseErr != nil {
				return 0, nil, fmt.Errorf("invalid page number output: %q", text)
			}
			pageMap[parts[1]] = page
		}
	}
	if totalPages <= 0 {
		return 0, nil, fmt.Errorf("unable to determine DOCX total pages from Word/WPS")
	}
	return totalPages, pageMap, nil
}
