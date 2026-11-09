#Requires -Version 7.0
[CmdletBinding()]param([string]$Settings='.env.lan')
$ErrorActionPreference='Stop'
$appRoot=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$configBinary=Join-Path $appRoot 'bin/lan-config.exe'
$appBinary=Join-Path $appRoot 'bin/local-server.exe'
foreach($binary in @($configBinary,$appBinary)){if(-not(Test-Path -LiteralPath $binary)){throw 'Run scripts/build-local.ps1 before starting the media engine.'}}
function Launch([string]$Binary,[string[]]$Arguments){
    $info=[Diagnostics.ProcessStartInfo]::new();$info.FileName=$Binary;$info.WorkingDirectory=$appRoot
    $info.UseShellExecute=$false;$info.CreateNoWindow=$true
    foreach($argument in $Arguments){$info.ArgumentList.Add($argument)}
    $child=[Diagnostics.Process]::new();$child.StartInfo=$info
    try{$null=$child.Start();return $child}catch{$child.Dispose();throw}
}
function Stop-Owned([Diagnostics.Process]$Child){if($null -eq $Child){return};try{if(-not $Child.HasExited){$Child.Kill($true);$null=$Child.WaitForExit(5000)}}finally{$Child.Dispose()}}
$app=$null;$proxy=$null
Push-Location $appRoot
try{
    $paths=(& $configBinary -settings $Settings -paths | ConvertFrom-Json)
    if($LASTEXITCODE -ne 0){throw 'Configuration render failed.'}
    $app=Launch $appBinary @('-env',$paths.app_env)
    $proxy=Launch $configBinary @('-settings',$Settings,'-serve-proxy')
    Write-Host 'Media engine started. Readiness and cloud authorization are available through the API.'
    Write-Host 'Provision a valid certificate and local DNS before admitting viewers. Ctrl+C stops owned processes.'
    $proxyReported=$false
    while(-not $app.WaitForExit(250)){
        if($proxy.HasExited -and -not $proxyReported){Write-Warning 'HTTPS proxy exited. Check CADDY_BIN, certificate provisioning, and provider credentials. Arena HTTP remains running.';$proxyReported=$true}
    }
    if($app.ExitCode -ne 0){throw "Arena server exited with code $($app.ExitCode)."}
}finally{try{Stop-Owned $proxy}finally{Stop-Owned $app;Pop-Location}}
