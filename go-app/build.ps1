$ErrorActionPreference = "Stop"

$go = "C:\Program Files\Go\bin\go.exe"
$root = $PSScriptRoot
$dist = Join-Path $root "..\dist"
$exe = Join-Path $dist "watermark-gui.exe"

if (-not (Test-Path $go)) {
    throw "Go toolchain not found at $go"
}

New-Item -ItemType Directory -Path $dist -Force | Out-Null

$upxCandidates = @()
$upxCommand = Get-Command upx -ErrorAction SilentlyContinue
if ($upxCommand) {
    $upxCandidates += $upxCommand.Source
}
$upxCandidates += Join-Path $env:LOCALAPPDATA "Microsoft\WinGet\Links\upx.exe"
$upxCandidates += Get-ChildItem (Join-Path $env:LOCALAPPDATA "Microsoft\WinGet\Packages") -Recurse -Filter upx.exe -ErrorAction SilentlyContinue | Select-Object -ExpandProperty FullName
$upxCandidates = $upxCandidates | Where-Object { $_ -and (Test-Path $_) } | Select-Object -Unique

Push-Location $root
try {
    & $go mod tidy
    & $go build -mod=vendor -trimpath -ldflags "-s -w -H=windowsgui" -o $exe .\cmd\watermark-gui
    if ($upxCandidates.Count -gt 0) {
        & $upxCandidates[0] --best --lzma $exe
    }
    Write-Host "Built: $exe"
    Get-Item $exe | Select-Object FullName, Length
} finally {
    Pop-Location
}
