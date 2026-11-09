#Requires -Version 7.0
<#
.SYNOPSIS
Publish a Windows webcam and microphone as a local RTSP test source.
.EXAMPLE
./scripts/start-webcam-test.ps1 -ListDevices
.EXAMPLE
./scripts/start-webcam-test.ps1 -VideoDevice 'Integrated Camera' -AudioDevice 'Microphone (Realtek Audio)'
.EXAMPLE
./scripts/start-webcam-test.ps1 -Synthetic -DurationSeconds 15
#>
[CmdletBinding()]
param(
    [switch] $ListDevices,
    [switch] $InitEnv,
    [switch] $Synthetic,
    [string] $VideoDevice,
    [string] $AudioDevice,
    [string] $FFmpegBin = 'ffmpeg',
    [string] $FFprobeBin = 'ffprobe',
    [string] $MediaMTXBin,
    [ValidateRange(0, 86400)] [int] $DurationSeconds = 0
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$appRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))

function Resolve-Binary([string] $Binary) {
    $command = Get-Command -Name $Binary -CommandType Application -ErrorAction Stop | Select-Object -First 1
    return $command.Source
}

function Start-OwnedProcess([string] $Binary, [string[]] $Arguments) {
    $info = [System.Diagnostics.ProcessStartInfo]::new()
    $info.FileName = $Binary
    $info.WorkingDirectory = $appRoot
    $info.UseShellExecute = $false
    $info.CreateNoWindow = $true
    $info.RedirectStandardInput = $true
    foreach ($argument in $Arguments) { $info.ArgumentList.Add($argument) }
    $child = [System.Diagnostics.Process]::new()
    $child.StartInfo = $info
    try {
        if (-not $child.Start()) { throw "Cannot start $Binary" }
        return $child
    } catch {
        $child.Dispose()
        throw
    }
}

function Stop-OwnedProcess([System.Diagnostics.Process] $Child, [switch] $FFmpeg) {
    if ($null -eq $Child) { return }
    try {
        if (-not $Child.HasExited -and $FFmpeg) {
            try { $Child.StandardInput.WriteLine('q'); $Child.StandardInput.Flush() } catch { }
            $null = $Child.WaitForExit(5000)
        }
        if (-not $Child.HasExited) { $Child.Kill($true); $null = $Child.WaitForExit(5000) }
    } finally { $Child.Dispose() }
}

$ffmpegPath = Resolve-Binary $FFmpegBin
if ($ListDevices) {
    if (-not $IsWindows) { throw 'DirectShow device listing requires Windows.' }
    # FFmpeg intentionally exits nonzero after listing DirectShow devices.
    & $ffmpegPath -hide_banner -list_devices true -f dshow -i dummy
    $global:LASTEXITCODE = 0
    return
}
if (-not $MediaMTXBin) {
    $bundledBinary = Join-Path $appRoot 'bin/mediamtx-v1.15.3/mediamtx.exe'
    $MediaMTXBin = if (Test-Path -LiteralPath $bundledBinary) { $bundledBinary } else { 'mediamtx' }
}
$mediaPath = Resolve-Binary $MediaMTXBin
$ffprobePath = Resolve-Binary $FFprobeBin
if (-not $InitEnv -and -not $Synthetic -and (-not $VideoDevice -or -not $AudioDevice)) {
    throw 'Specify -VideoDevice and -AudioDevice. Use -ListDevices to see their exact names, or -Synthetic for generated media.'
}
if (-not $InitEnv -and -not $Synthetic -and -not $IsWindows) { throw 'Webcam capture uses Windows DirectShow.' }

$envPath = Join-Path $appRoot '.env.webcam'
if (-not (Test-Path -LiteralPath $envPath)) {
    $random = [byte[]]::new(32)
    $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
    try { $rng.GetBytes($random) } finally { $rng.Dispose() }
    $token = [System.BitConverter]::ToString($random).Replace('-', '').ToLowerInvariant()
    $content = [System.IO.File]::ReadAllText((Join-Path $appRoot '.env.webcam.example'))
    $content = $content.Replace('REPLACE_WITH_A_RANDOM_SECRET_AT_LEAST_32_CHARACTERS', $token)
    # Double quotes are invalid in Windows filenames; single quotes avoid dotenv interpolation.
    if ($ffmpegPath.Contains("'") -or $mediaPath.Contains("'") -or $ffprobePath.Contains("'")) { throw 'Binary paths containing single quotes are not supported by the env initializer.' }
    $content = $content.Replace('FFMPEG_BIN=ffmpeg', "FFMPEG_BIN='$($ffmpegPath.Replace('\', '/'))'")
    $content = $content.Replace('FFPROBE_BIN=ffprobe', "FFPROBE_BIN='$($ffprobePath.Replace('\', '/'))'")
    $content = $content.Replace('MEDIAMTX_BIN=./bin/mediamtx-v1.15.3/mediamtx.exe', "MEDIAMTX_BIN='$($mediaPath.Replace('\', '/'))'")
    $file = [System.IO.File]::Open($envPath, [System.IO.FileMode]::CreateNew, [System.IO.FileAccess]::Write)
    try {
        $bytes = [System.Text.UTF8Encoding]::new($false).GetBytes($content)
        $file.Write($bytes, 0, $bytes.Length)
    } finally { $file.Dispose() }
    Write-Host 'Created .env.webcam with a random API token. Existing files are never overwritten.'
}
if ($InitEnv) { Write-Host 'Ready: go run ./cmd/api -env .env.webcam'; return }

$configPath = Join-Path $appRoot 'config/mediamtx-webcam.yml'
$mediaProcess = $null
$ffmpegProcess = $null
try {
    # Fail before starting children if another source/server owns the test port.
    $probe = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 8555)
    try { $probe.Start() } finally { $probe.Stop() }
    $mediaProcess = Start-OwnedProcess $mediaPath @($configPath)
    $ready = $false
    $deadline = [DateTime]::UtcNow.AddSeconds(10)
    while ([DateTime]::UtcNow -lt $deadline) {
        if ($mediaProcess.HasExited) { throw "Test MediaMTX exited with code $($mediaProcess.ExitCode)." }
        $client = [System.Net.Sockets.TcpClient]::new()
        try {
            $task = $client.ConnectAsync('127.0.0.1', 8555)
            if ($task.Wait(200) -and $client.Connected) { $ready = $true; break }
        } catch { } finally { $client.Dispose() }
        Start-Sleep -Milliseconds 100
    }
    if (-not $ready) { throw 'Test MediaMTX did not become ready on 127.0.0.1:8555.' }

    $ffmpegArgs = @('-hide_banner', '-loglevel', 'warning')
    if ($Synthetic) {
        $ffmpegArgs += @('-re', '-f', 'lavfi', '-i', 'testsrc2=size=640x480:rate=25', '-re', '-f', 'lavfi', '-i', 'sine=frequency=440:sample_rate=48000')
        $ffmpegArgs += @('-map', '0:v:0', '-map', '1:a:0')
    } else {
        $ffmpegArgs += @('-f', 'dshow', '-rtbufsize', '128M', '-i', "video=${VideoDevice}:audio=${AudioDevice}", '-map', '0:v:0', '-map', '0:a:0')
    }
    # H264 without B-frames + Opus also supports browser WebRTC playback.
    $ffmpegArgs += @('-vf', 'scale=640:480:force_original_aspect_ratio=decrease,pad=640:480:(ow-iw)/2:(oh-ih)/2', '-r', '25', '-c:v', 'libx264', '-preset', 'ultrafast', '-tune', 'zerolatency', '-pix_fmt', 'yuv420p', '-profile:v', 'baseline', '-bf', '0', '-g', '25', '-b:v', '1000k', '-c:a', 'libopus', '-ar', '48000', '-ac', '1', '-b:a', '64k')
    if ($DurationSeconds -gt 0) { $ffmpegArgs += @('-t', $DurationSeconds.ToString()) }
    $ffmpegArgs += @('-f', 'rtsp', '-rtsp_transport', 'tcp', 'rtsp://127.0.0.1:8555/webcam')
    $ffmpegProcess = Start-OwnedProcess $ffmpegPath $ffmpegArgs
    Write-Host 'Publishing test source: rtsp://127.0.0.1:8555/webcam'
    Write-Host 'In another terminal, from local-app: go run ./cmd/api -env .env.webcam'
    Write-Host 'Press Ctrl+C to stop the test source and its owned processes.'
    while (-not $ffmpegProcess.WaitForExit(250)) {
        if ($mediaProcess.HasExited) { throw "Test MediaMTX stopped with code $($mediaProcess.ExitCode)." }
    }
    if ($ffmpegProcess.ExitCode -ne 0) { throw "FFmpeg stopped with code $($ffmpegProcess.ExitCode). Check device names, permissions, and supported formats." }
} finally {
    try { Stop-OwnedProcess $ffmpegProcess -FFmpeg } finally { Stop-OwnedProcess $mediaProcess }
}
