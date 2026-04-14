$ErrorActionPreference = "Stop"

$go = "C:\Program Files\Go\bin\go.exe"
$root = $PSScriptRoot
$dist = Join-Path $root "..\dist"
$exe = Join-Path $dist "watermark-gui.exe"
$manifest = Join-Path $root "cmd\watermark-gui\watermark-gui.manifest"
$icon = Join-Path $root "assets\icon\waterdrop-logo.ico"
$syso = Join-Path $root "cmd\watermark-gui\rsrc.syso"

if (-not (Test-Path $go)) {
    throw "Go toolchain not found at $go"
}

New-Item -ItemType Directory -Path $dist -Force | Out-Null

$upxCandidates = @()
$projectUpxRoot = Join-Path $root "..\tools\upx"
if (Test-Path $projectUpxRoot) {
    $upxCandidates += Get-ChildItem $projectUpxRoot -Recurse -Filter upx.exe -ErrorAction SilentlyContinue | Select-Object -ExpandProperty FullName
}
$upxCommand = Get-Command upx -ErrorAction SilentlyContinue
if ($upxCommand) {
    $upxCandidates += $upxCommand.Source
}
$upxCandidates += Join-Path $env:LOCALAPPDATA "Microsoft\WinGet\Links\upx.exe"
$upxCandidates += Get-ChildItem (Join-Path $env:LOCALAPPDATA "Microsoft\WinGet\Packages") -Recurse -Filter upx.exe -ErrorAction SilentlyContinue | Select-Object -ExpandProperty FullName
$upxCandidates = @($upxCandidates | Where-Object { $_ -and (Test-Path $_) } | Select-Object -Unique)

$rsrcCandidates = @()
$rsrcCommand = Get-Command rsrc -ErrorAction SilentlyContinue
if ($rsrcCommand) {
    $rsrcCandidates += $rsrcCommand.Source
}
$goPath = & $go env GOPATH
if ($goPath) {
    $rsrcCandidates += Join-Path $goPath "bin\rsrc.exe"
}
$rsrcCandidates += Join-Path $env:USERPROFILE "go\bin\rsrc.exe"
$rsrcCandidates = @($rsrcCandidates | Where-Object { $_ -and (Test-Path $_) } | Select-Object -Unique)

Push-Location $root
try {
    if (-not (Test-Path $manifest)) {
        throw "Manifest file not found: $manifest"
    }
    if (-not (Test-Path $icon)) {
        throw "Icon file not found: $icon"
    }
    if ($rsrcCandidates.Count -eq 0) {
        throw "rsrc tool not found. Install it with: go install github.com/akavel/rsrc@latest"
    }
    & $rsrcCandidates[0] -manifest $manifest -ico $icon -o $syso

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
