# make-fixtures.ps1 - Regenerates the Matroska test fixtures with mkvmerge.
#
# The fixtures are committed; run this only when the sources in
# tests\testdata\matroska\source change. Requires MKVToolNix
# (https://mkvtoolnix.download/downloads.html).
#
# Usage:
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts\make-fixtures.ps1

$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "lib\native.ps1")

$root = Split-Path -Parent $PSScriptRoot
$fixtures = Join-Path $root "tests\testdata\matroska"
$sources = Join-Path $fixtures "source"

function Find-Mkvmerge {
    $command = Get-Command "mkvmerge" -ErrorAction SilentlyContinue
    if ($command) {
        return $command.Source
    }
    foreach ($base in @($env:ProgramFiles, ${env:ProgramFiles(x86)})) {
        if (-not $base) {
            continue
        }
        $candidate = Join-Path $base "MKVToolNix\mkvmerge.exe"
        if (Test-Path -LiteralPath $candidate) {
            return $candidate
        }
    }
    throw "mkvmerge was not found. Install MKVToolNix: https://mkvtoolnix.download/downloads.html"
}

# Subtitle tracks shared by every fixture: English (default), Brazilian
# Portuguese ASS (not default) and a forced Spanish line.
function Get-SubtitleArguments {
    return @(
        "--language", "0:en", "--track-name", "0:English", "--default-track-flag", "0:yes",
        (Join-Path $sources "english.srt"),
        "--language", "0:pt-BR", "--track-name", "0:Português (Brasil)", "--default-track-flag", "0:no",
        (Join-Path $sources "portuguese.ass"),
        "--language", "0:es", "--forced-display-flag", "0:yes", "--default-track-flag", "0:no",
        (Join-Path $sources "spanish-forced.srt")
    )
}

function Find-Vlc {
    $command = Get-Command "vlc" -ErrorAction SilentlyContinue
    if ($command) {
        return $command.Source
    }
    foreach ($base in @($env:ProgramFiles, ${env:ProgramFiles(x86)})) {
        if ($base) {
            $candidate = Join-Path $base "VideoLAN\VLC\vlc.exe"
            if (Test-Path -LiteralPath $candidate) {
                return $candidate
            }
        }
    }
    return $null
}

# Writes a mono 16-bit PCM WAV with a quiet 440 Hz tone.
function New-ToneWav([string]$Path, [int]$Seconds) {
    $rate = 48000
    $samples = $rate * $Seconds
    $writer = New-Object System.IO.BinaryWriter([System.IO.File]::Create($Path))
    try {
        $writer.Write([Text.Encoding]::ASCII.GetBytes("RIFF")); $writer.Write([int](36 + $samples * 2))
        $writer.Write([Text.Encoding]::ASCII.GetBytes("WAVEfmt ")); $writer.Write([int]16)
        $writer.Write([int16]1); $writer.Write([int16]1); $writer.Write([int]$rate); $writer.Write([int]($rate * 2))
        $writer.Write([int16]2); $writer.Write([int16]16)
        $writer.Write([Text.Encoding]::ASCII.GetBytes("data")); $writer.Write([int]($samples * 2))
        $buffer = New-Object byte[] ($samples * 2)
        for ($i = 0; $i -lt $samples; $i++) {
            $value = [int16](1500 * [Math]::Sin(2 * [Math]::PI * 440 * $i / $rate))
            $buffer[2 * $i] = [byte]($value -band 0xFF)
            $buffer[2 * $i + 1] = [byte](($value -shr 8) -band 0xFF)
        }
        $writer.Write($buffer)
    }
    finally {
        $writer.Close()
    }
}

# Builds a 20-second study video: a still image and a tone encoded by VLC,
# then muxed by mkvmerge with the English and Portuguese subtitles.
function New-StudySample([string]$Vlc, [string]$Mkvmerge, [string]$Destination) {
    $seconds = 20
    $work = Join-Path ([System.IO.Path]::GetTempPath()) ("lingoreel-fixture-" + [guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory -Force -Path $work | Out-Null
    try {
        Add-Type -AssemblyName System.Drawing
        $image = New-Object System.Drawing.Bitmap 320, 180
        $graphics = [System.Drawing.Graphics]::FromImage($image)
        $graphics.Clear([System.Drawing.Color]::FromArgb(30, 60, 90))
        $font = New-Object System.Drawing.Font("Arial", 16)
        $graphics.DrawString("LingoReel", $font, [System.Drawing.Brushes]::White, 70, 75)
        $graphics.Dispose()
        $png = Join-Path $work "frame.png"
        $image.Save($png, [System.Drawing.Imaging.ImageFormat]::Png)
        $image.Dispose()

        $wav = Join-Path $work "tone.wav"
        New-ToneWav -Path $wav -Seconds $seconds

        $encoded = Join-Path $work "encoded.mkv"
        # Stereo on purpose: VLC's AAC encoder writes doubled timestamps for mono.
        $sout = "#transcode{vcodec=h264,vb=120,fps=24,width=320,height=180,acodec=mp4a,ab=64,channels=2,samplerate=48000}:std{access=file,mux=mkv,dst=$encoded}"
        $vlcArguments = @("-I", "dummy", "--dummy-quiet", "`"$png`"", "--image-duration=$seconds", "--image-fps=24/1",
            "--input-slave=`"$wav`"", "--sout", "`"$sout`"", "vlc://quit")
        $process = Start-Process -FilePath $Vlc -ArgumentList $vlcArguments -PassThru -WindowStyle Hidden
        if (-not $process.WaitForExit(180000) -or -not (Test-Path -LiteralPath $encoded)) {
            throw "VLC could not generate the sample video."
        }

        # Track IDs follow VLC's muxing order, so they are read instead of assumed.
        $identification = Invoke-Native -FilePath $Mkvmerge -Arguments @("-J", $encoded) `
            -FailureMessage "mkvmerge could not identify the generated video" -Capture
        $tracks = (($identification -join "`n") | ConvertFrom-Json).tracks
        $videoId = ($tracks | Where-Object { $_.type -eq "video" } | Select-Object -First 1).id
        $audioId = ($tracks | Where-Object { $_.type -eq "audio" } | Select-Object -First 1).id
        if ($null -eq $videoId -or $null -eq $audioId) {
            throw "The video generated by VLC lacks the expected video and audio tracks."
        }

        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Destination) | Out-Null
        Invoke-Native -FilePath $Mkvmerge -Arguments (@("--quiet", "--no-date", "--output", $Destination,
                "--language", "$($videoId):und", "--default-duration", "$($videoId):24fps",
                "--language", "$($audioId):en", "--track-name", "$($audioId):English",
                "--track-order", "0:$videoId,0:$audioId", $encoded) + (Get-SubtitleArguments)) `
            -FailureMessage "mkvmerge failed to generate study-sample.mkv"
    }
    finally {
        Remove-Item -LiteralPath $work -Recurse -Force -ErrorAction SilentlyContinue
    }
}

$mkvmerge = Find-Mkvmerge
Write-Host "Using $mkvmerge"

Write-Host "[1/4] subtitles.mkv (no compression)..."
$plain = Join-Path $fixtures "subtitles.mkv"
Invoke-Native -FilePath $mkvmerge -Arguments (@("--quiet", "--no-date", "--output", $plain, "--compression", "-1:none") + (Get-SubtitleArguments)) `
    -FailureMessage "mkvmerge failed to generate subtitles.mkv"

Write-Host "[2/4] subtitles-zlib.mkv (zlib compression)..."
$compressed = Join-Path $fixtures "subtitles-zlib.mkv"
Invoke-Native -FilePath $mkvmerge -Arguments (@("--quiet", "--no-date", "--output", $compressed, "--compression", "-1:zlib") + (Get-SubtitleArguments)) `
    -FailureMessage "mkvmerge failed to generate subtitles-zlib.mkv"

Write-Host "[3/4] study-sample.mkv (synthetic video with audio and subtitles)..."
$studySample = Join-Path $root "tests\testdata\video\study-sample.mkv"
$vlc = Find-Vlc
if (-not $vlc) {
    Write-Host "Warning: VLC not found; study-sample.mkv was not regenerated." -ForegroundColor Yellow
}
else {
    New-StudySample -Vlc $vlc -Mkvmerge $mkvmerge -Destination $studySample
}

# Positions past 2^31 microseconds (35 min 47 s) overflow 32-bit integers;
# stretching the sample's timestamps 120 times gives a 40-minute file of a
# few kilobytes to test them.
Write-Host "[4/4] long-sample.mkv (40 minutes, video only)..."
Invoke-Native -FilePath $mkvmerge -Arguments @("--quiet", "--no-date", "--output", (Join-Path $root "tests\testdata\video\long-sample.mkv"),
    "--no-audio", "--no-subtitles", "--sync", "-1:0,120", $studySample) `
    -FailureMessage "mkvmerge failed to generate long-sample.mkv"

Write-Host ""
Write-Host "Fixtures generated in $fixtures" -ForegroundColor Green
