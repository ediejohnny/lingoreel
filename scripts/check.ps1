# check.ps1 - Quality gates required before every commit: formatting, the
# third-party notices the app carries, static analysis and tests (with the
# race detector when a C compiler is available),
# and, side by side with them, the window's frontend: types, lint and tests.
#
# By default the Go tests run with -short, which skips the slow ones (windows
# that wait for the screen, real waits, huge inputs); -Full runs every test
# and is required before every merge and tag.
#
# Usage:
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts\check.ps1 [-Full] [-SkipRace]

param(
    [switch]$Full,
    [switch]$SkipRace
)

$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "lib\native.ps1")
. (Join-Path $PSScriptRoot "lib\notices.ps1")

$root = Split-Path -Parent $PSScriptRoot
# Every test lives under tests\, mirroring the production tree.
$testsRoot = Join-Path $root "tests"
$goSourceDirs = @("main.go", "cmd", "internal", "tests")
# Our Go packages; ./... would also take a Go package npm puts in
# frontend\node_modules.
$goPackages = @(".", "./cmd/...", "./internal/...", "./tests/...")
$frontendDir = Join-Path $root "frontend"
# staticcheck finds what go vet does not, such as code nothing calls.
$staticcheckVersion = "v0.8.1"

function Find-Staticcheck {
    $command = Get-Command "staticcheck" -ErrorAction SilentlyContinue
    if ($command) {
        return $command.Source
    }
    $goPath = Invoke-Native -FilePath "go" -Arguments @("env", "GOPATH") -FailureMessage "Could not read GOPATH" -Capture
    $candidate = Join-Path ($goPath | Select-Object -First 1) "bin\staticcheck.exe"
    if (Test-Path -LiteralPath $candidate) {
        return $candidate
    }
    throw "staticcheck was not found. Install it: go install honnef.co/go/tools/cmd/staticcheck@$staticcheckVersion"
}

# Scripts need a UTF-8 BOM: without it Windows PowerShell 5.1 reads them as ANSI
# and breaks every accented message.
function Test-PowerShellScripts {
    $problems = @()
    foreach ($script in Get-ChildItem -LiteralPath $PSScriptRoot -Recurse -Filter "*.ps1") {
        $bytes = [System.IO.File]::ReadAllBytes($script.FullName)
        $hasBom = $bytes.Length -ge 3 -and $bytes[0] -eq 0xEF -and $bytes[1] -eq 0xBB -and $bytes[2] -eq 0xBF
        if (-not $hasBom) {
            $problems += "$($script.Name): no UTF-8 BOM"
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
$previousGogc = $env:GOGC
# The collector runs a quarter as often in the compiler, the linker and the
# test binaries: more memory for less time (measured: about 7% off a cold run).
$env:GOGC = "400"
$previousTestFFmpeg = $env:LINGOREEL_TEST_FFMPEG
# The FFmpeg tests use one kept for them outside git (the app ships none);
# without it, they are skipped.
$localFFmpeg = Join-Path $root ".docs\ffmpeg_project\lgpl-shared\bin\ffmpeg.exe"
if (-not $env:LINGOREEL_TEST_FFMPEG -and (Test-Path -LiteralPath $localFFmpeg)) {
    $env:LINGOREEL_TEST_FFMPEG = $localFFmpeg
}
try {
    Write-Host "[1/7] Validating the PowerShell scripts (encoding and syntax)..."
    $scriptProblems = @(Test-PowerShellScripts)
    if ($scriptProblems.Count -gt 0) {
        $scriptProblems | ForEach-Object { Write-Host "  $_" -ForegroundColor Red }
        throw "Some PowerShell scripts have problems."
    }

    Write-Host "[2/7] Checking where the tests live (only under tests\)..."
    $packages = Invoke-Native -FilePath "go" -Arguments (@("list", "-f", "{{.Dir}}|{{len .TestGoFiles}}|{{len .XTestGoFiles}}") + $goPackages) `
        -FailureMessage "Could not list the Go packages" -Capture
    $misplacedTests = @($packages | ForEach-Object {
            $dir, $internalTests, $externalTests = $_ -split '\|'
            $hasTests = ([int]$internalTests + [int]$externalTests) -gt 0
            $insideTests = $dir -eq $testsRoot -or $dir.StartsWith($testsRoot + [System.IO.Path]::DirectorySeparatorChar)
            if ($hasTests -and -not $insideTests) { $dir }
        })
    if ($misplacedTests.Count -gt 0) {
        $misplacedTests | ForEach-Object { Write-Host "  $($_.Substring($root.Length + 1))" -ForegroundColor Red }
        throw "Some packages have test files outside the tests\ folder."
    }

    Write-Host "[3/7] Checking the formatting (gofmt)..."
    $sourceDirs = @($goSourceDirs | Where-Object { Test-Path -LiteralPath (Join-Path $root $_) })
    $unformatted = Invoke-Native -FilePath "gofmt" -Arguments (@("-l") + $sourceDirs) `
        -FailureMessage "gofmt failed" -Capture
    if ($unformatted) {
        Write-Host "Files not formatted by gofmt:" -ForegroundColor Red
        $unformatted | ForEach-Object { Write-Host "  $_" -ForegroundColor Red }
        throw "Formatting pending. Run: gofmt -w $($sourceDirs -join ' ')"
    }

    # The frontend's checks need nothing from Go: they run side by side, in
    # the background, while Go is analysed and tested.
    Write-Host "[4/7] Interface (types, lint, formatting and tests), alongside Go..."
    if (-not (Test-Path -LiteralPath (Join-Path $frontendDir "node_modules"))) {
        Push-Location -LiteralPath $frontendDir
        try {
            Invoke-Native -FilePath "npm.cmd" -Arguments @("ci") -FailureMessage "Installing the interface dependencies failed"
        }
        finally {
            Pop-Location
        }
    }
    $interfaceJobs = @(
        Start-NativeJob -FilePath "npm.cmd" -Arguments @("run", "typecheck") -WorkingDirectory $frontendDir -FailureMessage "The interface has type errors"
        Start-NativeJob -FilePath "npm.cmd" -Arguments @("run", "lint") -WorkingDirectory $frontendDir -FailureMessage "The interface lint found problems"
        Start-NativeJob -FilePath "npm.cmd" -Arguments @("run", "format:check") -WorkingDirectory $frontendDir -FailureMessage "The interface is not formatted: run npm run format in the frontend folder"
        Start-NativeJob -FilePath "npm.cmd" -Arguments @("test") -WorkingDirectory $frontendDir -FailureMessage "Some interface tests are failing"
    )

    # The licenses the app carries must name what goes inside it today.
    Write-Host "[5/7] Checking the third-party notices the app carries..."
    $noticesFailure = $null
    try {
        if (-not (Test-ThirdPartyNotices -Root $root)) {
            $noticesFailure = "The third-party notices are out of date: run scripts\update-notices.ps1."
        }
    }
    catch {
        $noticesFailure = $_.Exception.Message
    }
    # Go's failures are kept until the frontend is done, so both are told.
    $goFailure = $null
    try {
        Write-Host "[6/7] Static analysis (go vet and staticcheck)..."
        Invoke-Native -FilePath "go" -Arguments (@("vet") + $goPackages) -FailureMessage "go vet found problems"
        Invoke-Native -FilePath (Find-Staticcheck) -Arguments $goPackages -FailureMessage "staticcheck found problems"

        # Packages run side by side (-p) and, inside each, the tests that call
        # t.Parallel (-parallel), one worker per processor but one, which stays
        # free: 7 on an 8-processor computer.
        $workers = [string][Math]::Max(1, [Environment]::ProcessorCount - 1)
        $testFlags = @("-p", $workers, "-parallel", $workers)
        if (-not $Full) {
            $testFlags += "-short"
        }
        if (-not $SkipRace) {
            if (Get-Command "gcc" -ErrorAction SilentlyContinue) {
                $env:CGO_ENABLED = "1"
                $testFlags = @("-race") + $testFlags
            }
            else {
                Write-Host "Warning: gcc not found on the PATH; the tests will run without -race." -ForegroundColor Yellow
            }
        }
        $testArguments = @("test") + $testFlags + $goPackages
        $mode = if ($Full) { "full" } else { "quick: the slow tests run with -Full" }
        Write-Host "[7/7] Tests, $mode (go $($testArguments -join ' '))..."
        Invoke-Native -FilePath "go" -Arguments $testArguments -FailureMessage "Some tests are failing"
    }
    catch {
        $goFailure = $_.Exception.Message
    }

    Write-Host "Interface result:"
    $interfaceFailure = $null
    try {
        Wait-NativeJobs -Jobs $interfaceJobs
    }
    catch {
        $interfaceFailure = $_.Exception.Message
    }
    $failures = @($noticesFailure, $goFailure, $interfaceFailure) | Where-Object { $_ }
    if ($failures.Count -gt 0) {
        throw ($failures -join " ")
    }

    Write-Host ""
    Write-Host "All good: scripts, layout, formatting, notices, static analysis, tests and interface passed." -ForegroundColor Green
}
finally {
    $env:CGO_ENABLED = $previousCgo
    $env:GOGC = $previousGogc
    $env:LINGOREEL_TEST_FFMPEG = $previousTestFFmpeg
    Pop-Location
}
