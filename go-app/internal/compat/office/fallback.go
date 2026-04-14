package office

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
