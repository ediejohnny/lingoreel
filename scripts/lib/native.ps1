# native.ps1 - Runs native programs (go, gofmt, git, wails) from the build scripts.
#
# Windows PowerShell 5.1 turns stderr lines of native programs into error records
# when the host redirects stderr; under $ErrorActionPreference = "Stop" that aborts
# the script even when the program succeeded. Success is decided by the exit code.

function Invoke-Native {
    param(
        [Parameter(Mandatory = $true)][string]$FilePath,
        [string[]]$Arguments = @(),
        [Parameter(Mandatory = $true)][string]$FailureMessage,
        # Returns stdout instead of printing it.
        [switch]$Capture
    )

    $previousPreference = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
        if ($Capture) {
            $output = & $FilePath @Arguments
        }
        else {
            & $FilePath @Arguments | Out-Host
        }
        $exitCode = $LASTEXITCODE
    }
    finally {
        $ErrorActionPreference = $previousPreference
    }

    if ($exitCode -ne 0) {
        throw "$FailureMessage (código de saída $exitCode)."
    }
    if ($Capture) {
        return $output
    }
}

# Start-NativeJob starts a native program in the background, its output kept
# in temporary files; Wait-NativeJobs waits for it. Independent steps run side
# by side this way.
function Start-NativeJob {
    param(
        [Parameter(Mandatory = $true)][string]$FilePath,
        [string[]]$Arguments = @(),
        [Parameter(Mandatory = $true)][string]$WorkingDirectory,
        [Parameter(Mandatory = $true)][string]$FailureMessage
    )

    $outFile = [System.IO.Path]::GetTempFileName()
    $errFile = [System.IO.Path]::GetTempFileName()
    $process = Start-Process -FilePath $FilePath -ArgumentList $Arguments -WorkingDirectory $WorkingDirectory `
        -NoNewWindow -PassThru -RedirectStandardOutput $outFile -RedirectStandardError $errFile
    # Reading the handle now keeps the exit code readable once it ends.
    $null = $process.Handle
    return [pscustomobject]@{ Process = $process; OutFile = $outFile; ErrFile = $errFile; FailureMessage = $FailureMessage }
}

# Wait-NativeJobs waits for every job, prints each one's output in turn and
# throws one error naming every job that failed.
function Wait-NativeJobs {
    param([object[]]$Jobs = @())

    $failures = @()
    foreach ($job in $Jobs) {
        $job.Process.WaitForExit()
        foreach ($file in @($job.OutFile, $job.ErrFile)) {
            Get-Content -LiteralPath $file -Encoding UTF8 | Out-Host
            [System.IO.File]::Delete($file)
        }
        if ($job.Process.ExitCode -ne 0) {
            $failures += "$($job.FailureMessage) (código de saída $($job.Process.ExitCode))."
        }
    }
    if ($failures.Count -gt 0) {
        throw ($failures -join " ")
    }
}
