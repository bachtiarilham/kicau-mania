#Requires -Version 7.0
[CmdletBinding()]param([string]$SourceEnv='.env.webcam',[string]$Settings='.env.lan')
$ErrorActionPreference='Stop'
$appRoot=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$binary=Join-Path $appRoot 'bin/lan-config.exe'
if(-not(Test-Path -LiteralPath $binary)){throw 'Run scripts/build-local.ps1 first.'}
Push-Location $appRoot
try { & $binary -source $SourceEnv -settings $Settings; if($LASTEXITCODE -ne 0){throw 'LAN configuration failed.'} } finally {Pop-Location}
