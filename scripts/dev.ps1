# dev.ps1 - Runs the app in development mode (wails dev): the window reloads
# when the interface changes and the Go side is rebuilt when it changes.
#
# Usage:
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts\dev.ps1
#
# Close the app window (or press Ctrl+C here) to stop.

$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "lib\native.ps1")

$root = Split-Path -Parent $PSScriptRoot
$frontend = Join-Path $root "frontend"

Write-Host "[1/3] Checking the tools..."
foreach ($tool in @("go", "wails", "npm")) {
    if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) {
        throw "The tool '$tool' was not found on the PATH."
    }
}

Write-Host "[2/3] Checking the interface dependencies..."
if (-not (Test-Path -LiteralPath (Join-Path $frontend "node_modules"))) {
    Push-Location -LiteralPath $frontend
    try {
        Invoke-Native -FilePath "npm" -Arguments @("install") -FailureMessage "Installing the interface dependencies failed"
    }
    finally {
        Pop-Location
    }
}

Write-Host "[3/3] Opening the app in development mode (close the window to stop)..."
Push-Location -LiteralPath $root
try {
    Invoke-Native -FilePath "wails" -Arguments @("dev") -FailureMessage "Development mode ended with an error"
}
finally {
    Pop-Location
}