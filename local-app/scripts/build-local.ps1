#Requires -Version 7.0
[CmdletBinding()]param()
$ErrorActionPreference='Stop'
$appRoot=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
Push-Location $appRoot
try {
    foreach ($entry in @('api','lan-config','event-package')) {
        $name=if($entry -eq 'api'){'local-server'}else{$entry}
        & go build -o "bin/$name.exe" "./cmd/$entry"
        if($LASTEXITCODE -ne 0){throw "Build failed: $entry"}
    }
    Write-Host 'Binaries prepared. Startup will not compile or download dependencies.'
} finally { Pop-Location }
