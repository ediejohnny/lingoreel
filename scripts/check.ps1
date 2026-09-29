# check.ps1 - Quality gates required before every commit: formatting, static
# analysis and tests (with the race detector when a C compiler is available).
#
# Usage:
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts\check.ps1 [-SkipRace]

param(
    [switch]$SkipRace
)

$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "lib\native.ps1")

$root = Split-Path -Parent $PSScriptRoot
# Every test lives under tests\, mirroring the production tree.
$testsRoot = Join-Path $root "tests"
$goSourceDirs = @("cmd", "internal", "tests")

# Scripts need a UTF-8 BOM: without it Windows PowerShell 5.1 reads them as ANSI
# and breaks every accented message.
function Test-PowerShellScripts {
    $problems = @()
    foreach ($script in Get-ChildItem -LiteralPath $PSScriptRoot -Recurse -Filter "*.ps1") {
        $bytes = [System.IO.File]::ReadAllBytes($script.FullName)
        $hasBom = $bytes.Length -ge 3 -and $bytes[0] -eq 0xEF -and $bytes[1] -eq 0xBB -and $bytes[2] -eq 0xBF
        if (-not $hasBom) {
            $problems += "$($script.Name): sem BOM UTF-8"
        }
        $parseErrors = $null
        [System.Management.Automation.Language.Parser]::ParseFile($script.FullName, [ref]$null, [ref]$parseErrors) | Out-Null
        foreach ($parseError in $parseErrors) {
            $problems += "$($script.Name):$($parseError.Extent.StartLineNumber): $($parseError.Message)"
        }
    }
    return $problems
}

Push-Location -LiteralPath $root
$previousCgo = $env:CGO_ENABLED
try {
    Write-Host "[1/5] Validando os scripts PowerShell (codificação e sintaxe)..."
    $scriptProblems = @(Test-PowerShellScripts)
    if ($scriptProblems.Count -gt 0) {
        $scriptProblems | ForEach-Object { Write-Host "  $_" -ForegroundColor Red }
        throw "Há scripts PowerShell com problemas."
    }

    Write-Host "[2/5] Conferindo a organização dos testes (só em tests\)..."
    $packages = Invoke-Native -FilePath "go" -Arguments @("list", "-f", "{{.Dir}}|{{len .TestGoFiles}}|{{len .XTestGoFiles}}", "./...") `
        -FailureMessage "Não foi possível listar os pacotes Go" -Capture
    $misplacedTests = @($packages | ForEach-Object {
            $dir, $internalTests, $externalTests = $_ -split '\|'
            $hasTests = ([int]$internalTests + [int]$externalTests) -gt 0
            $insideTests = $dir -eq $testsRoot -or $dir.StartsWith($testsRoot + [System.IO.Path]::DirectorySeparatorChar)
            if ($hasTests -and -not $insideTests) { $dir }
        })
    if ($misplacedTests.Count -gt 0) {
        $misplacedTests | ForEach-Object { Write-Host "  $($_.Substring($root.Length + 1))" -ForegroundColor Red }
        throw "Há pacotes com arquivos de teste fora da pasta tests\."
    }

    Write-Host "[3/5] Verificando a formatação (gofmt)..."
    $sourceDirs = @($goSourceDirs | Where-Object { Test-Path -LiteralPath (Join-Path $root $_) })
    $unformatted = Invoke-Native -FilePath "gofmt" -Arguments (@("-l") + $sourceDirs) `
        -FailureMessage "O gofmt falhou" -Capture
    if ($unformatted) {
        Write-Host "Arquivos fora do padrão do gofmt:" -ForegroundColor Red
        $unformatted | ForEach-Object { Write-Host "  $_" -ForegroundColor Red }
        throw "Formatação pendente. Rode: gofmt -w $($sourceDirs -join ' ')"
    }

    Write-Host "[4/5] Análise estática (go vet)..."
    Invoke-Native -FilePath "go" -Arguments @("vet", "./...") -FailureMessage "O go vet encontrou problemas"

    $testArguments = @("test", "./...")
    if (-not $SkipRace) {
        if (Get-Command "gcc" -ErrorAction SilentlyContinue) {
            $env:CGO_ENABLED = "1"
            $testArguments = @("test", "-race", "./...")
        }
        else {
            Write-Host "Aviso: gcc não encontrado no PATH; os testes vão rodar sem -race." -ForegroundColor Yellow
        }
    }
    Write-Host "[5/5] Testes (go $($testArguments -join ' '))..."
    Invoke-Native -FilePath "go" -Arguments $testArguments -FailureMessage "Há testes falhando"

    Write-Host ""
    Write-Host "Tudo certo: scripts, organização, formatação, análise estática e testes passaram." -ForegroundColor Green
}
finally {
    $env:CGO_ENABLED = $previousCgo
    Pop-Location
}
