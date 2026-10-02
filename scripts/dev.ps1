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

Write-Host "[1/3] Conferindo as ferramentas..."
foreach ($tool in @("go", "wails", "npm")) {
    if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) {
        throw "A ferramenta '$tool' não foi encontrada no PATH."
    }
}

Write-Host "[2/3] Conferindo as dependências da interface..."
if (-not (Test-Path -LiteralPath (Join-Path $frontend "node_modules"))) {
    Push-Location -LiteralPath $frontend
    try {
        Invoke-Native -FilePath "npm" -Arguments @("install") -FailureMessage "A instalação das dependências da interface falhou"
    }
    finally {
        Pop-Location
    }
}

Write-Host "[3/3] Abrindo o app em modo de desenvolvimento (feche a janela para encerrar)..."
Push-Location -LiteralPath $root
try {
    Invoke-Native -FilePath "wails" -Arguments @("dev") -FailureMessage "O modo de desenvolvimento terminou com erro"
}
finally {
    Pop-Location
}