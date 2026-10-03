# package.ps1 - Packs the built app into a portable zip in build\dist: the
# executable, the license and a short read-me. Run build.ps1 first.
#
# Usage:
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts\package.ps1 [-Version 1.2.3]

param(
    [string]$Version = ""
)

$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "lib\native.ps1")
. (Join-Path $PSScriptRoot "lib\version.ps1")

$root = Split-Path -Parent $PSScriptRoot
$app = Join-Path $root "build\bin\LingoReel.exe"
$license = Join-Path $root "LICENSE"
$distDir = Join-Path $root "build\dist"

$buildVersion = Resolve-BuildVersion -Root $root -Version $Version
$fileVersion = Get-NumericVersion -BuildVersion $buildVersion

# The read-me that goes in the zip, for whoever downloads it.
$readMe = @"
LingoReel $fileVersion
============================

Duas legendas sincronizadas sobre o vídeo, para estudar idiomas com filmes e séries.

COMO USAR
1. Extraia esta pasta para onde quiser (por exemplo, Documentos\LingoReel).
2. Abra "LingoReel.exe" e arraste um vídeo para a janela.
3. Se nenhum player compatível estiver instalado, o app oferece instalar o VLC.

REQUISITOS
- Windows 10 ou 11, 64 bits.
- Um player de vídeo compatível (o VLC; o app ajuda a instalar).

ONDE FICAM SEUS DADOS
- Configurações e perfis: %APPDATA%\LingoReel
- Registros (logs) dos últimos 14 dias: %APPDATA%\LingoReel\logs
- As correções de legenda de cada vídeo: ao lado do vídeo, em "Filme.dualsub.json".
Nada é enviado pela internet.

PARA REMOVER
- Se você ligou "Abrir vídeos pelo Windows" (Configurações > Avançado), desligue antes.
- Apague esta pasta. Para apagar também as configurações, apague %APPDATA%\LingoReel.

LICENÇA
MIT. Veja o arquivo LICENSE. (c) Edie Johnny.
"@

foreach ($required in @($app, $license)) {
    if (-not (Test-Path -LiteralPath $required)) {
        throw "Arquivo não encontrado: $required. Rode scripts\build.ps1 antes."
    }
}

Write-Host "[1/3] Conferindo a versão do app compilado..."
$built = Get-FileProperty -Path $app -Name "ProductVersion"
if ($built -ne $fileVersion) {
    throw "O app compilado é a versão '$built', mas o pacote seria '$fileVersion'. Rode scripts\build.ps1 de novo."
}

Write-Host "[2/3] Montando a pasta do pacote..."
$name = "LingoReel $fileVersion"
$staging = Join-Path $distDir $name
if (Test-Path -LiteralPath $staging) {
    Remove-Item -LiteralPath $staging -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $staging | Out-Null
Copy-Item -LiteralPath $app -Destination $staging
Copy-Item -LiteralPath $license -Destination (Join-Path $staging "LICENSE.txt")
# Notepad reads UTF-8 with a BOM correctly on every Windows version.
[IO.File]::WriteAllText((Join-Path $staging "LEIA-ME.txt"), ($readMe -replace "`r?`n", "`r`n"), (New-Object Text.UTF8Encoding($true)))

Write-Host "[3/3] Compactando..."
$zip = Join-Path $distDir "$name.zip"
if (Test-Path -LiteralPath $zip) {
    Remove-Item -LiteralPath $zip -Force
}
Compress-Archive -Path (Join-Path $staging "*") -DestinationPath $zip
Remove-Item -LiteralPath $staging -Recurse -Force

$zipMb = [math]::Round((Get-Item -LiteralPath $zip).Length / 1MB, 1)
Write-Host ""
Write-Host "Pacote pronto." -ForegroundColor Green
Write-Host "  $zip ($zipMb MB)"
