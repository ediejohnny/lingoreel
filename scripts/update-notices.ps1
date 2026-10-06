# update-notices.ps1 - Writes the third-party notices the app carries inside
# it (internal\presentation\notices\third-party.txt): the licenses of Go, of
# the Go modules and frontend packages built into the app and of the Silero
# voice model. Run it after adding, removing or updating a dependency; the
# check refuses notices out of date.
#
# Usage:
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts\update-notices.ps1

$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "lib\native.ps1")
. (Join-Path $PSScriptRoot "lib\notices.ps1")

$root = Split-Path -Parent $PSScriptRoot
$path = Get-NoticesPath -Root $root
$text = Get-ThirdPartyNotices -Root $root
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $path) | Out-Null
[IO.File]::WriteAllText($path, $text, (New-Object Text.UTF8Encoding($false)))

$count = ([regex]::Matches($text, "(?m)^={78}$")).Count
$kb = [math]::Round((Get-Item -LiteralPath $path).Length / 1KB)
Write-Host "Third-party notices written: $count licenses, $kb KB." -ForegroundColor Green
Write-Host "  $path"
