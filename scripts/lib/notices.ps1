# notices.ps1 - Gathers the third-party notices built into the app: the
# license of Go itself, of every Go module compiled into it, of the Silero
# voice model it embeds and of the frontend packages bundled into its screens.
# Permissive licenses (MIT, BSD, ISC, Apache-2.0) ask for their notice to go
# with every copy of the software, and the app ships as a single file, so the
# notices go inside it: the About card of its settings' Help page shows them.

# License files: LICENSE, LICENCE, COPYING and NOTICE, with any extension.
$script:noticeFilePattern = '^(LICEN[CS]E|COPYING|NOTICE)([.-].*)?$'

# Where the notices live in the source, embedded into the app when it builds.
$script:noticesFile = "internal\presentation\notices\third-party.txt"

# The build tags Wails compiles the app with (a desktop build, in production,
# that offers to download WebView2): the modules they bring in are the ones
# that go inside the app.
$script:appBuildTags = "desktop,wv2runtime.download,production"

# ConvertTo-ModuleFolder turns a module path into its folder in the module cache,
# where an upper-case letter is written as "!" and the letter in lower case.
function ConvertTo-ModuleFolder {
    param([Parameter(Mandatory = $true)][string]$Path)
    $escaped = New-Object Text.StringBuilder
    foreach ($char in $Path.ToCharArray()) {
        if ([char]::IsUpper($char)) {
            [void]$escaped.Append('!').Append([char]::ToLowerInvariant($char))
        }
        else {
            [void]$escaped.Append($char)
        }
    }
    return $escaped.ToString()
}

# Get-LicenseFiles lists a folder's license files, its own and those of the
# code it carries inside (for example a vendored package), leaving out tests
# and templates, which do not ship.
function Get-LicenseFiles {
    param([Parameter(Mandatory = $true)][string]$Folder)
    Get-ChildItem -LiteralPath $Folder -Recurse -File |
        Where-Object { $_.Name -match $script:noticeFilePattern -and $_.FullName -notmatch '\\(testdata|templates|examples|node_modules)\\' } |
        Sort-Object FullName
}

# Get-AppModules lists the Go modules compiled into the app, as "path version",
# from the packages Go builds for it. The app itself carries the notices, so
# they cannot be read from the built executable.
function Get-AppModules {
    param([Parameter(Mandatory = $true)][string]$Root)
    $template = "{{with .Module}}{{if not .Main}}{{.Path}} {{.Version}}{{if .Replace}} replaced{{end}}{{end}}{{end}}"
    $previousCgo = $env:CGO_ENABLED
    $env:CGO_ENABLED = "0"
    Push-Location -LiteralPath $Root
    try {
        $listed = Invoke-Native -FilePath "go" -Arguments @("list", "-deps", "-tags", $script:appBuildTags, "-f", $template, ".") `
            -FailureMessage "go list failed" -Capture
    }
    finally {
        Pop-Location
        $env:CGO_ENABLED = $previousCgo
    }
    $modules = New-Object Collections.Generic.SortedSet[string] ([StringComparer]::Ordinal)
    foreach ($line in $listed) {
        $entry = "$line".Trim()
        if (-not $entry) {
            continue
        }
        # A replaced module ships the code of another: its license is not
        # the one in the module cache folder read below.
        if ($entry.EndsWith(" replaced")) {
            throw "The module $($entry.Substring(0, $entry.Length - 9)) is replaced; its notice needs the replacement's license."
        }
        [void]$modules.Add($entry)
    }
    return $modules
}

# Get-ThirdPartyNotices returns the notices of the app at $Root: each license
# as its authors wrote it, under the name and version of what it covers, with
# Unix line ends. Nothing in it depends on where the code lies on this computer.
function Get-ThirdPartyNotices {
    param([Parameter(Mandatory = $true)][string]$Root)

    $sections = New-Object Collections.Generic.List[string]
    $add = {
        param([string]$Name, [string]$Version, [string]$File, [string]$Within)
        $text = ([IO.File]::ReadAllText($File) -replace "`r`n?", "`n").Trim()
        $where = if ($Within) { " ($Within)" } else { "" }
        $sections.Add(("=" * 78) + "`n$Name $Version$where`n" + ("-" * 78) + "`n$text`n")
    }

    # Go itself: the runtime and the standard library are inside every Go program.
    $goRoot = (Invoke-Native -FilePath "go" -Arguments @("env", "GOROOT") -FailureMessage "go env failed" -Capture | Out-String).Trim()
    $goVersion = (Invoke-Native -FilePath "go" -Arguments @("env", "GOVERSION") -FailureMessage "go env failed" -Capture | Out-String).Trim()
    & $add "Go" $goVersion (Join-Path $goRoot "LICENSE") ""

    # The modules compiled into the app.
    $moduleCache = (Invoke-Native -FilePath "go" -Arguments @("env", "GOMODCACHE") -FailureMessage "go env failed" -Capture | Out-String).Trim()
    foreach ($entry in (Get-AppModules -Root $Root)) {
        $module, $version = $entry -split ' ', 2
        $folder = Join-Path $moduleCache ((ConvertTo-ModuleFolder $module) + "@" + $version)
        $files = @(if (Test-Path -LiteralPath $folder) { Get-LicenseFiles -Folder $folder })
        if ($files.Count -eq 0) {
            throw "No license file for $module $version in $folder."
        }
        foreach ($file in $files) {
            $inner = Split-Path -Parent $file.FullName.Substring($folder.Length).TrimStart('\')
            & $add $module $version $file.FullName ($inner -replace '\\', '/')
        }
    }

    # The Silero VAD model, embedded in the executable as its authors published
    # it: not a Go module, so named here.
    & $add "Silero VAD (silero_vad_16k_op15.onnx)" "v6.2.1" (Join-Path $Root "internal\infrastructure\silero\model\LICENSE") ""

    # The frontend packages bundled into the screens: those the lock file does
    # not mark as development only, and Tailwind, whose CSS is generated into
    # the bundle.
    $frontend = Join-Path $Root "frontend"
    # The lock file names its root package "", which ConvertFrom-Json refuses
    # in Windows PowerShell 5.1; .NET's reader takes it.
    Add-Type -AssemblyName System.Web.Extensions
    $reader = New-Object Web.Script.Serialization.JavaScriptSerializer
    $reader.MaxJsonLength = [int]::MaxValue
    $lock = $reader.DeserializeObject([IO.File]::ReadAllText((Join-Path $frontend "package-lock.json")))
    $shipped = New-Object Collections.Generic.SortedSet[string] ([StringComparer]::Ordinal)
    foreach ($entry in $lock["packages"].GetEnumerator()) {
        if ($entry.Key -like "node_modules/*" -and -not $entry.Value["dev"]) {
            [void]$shipped.Add($entry.Key.Substring("node_modules/".Length))
        }
    }
    [void]$shipped.Add("tailwindcss")
    foreach ($package in $shipped) {
        $folder = Join-Path $frontend ("node_modules\" + ($package -replace '/', '\'))
        $manifest = Get-Content -Raw -LiteralPath (Join-Path $folder "package.json") | ConvertFrom-Json
        $files = @(Get-ChildItem -LiteralPath $folder -File | Where-Object { $_.Name -match $script:noticeFilePattern } | Sort-Object Name)
        if ($files.Count -eq 0) {
            throw "No license file for the frontend package $package."
        }
        foreach ($file in $files) {
            & $add $package $manifest.version $file.FullName ""
        }
    }

    return ($sections -join "`n")
}

# Get-NoticesPath is where the app's notices live in the source at $Root.
function Get-NoticesPath {
    param([Parameter(Mandatory = $true)][string]$Root)
    return Join-Path $Root $script:noticesFile
}

# Test-ThirdPartyNotices tells whether the notices in the source at $Root are
# those the code builds into the app today.
function Test-ThirdPartyNotices {
    param([Parameter(Mandatory = $true)][string]$Root)
    $path = Get-NoticesPath -Root $Root
    if (-not (Test-Path -LiteralPath $path)) {
        return $false
    }
    return [IO.File]::ReadAllText($path) -ceq (Get-ThirdPartyNotices -Root $Root)
}
