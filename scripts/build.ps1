# build.ps1 - Compiles the LingoReel executables into build\bin: the
# development tool (spdev.exe) and the app with its window (LingoReel.exe).
#
# Usage:
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build.ps1 [-Version 1.2.3]
#
# Without -Version, the version comes from the nearest git tag (git describe).
# The app carries the version in its file properties, and the WebView2
# installer goes inside it, for Windows 10 machines that lack WebView2.

param(
    [string]$Version = ""
)

$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "lib\native.ps1")
. (Join-Path $PSScriptRoot "lib\version.ps1")

$root = Split-Path -Parent $PSScriptRoot
$binDir = Join-Path $root "build\bin"
$wailsConfig = Join-Path $root "wails.json"

$buildVersion = Resolve-BuildVersion -Root $root -Version $Version
$fileVersion = Get-NumericVersion -BuildVersion $buildVersion
$ldflags = "-s -w -X lingoreel/internal/buildinfo.Version=$buildVersion"
New-Item -ItemType Directory -Force -Path $binDir | Out-Null

Push-Location -LiteralPath $root
try {
    $spdev = Join-Path $binDir "spdev.exe"
    Write-Host "[1/4] Compilando spdev.exe (versão $buildVersion)..."
    Invoke-Native -FilePath "go" -Arguments @("build", "-trimpath", "-ldflags", $ldflags, "-o", $spdev, "./cmd/spdev") `
        -FailureMessage "A compilação do spdev falhou"

    Write-Host "[2/4] Conferindo a versão gravada no executável..."
    $reported = Invoke-Native -FilePath $spdev -Arguments @("version") -FailureMessage "O spdev.exe não executou" -Capture
    if ("$reported".Trim() -ne $buildVersion) {
        throw "O spdev.exe informa a versão '$reported', mas o esperado era '$buildVersion'."
    }

    $app = Join-Path $binDir "LingoReel.exe"
    Write-Host "[3/4] Compilando o app (LingoReel.exe) com a interface..."
    # The file version lives in wails.json: it is set for this build only and
    # the file goes back as it was.
    $originalConfig = [IO.File]::ReadAllBytes($wailsConfig)
    try {
        $config = [IO.File]::ReadAllText($wailsConfig)
        $patched = [regex]::Replace($config, '"productVersion":\s*"[^"]*"', "`"productVersion`": `"$fileVersion`"")
        if ($patched -eq $config -and $config -notmatch "`"productVersion`": `"$fileVersion`"") {
            throw "O wails.json não tem o campo productVersion."
        }
        [IO.File]::WriteAllText($wailsConfig, $patched, (New-Object Text.UTF8Encoding($false)))
        Invoke-Native -FilePath "wails" -Arguments @("build", "-trimpath", "-webview2", "embed", "-ldflags", $ldflags) `
            -FailureMessage "A compilação do app falhou"
    }
    finally {
        [IO.File]::WriteAllBytes($wailsConfig, $originalConfig)
    }
    if (-not (Test-Path -LiteralPath $app)) {
        throw "O app não foi gerado em $app."
    }

    Write-Host "[4/4] Conferindo as propriedades do app..."
    $productVersion = Get-FileProperty -Path $app -Name "ProductVersion"
    $company = Get-FileProperty -Path $app -Name "CompanyName"
    if ($productVersion -ne $fileVersion -or $company -ne "Edie Johnny") {
        throw "O app saiu com a versão '$productVersion' e o autor '$company'; o esperado era '$fileVersion' e 'Edie Johnny'."
    }

    $sizeMb = [math]::Round((Get-Item -LiteralPath $spdev).Length / 1MB, 1)
    Write-Host ""
    Write-Host "Compilação concluída." -ForegroundColor Green
    Write-Host "  $spdev ($sizeMb MB)"
    $appMb = [math]::Round((Get-Item -LiteralPath $app).Length / 1MB, 1)
    Write-Host "  $app ($appMb MB, versão $fileVersion)"
}
finally {
    Pop-Location
}
