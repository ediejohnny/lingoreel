# package.ps1 - Packs the built app into a portable zip in build\dist: the
# executable, the license, the third-party notices and a short read-me. Run
# build.ps1 first.
#
# Usage:
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts\package.ps1 [-Version 1.2.3]

param(
    [string]$Version = ""
)

$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "lib\native.ps1")
. (Join-Path $PSScriptRoot "lib\version.ps1")
. (Join-Path $PSScriptRoot "lib\notices.ps1")

$root = Split-Path -Parent $PSScriptRoot
$app = Join-Path $root "build\bin\LingoReel.exe"
$license = Join-Path $root "LICENSE"
$distDir = Join-Path $root "build\dist"

$buildVersion = Resolve-BuildVersion -Root $root -Version $Version
$fileVersion = Get-NumericVersion -BuildVersion $buildVersion

# The read-me that goes in the zip, for whoever downloads it.
$readMe = @"
LingoReel $fileVersion
==================

Two synced subtitles over the video, to study languages with movies and series.

HOW TO USE
1. Extract this folder wherever you like (for example, Documents\LingoReel).
2. Open "LingoReel.exe". The first time, a quick setup assistant asks which
   language you speak, which one you study, the study mode and the player.
3. Drag a video into the window. Settings > Help > Documentation explains
   everything the app does.

REQUIREMENTS
- Windows 10 or 11, 64-bit.
- A supported video player: VLC or PotPlayer (the app helps you install one).
- Optional and free: FFmpeg, to choose the subtitle by the sound, follow the
  actors' voice a little ahead and show thumbnails on the seek bar.

WHERE YOUR DATA IS KEPT
Everything stays in %APPDATA%\LingoReel, never in the video's folder:
- Settings and profiles: config.json and profiles.json.
- Each video's corrections, sync and passages: the projects folder.
- The seek bar's thumbnails: the thumbnails folder (up to 4 GB; Settings >
  Appearance clears it).
- Logs of the last 14 days: the logs folder.
Nothing is sent over the internet.

TO REMOVE
- If you turned on "Open videos from Windows" (Settings > Advanced), turn it
  off first.
- Delete this folder. To delete your data too, delete %APPDATA%\LingoReel.

LICENSE
MIT. See the LICENSE file. (c) Edie Johnny.
The licenses of the third-party software included are in THIRD-PARTY-NOTICES.txt
and in the app itself, in Settings > Help > About.
"@

# The notices' introduction; the licenses themselves stay as their authors
# wrote them (the app carries the same ones).
$noticesHeader = @"
LingoReel - third-party notices
===============================

LingoReel is distributed under the MIT license (see LICENSE.txt). It includes
third-party software, each under its own license, reproduced below as their
authors ask. None of it belongs to the author of LingoReel.

"@
foreach ($required in @($app, $license)) {
    if (-not (Test-Path -LiteralPath $required)) {
        throw "File not found: $required. Run scripts\build.ps1 first."
    }
}

Write-Host "[1/3] Checking the version of the built app..."
$built = Get-FileProperty -Path $app -Name "ProductVersion"
if ($built -ne $fileVersion) {
    throw "The built app is version '$built', but the package would be '$fileVersion'. Run scripts\build.ps1 again."
}

Write-Host "[2/3] Assembling the package folder..."
$name = "LingoReel $fileVersion"
$staging = Join-Path $distDir $name
if (Test-Path -LiteralPath $staging) {
    Remove-Item -LiteralPath $staging -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $staging | Out-Null
Copy-Item -LiteralPath $app -Destination $staging
Copy-Item -LiteralPath $license -Destination (Join-Path $staging "LICENSE.txt")
if (-not (Test-ThirdPartyNotices -Root $root)) {
    throw "The third-party notices are out of date. Run scripts\update-notices.ps1."
}
$notices = $noticesHeader + "`n" + [IO.File]::ReadAllText((Get-NoticesPath -Root $root))
# Notepad reads UTF-8 with a BOM correctly on every Windows version.
$withBom = New-Object Text.UTF8Encoding($true)
[IO.File]::WriteAllText((Join-Path $staging "THIRD-PARTY-NOTICES.txt"), ($notices -replace "`r?`n", "`r`n"), $withBom)
[IO.File]::WriteAllText((Join-Path $staging "README.txt"), ($readMe -replace "`r?`n", "`r`n"), $withBom)

Write-Host "[3/3] Compressing..."
$zip = Join-Path $distDir "$name.zip"
if (Test-Path -LiteralPath $zip) {
    Remove-Item -LiteralPath $zip -Force
}
Compress-Archive -Path (Join-Path $staging "*") -DestinationPath $zip
Remove-Item -LiteralPath $staging -Recurse -Force

$zipMb = [math]::Round((Get-Item -LiteralPath $zip).Length / 1MB, 1)
Write-Host ""
Write-Host "Package ready." -ForegroundColor Green
Write-Host "  $zip ($zipMb MB)"
