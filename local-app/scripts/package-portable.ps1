#Requires -Version 7.0
[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$FFmpegPath,
    [Parameter(Mandatory)][string]$FFprobePath,
    [Parameter(Mandatory)][string]$MediaMTXPath,
    [Parameter(Mandatory)][string]$FFmpegLicenseDirectory,
    [string]$OutputDirectory
)
$ErrorActionPreference = 'Stop'
$appRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
if (!$OutputDirectory) { $OutputDirectory = Join-Path $appRoot ("bin/portable-" + (Get-Date -Format 'yyyyMMdd-HHmmss')) }
$outputPath = [IO.Path]::GetFullPath($OutputDirectory)
foreach ($file in @($FFmpegPath,$FFprobePath,$MediaMTXPath)) { if (!(Test-Path -LiteralPath $file -PathType Leaf)) { throw "Missing binary: $file" } }
if (!(Test-Path -LiteralPath $FFmpegLicenseDirectory -PathType Container)) { throw 'Provide the FFmpeg build license/source-notice directory.' }
$ffmpegVersion = (& $FFmpegPath -version 2>&1 | Out-String)
if ($LASTEXITCODE -ne 0 -or $ffmpegVersion -match '--enable-shared') { throw 'Provide a working static FFmpeg build for the portable package.' }
$mediaVersion = (& $MediaMTXPath --version 2>&1 | Out-String)
if ($LASTEXITCODE -ne 0 -or $mediaVersion -notmatch 'v?1\.15\.3\b') { throw 'This configuration requires MediaMTX v1.15.3.' }
if (Test-Path -LiteralPath $outputPath) { throw 'Choose a new output directory; existing packages and data are not overwritten.' }
New-Item -ItemType Directory -Path (Join-Path $outputPath 'bin'),(Join-Path $outputPath 'licenses/ffmpeg'),(Join-Path $outputPath 'docs') -Force | Out-Null
Push-Location $appRoot
try {
    $previousOS=$env:GOOS; $previousArch=$env:GOARCH
    try {
        $env:GOOS='windows'; $env:GOARCH='amd64'
        & go build -trimpath -o (Join-Path $outputPath 'local-server.exe') ./cmd/api
        if ($LASTEXITCODE -ne 0) { throw 'Windows build failed' }
    } finally { $env:GOOS=$previousOS; $env:GOARCH=$previousArch }
    Copy-Item -LiteralPath $FFmpegPath -Destination (Join-Path $outputPath 'bin/ffmpeg.exe')
    Copy-Item -LiteralPath $FFprobePath -Destination (Join-Path $outputPath 'bin/ffprobe.exe')
    Copy-Item -LiteralPath $MediaMTXPath -Destination (Join-Path $outputPath 'bin/mediamtx.exe')
    Copy-Item -Path (Join-Path $FFmpegLicenseDirectory '*') -Destination (Join-Path $outputPath 'licenses/ffmpeg') -Recurse
    Copy-Item -LiteralPath (Join-Path $appRoot '.env.portable.example') -Destination (Join-Path $outputPath '.env')
    Copy-Item -LiteralPath (Join-Path $appRoot 'docs/setup-api.md') -Destination (Join-Path $outputPath 'docs/setup-api.md')
    Copy-Item -LiteralPath (Join-Path $appRoot 'third-party/mediamtx-LICENSE') -Destination (Join-Path $outputPath 'licenses/mediamtx-LICENSE')
    [IO.File]::WriteAllText((Join-Path $outputPath 'Start-local-app.cmd'),"@echo off`r`ncd /d `"%~dp0`"`r`nlocal-server.exe -env .env`r`nif errorlevel 1 pause`r`n",[Text.Encoding]::ASCII)
    $sums = Get-ChildItem -LiteralPath $outputPath -File -Recurse | Get-FileHash -Algorithm SHA256 | ForEach-Object { "{0}  {1}" -f $_.Hash,([IO.Path]::GetRelativePath($outputPath,$_.Path)) }
    $sums | Set-Content -LiteralPath (Join-Path $outputPath 'SHA256SUMS.txt') -Encoding utf8
    Write-Host "Portable API prepared: $outputPath"
    Write-Host 'No UI included. Open docs/setup-api.md to connect your frontend or API client.'
} finally { Pop-Location }
