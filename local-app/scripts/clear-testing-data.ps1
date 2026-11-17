#Requires -Version 7.0
[CmdletBinding()]
param([switch]$DeleteTestData)
$ErrorActionPreference='Stop'
$appRoot=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$dataRoot=[IO.Path]::GetFullPath((Join-Path $appRoot 'data'))
if(!(Test-Path -LiteralPath $dataRoot)){throw 'DATA_DIR lokal tidak ditemukan'}
if((Resolve-Path -LiteralPath $dataRoot).Path -ne $dataRoot){throw 'DATA_DIR harus direktori nyata dalam local-app'}
if((Get-Item -LiteralPath $dataRoot).Attributes -band [IO.FileAttributes]::ReparsePoint){throw 'DATA_DIR tidak boleh reparse point'}
$files=[Collections.Generic.List[IO.FileInfo]]::new()
foreach($name in @('recordings','queue','media-sessions','commands','media-updates')){
 $target=[IO.Path]::GetFullPath((Join-Path $dataRoot $name))
 if(!$target.StartsWith($dataRoot+[IO.Path]::DirectorySeparatorChar,[StringComparison]::OrdinalIgnoreCase)){throw 'Target di luar DATA_DIR'}
 if(Test-Path -LiteralPath $target){
  if((Get-Item -LiteralPath $target).Attributes -band [IO.FileAttributes]::ReparsePoint){throw 'Target tidak boleh reparse point'}
  $entries=@(Get-ChildItem -LiteralPath $target -Recurse -Force)
  foreach($entry in $entries){if($entry.Attributes -band [IO.FileAttributes]::ReparsePoint){throw 'Reparse point di dalam target'}}
  foreach($entry in $entries){if(!$entry.PSIsContainer){$files.Add($entry)}}
 }
}
$cloud=Join-Path $dataRoot 'cloud-media'
if(Test-Path -LiteralPath $cloud){
 if((Get-Item -LiteralPath $cloud).Attributes -band [IO.FileAttributes]::ReparsePoint){throw 'Cloud metadata tidak boleh reparse point'}
 foreach($entry in @(Get-ChildItem -LiteralPath $cloud -File -Force)){
  if($entry.Name -match '^(session-|upload-).+\.json$'){
   if($entry.Attributes -band [IO.FileAttributes]::ReparsePoint){throw 'Metadata tidak boleh reparse point'}
   $files.Add($entry)
  }
 }
}
foreach($entry in $files){if(!$entry.FullName.StartsWith($dataRoot+[IO.Path]::DirectorySeparatorChar,[StringComparison]::OrdinalIgnoreCase)){throw 'Berkas di luar DATA_DIR'}}
$bytes=($files|Measure-Object Length -Sum).Sum
Write-Output "Testing files: $($files.Count), bytes: $bytes"
Write-Output 'setup.json, pairing.bin dan credential tetap dipertahankan; tidak mengakses cloud.'
if(!$DeleteTestData){Write-Output 'Preview saja. Tambahkan -DeleteTestData untuk menghapus daftar berkas ini.';return}
if(Get-NetTCPConnection -LocalPort 8090 -State Listen -ErrorAction SilentlyContinue){throw 'Hentikan engine di port 8090 sebelum cleanup'}
$lockPath=Join-Path $dataRoot 'edge.lock'
$guard=[IO.File]::Open($lockPath,[IO.FileMode]::OpenOrCreate,[IO.FileAccess]::ReadWrite,[IO.FileShare]::None)
try{
 $protected=@{}
 foreach($name in @('setup.json','cloud-media/pairing.bin')){
  $path=Join-Path $dataRoot $name
  if(Test-Path -LiteralPath $path){$protected[$name]=(Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash}
 }
 foreach($entry in $files){Remove-Item -LiteralPath $entry.FullName -Force}
 foreach($name in $protected.Keys){if((Get-FileHash -LiteralPath (Join-Path $dataRoot $name) -Algorithm SHA256).Hash -ne $protected[$name]){throw 'Protected configuration changed'}}
 Write-Output "Removed $($files.Count) testing files. NVR configuration and pairing unchanged."
}finally{$guard.Dispose()}
