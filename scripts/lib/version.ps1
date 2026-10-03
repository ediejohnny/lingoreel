# version.ps1 - The version of a build, shared by build.ps1 and package.ps1.

$fallbackVersion = "0.0.0-dev"
$semverPattern = '^\d+\.\d+\.\d+'

# Turns "v0.3.0-2-gabc1234-dirty" into "0.3.0-2-gabc1234-dirty"; untagged
# history becomes "0.0.0-dev+<commit>". A version given wins.
function Resolve-BuildVersion {
    param(
        [Parameter(Mandatory = $true)][string]$Root,
        [string]$Version = ""
    )
    if ($Version) {
        return $Version.TrimStart("v")
    }
    try {
        $described = Invoke-Native -FilePath "git" -Arguments @("-C", $Root, "describe", "--tags", "--always", "--dirty") `
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

# Reads a text property of an executable ("ProductVersion", "CompanyName") the
# way Windows itself does. .NET's FileVersionInfo returns empty strings for the
# version block Wails writes, while Windows and the Explorer read it.
function Get-FileProperty {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][string]$Name
    )
    if (-not ("LingoReelBuild.VersionBlock" -as [type])) {
        Add-Type -TypeDefinition @"
using System;
using System.Runtime.InteropServices;
namespace LingoReelBuild {
    public static class VersionBlock {
        [DllImport("version.dll", CharSet = CharSet.Unicode)]
        static extern int GetFileVersionInfoSize(string file, out int handle);
        [DllImport("version.dll", CharSet = CharSet.Unicode)]
        static extern bool GetFileVersionInfo(string file, int handle, int size, byte[] data);
        [DllImport("version.dll", CharSet = CharSet.Unicode)]
        static extern bool VerQueryValue(byte[] block, string path, out IntPtr value, out int length);

        public static string Read(string file, string name) {
            int handle;
            int size = GetFileVersionInfoSize(file, out handle);
            if (size <= 0) return "";
            byte[] data = new byte[size];
            IntPtr value;
            int length;
            if (!GetFileVersionInfo(file, 0, size, data) || !VerQueryValue(data, "\\VarFileInfo\\Translation", out value, out length) || length < 4) return "";
            string key = Marshal.ReadInt16(value).ToString("x4") + Marshal.ReadInt16(value, 2).ToString("x4");
            if (!VerQueryValue(data, "\\StringFileInfo\\" + key + "\\" + name, out value, out length) || length == 0) return "";
            return Marshal.PtrToStringUni(value);
        }
    }
}
"@
    }
    return [LingoReelBuild.VersionBlock]::Read($Path, $Name)
}

# The numbers of a version, as Windows file versions need them: "0.3.0".
function Get-NumericVersion {
    param([Parameter(Mandatory = $true)][string]$BuildVersion)
    if ($BuildVersion -match $semverPattern) {
        return $Matches[0]
    }
    return "0.0.0"
}
