# build.ps1 - Compiles the LingoReel executables into build\bin.
#
# Usage:
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build.ps1 [-Version 1.2.3]
#
# Without -Version, the version comes from the nearest git tag (git describe).

param(
    [string]$Version = ""
)

$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "lib\native.ps1")

$root = Split-Path -Parent $PSScriptRoot
$binDir = Join-Path $root "build\bin"
$fallbackVersion = "0.0.0-dev"
$semverPattern = '^\d+\.\d+\.\d+'

# Turns "v0.3.0-2-gabc1234-dirty" into "0.3.0-2-gabc1234-dirty"; untagged
# history becomes "0.0.0-dev+<commit>".
function Resolve-BuildVersion {
    if ($Version) {
        return $Version.TrimStart("v")
    }
    try {
        $described = Invoke-Native -FilePath "git" -Arguments @("-C", $root, "describe", "--tags", "--always", "--dirty") `
            -FailureMessage "git describe falhou" -Capture
    }
    catch {
        return $fallbackVersion
    }
    $described = "$described".Trim().TrimStart("v")
    if ($described -match $semverPattern) {
        return $described
    }
    return "$fallbackVersion+$described"
}

$buildVersion = Resolve-BuildVersion
$ldflags = "-s -w -X lingoreel/internal/buildinfo.Version=$buildVersion"
New-Item -ItemType Directory -Force -Path $binDir | Out-Null

Push-Location -LiteralPath $root
try {
    $spdev = Join-Path $binDir "spdev.exe"
    Write-Host "[1/2] Compilando spdev.exe (versão $buildVersion)..."
    Invoke-Native -FilePath "go" -Arguments @("build", "-trimpath", "-ldflags", $ldflags, "-o", $spdev, "./cmd/spdev") `
        -FailureMessage "A compilação do spdev falhou"

    Write-Host "[2/2] Conferindo a versão gravada no executável..."
    $reported = Invoke-Native -FilePath $spdev -Arguments @("version") -FailureMessage "O spdev.exe não executou" -Capture
    if ("$reported".Trim() -ne $buildVersion) {
        throw "O spdev.exe informa a versão '$reported', mas o esperado era '$buildVersion'."
    }

    $sizeMb = [math]::Round((Get-Item -LiteralPath $spdev).Length / 1MB, 1)
    Write-Host ""
    Write-Host "Compilação concluída." -ForegroundColor Green
    Write-Host "  $spdev ($sizeMb MB)"
}
finally {
    Pop-Location
}
