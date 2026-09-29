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
    throw "mkvmerge não foi encontrado. Instale o MKVToolNix: https://mkvtoolnix.download/downloads.html"
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

$mkvmerge = Find-Mkvmerge
Write-Host "Usando $mkvmerge"

Write-Host "[1/2] subtitles.mkv (sem compressão)..."
$plain = Join-Path $fixtures "subtitles.mkv"
Invoke-Native -FilePath $mkvmerge -Arguments (@("--quiet", "--no-date", "--output", $plain, "--compression", "-1:none") + (Get-SubtitleArguments)) `
    -FailureMessage "O mkvmerge falhou ao gerar subtitles.mkv"

Write-Host "[2/2] subtitles-zlib.mkv (compressão zlib)..."
$compressed = Join-Path $fixtures "subtitles-zlib.mkv"
Invoke-Native -FilePath $mkvmerge -Arguments (@("--quiet", "--no-date", "--output", $compressed, "--compression", "-1:zlib") + (Get-SubtitleArguments)) `
    -FailureMessage "O mkvmerge falhou ao gerar subtitles-zlib.mkv"

Write-Host ""
Write-Host "Fixtures gerados em $fixtures" -ForegroundColor Green
