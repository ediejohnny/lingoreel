# notices.ps1 - Writes the third-party notices that go in the zip: the license
# of every Go module compiled into the app (read from the executable itself,
# so the list always matches what ships), of Go itself, and of the frontend
# packages bundled into its screens. Permissive licenses (MIT, BSD, ISC,
# Apache-2.0) ask for their notice to go with every copy of the software.

# License files: LICENSE, LICENCE, COPYING and NOTICE, with any extension.
$script:noticeFilePattern = '^(LICEN[CS]E|COPYING|NOTICE)([.-].*)?$'

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

# New-ThirdPartyNotices writes the notices for the app at $App to $Destination.
function New-ThirdPartyNotices {
    param(
        [Parameter(Mandatory = $true)][string]$Root,
        [Parameter(Mandatory = $true)][string]$App,
        [Parameter(Mandatory = $true)][string]$Destination
    )

    $sections = New-Object Collections.Generic.List[string]
    $add = {
        param([string]$Name, [string]$Version, [string]$File, [string]$Within)
        $text = [IO.File]::ReadAllText($File).Trim()
        $where = if ($Within) { " ($Within)" } else { "" }
        $sections.Add(("=" * 78) + "`n$Name $Version$where`n" + ("-" * 78) + "`n$text`n")
    }

    # Go itself: the runtime and the standard library are inside every Go program.
    $goRoot = (Invoke-Native -FilePath "go" -Arguments @("env", "GOROOT") -FailureMessage "go env failed" -Capture | Out-String).Trim()
    $goVersion = (Invoke-Native -FilePath "go" -Arguments @("env", "GOVERSION") -FailureMessage "go env failed" -Capture | Out-String).Trim()
    & $add "Go" $goVersion (Join-Path $goRoot "LICENSE") ""

    # The modules compiled into the executable, as it records them.
    $moduleCache = (Invoke-Native -FilePath "go" -Arguments @("env", "GOMODCACHE") -FailureMessage "go env failed" -Capture | Out-String).Trim()
    $info = Invoke-Native -FilePath "go" -Arguments @("version", "-m", $App) -FailureMessage "go version -m failed" -Capture
    foreach ($line in $info) {
        $fields = "$line".Trim() -split '\s+'
        if ($fields.Count -lt 3 -or $fields[0] -ne "dep") {
            continue
        }
        $module, $version = $fields[1], $fields[2]
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
    $shipped = New-Object Collections.Generic.SortedSet[string]
    foreach ($entry in $lock["packages"].GetEnumerator()) {
        if ($entry.Key -like "node_modules/*" -and -not $entry.Value["dev"]) {
            [void]$shipped.Add($entry.Key.Substring("node_modules/".Length))
        }
    }
    [void]$shipped.Add("tailwindcss")
    foreach ($package in $shipped) {
        $folder = Join-Path $frontend ("node_modules\" + ($package -replace '/', '\'))
        $manifest = Get-Content -Raw -LiteralPath (Join-Path $folder "package.json") | ConvertFrom-Json
        $files = @(Get-ChildItem -LiteralPath $folder -File | Where-Object { $_.Name -match $script:noticeFilePattern })
        if ($files.Count -eq 0) {
            throw "No license file for the frontend package $package."
        }
        foreach ($file in $files) {
            & $add $package $manifest.version $file.FullName ""
        }
    }

    # In the language of the app's interface, as the read-me; the licenses
    # themselves stay as their authors wrote them.
    $header = @"
LingoReel - avisos de terceiros
======================================

O LingoReel é distribuído sob a licença MIT (veja LICENSE.txt). Ele inclui
programas de terceiros, cada um sob a sua própria licença, reproduzida abaixo
como os autores pedem. Nenhum deles pertence ao autor do LingoReel.

"@
    $text = $header + "`n" + ($sections -join "`n")
    # Notepad reads UTF-8 with a BOM correctly on every Windows version.
    [IO.File]::WriteAllText($Destination, ($text -replace "`r?`n", "`r`n"), (New-Object Text.UTF8Encoding($true)))
    return $sections.Count
}
