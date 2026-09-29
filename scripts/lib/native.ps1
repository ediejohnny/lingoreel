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
