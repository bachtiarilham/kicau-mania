#Requires -Version 7.0
# Run during online preparation, never during event startup.
[CmdletBinding()]param()
$ErrorActionPreference='Stop'
$appRoot=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$buildDir=Join-Path $appRoot 'runtime/proxy-build'
$null=New-Item -ItemType Directory -Path $buildDir -Force
$null=New-Item -ItemType Directory -Path (Join-Path $appRoot 'bin') -Force
Push-Location $buildDir
try{
    & go run github.com/caddyserver/xcaddy/cmd/xcaddy@v0.4.5 build v2.11.2 --with github.com/sbrunk/caddy-dns-hostinger@v0.1.3 --output (Join-Path $appRoot 'bin/caddy.exe')
    if($LASTEXITCODE -ne 0){throw 'Caddy build failed.'}
}finally{Pop-Location}
