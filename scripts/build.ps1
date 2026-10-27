# build.ps1 - Compiles the LingoReel executables into build\bin: the
# development tool (lrdev.exe) and the app with its window (LingoReel.exe).
#
# Usage:
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build.ps1 [-Version 1.2.3] [-OutDir folder]
#
# Without -Version, the version comes from the nearest git tag (git describe).
# With -OutDir, both executables go to that folder instead, and build\bin is
# left as it is (an app open from there keeps running).
# The app carries the version in its file properties. Microsoft's WebView2
# installer does not go inside it (it has Microsoft's own terms): a Windows 10
# machine that lacks WebView2 is offered the download from Microsoft. The app
# is pure Go (no cgo), so no C runtime goes inside it either.

param(
    [string]$Version = "",
    [string]$OutDir = ""
)

$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "lib\native.ps1")
. (Join-Path $PSScriptRoot "lib\version.ps1")
. (Join-Path $PSScriptRoot "lib\notices.ps1")

$root = Split-Path -Parent $PSScriptRoot
$wailsBin = Join-Path $root "build\bin"
$binDir = if ($OutDir) { [IO.Path]::GetFullPath((Join-Path $root $OutDir)) } else { $wailsBin }
$wailsConfig = Join-Path $root "wails.json"
# Wails always writes into build\bin: elsewhere, it writes a name of its own
# there and the file moves to the folder asked.
$wailsName = if ($OutDir) { "LingoReel (build).exe" } else { "LingoReel.exe" }

$buildVersion = Resolve-BuildVersion -Root $root -Version $Version
$fileVersion = Get-NumericVersion -BuildVersion $buildVersion
$ldflags = "-s -w -X github.com/ediejohnny/lingoreel/internal/buildinfo.Version=$buildVersion"
New-Item -ItemType Directory -Force -Path $binDir | Out-Null
$previousCgo = $env:CGO_ENABLED
$env:CGO_ENABLED = "0"

Push-Location -LiteralPath $root
try {
    $lrdev = Join-Path $binDir "lrdev.exe"
    Write-Host "[1/5] Building lrdev.exe (version $buildVersion)..."
    Invoke-Native -FilePath "go" -Arguments @("build", "-trimpath", "-ldflags", $ldflags, "-o", $lrdev, "./cmd/lrdev") `
        -FailureMessage "Building lrdev failed"

    Write-Host "[2/5] Checking the version stamped in the executable..."
    $reported = Invoke-Native -FilePath $lrdev -Arguments @("version") -FailureMessage "lrdev.exe did not run" -Capture
    if ("$reported".Trim() -ne $buildVersion) {
        throw "lrdev.exe reports version '$reported', but '$buildVersion' was expected."
    }

    $app = Join-Path $binDir "LingoReel.exe"
    Write-Host "[3/5] Building the app (LingoReel.exe) with its interface..."
    # The file version lives in wails.json: it is set for this build only and
    # the file goes back as it was.
    $originalConfig = [IO.File]::ReadAllBytes($wailsConfig)
    try {
        $config = [IO.File]::ReadAllText($wailsConfig)
        $patched = [regex]::Replace($config, '"productVersion":\s*"[^"]*"', "`"productVersion`": `"$fileVersion`"")
        if ($patched -eq $config -and $config -notmatch "`"productVersion`": `"$fileVersion`"") {
            throw "wails.json has no productVersion field."
        }
        [IO.File]::WriteAllText($wailsConfig, $patched, (New-Object Text.UTF8Encoding($false)))
        Invoke-Native -FilePath "wails" -Arguments @("build", "-trimpath", "-webview2", "download", "-ldflags", $ldflags, "-o", $wailsName) `
            -FailureMessage "Building the app failed"
    }
    finally {
        [IO.File]::WriteAllBytes($wailsConfig, $originalConfig)
    }
    $built = Join-Path $wailsBin $wailsName
    if ($built -ne $app) {
        Move-Item -LiteralPath $built -Destination $app -Force
    }
    if (-not (Test-Path -LiteralPath $app)) {
        throw "The app was not built at $app."
    }

    Write-Host "[4/5] Checking the app's file properties..."
    $productVersion = Get-FileProperty -Path $app -Name "ProductVersion"
    $company = Get-FileProperty -Path $app -Name "CompanyName"
    if ($productVersion -ne $fileVersion -or $company -ne "Edie Johnny") {
        throw "The app came out with version '$productVersion' and author '$company'; expected '$fileVersion' and 'Edie Johnny'."
    }

    Write-Host "[5/5] Checking the notices inside the app name every module it carries..."
    $notices = [IO.File]::ReadAllText((Get-NoticesPath -Root $root))
    $carried = Invoke-Native -FilePath "go" -Arguments @("version", "-m", $app) -FailureMessage "go version -m failed" -Capture
    $missing = @($carried | ForEach-Object {
            $fields = "$_".Trim() -split '\s+'
            if ($fields.Count -ge 3 -and $fields[0] -eq "dep" -and -not $notices.Contains("`n$($fields[1]) $($fields[2])")) {
                "$($fields[1]) $($fields[2])"
            }
        })
    if ($missing.Count -gt 0) {
        throw "The app carries no notice for $($missing -join ', '). Run scripts\update-notices.ps1 and build again."
    }
    $sizeMb = [math]::Round((Get-Item -LiteralPath $lrdev).Length / 1MB, 1)
    Write-Host ""
    Write-Host "Build finished." -ForegroundColor Green
    Write-Host "  $lrdev ($sizeMb MB)"
    $appMb = [math]::Round((Get-Item -LiteralPath $app).Length / 1MB, 1)
    Write-Host "  $app ($appMb MB, version $fileVersion)"
}
finally {
    Pop-Location
    $env:CGO_ENABLED = $previousCgo
}
