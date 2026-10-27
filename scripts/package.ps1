# package.ps1 - Packs the built app into a portable zip in build\dist: the
# executable, the license, the third-party notices and a short read-me. Run
# build.ps1 first.
#
# Usage:
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts\package.ps1 [-Version 1.2.3]

param(
    [string]$Version = ""
)

$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "lib\native.ps1")
. (Join-Path $PSScriptRoot "lib\version.ps1")
. (Join-Path $PSScriptRoot "lib\notices.ps1")

$root = Split-Path -Parent $PSScriptRoot
$app = Join-Path $root "build\bin\LingoReel.exe"
$license = Join-Path $root "LICENSE"
$distDir = Join-Path $root "build\dist"

$buildVersion = Resolve-BuildVersion -Root $root -Version $Version
$fileVersion = Get-NumericVersion -BuildVersion $buildVersion

# The read-me that goes in the zip, for whoever downloads it, in Brazilian
# Portuguese like the app's first audience; the app itself also speaks
# English.
$readMe = @"
LingoReel $fileVersion
============================

Duas legendas sincronizadas sobre o vídeo, para estudar idiomas com filmes e séries.

COMO USAR
1. Extraia esta pasta para onde quiser (por exemplo, Documentos\LingoReel).
2. Abra "LingoReel.exe". Na primeira vez, um assistente rápido pergunta a língua
   que você fala, a que estuda, o modo de estudo e o player.
3. Arraste um vídeo para a janela. Configurações > Ajuda > Documentação explica tudo
   o que o app faz.

REQUISITOS
- Windows 10 ou 11, 64 bits.
- Um player de vídeo compatível: o VLC ou o PotPlayer (o app ajuda a instalar).
- Opcional e grátis: o FFmpeg, para escolher a legenda pelo áudio, seguir a voz dos
  atores um pouco à frente e mostrar miniaturas na barra.

ONDE FICAM SEUS DADOS
Tudo fica em %APPDATA%\LingoReel, nunca na pasta do vídeo:
- Configurações e perfis: config.json e profiles.json.
- As correções, a sincronia e os trechos de cada vídeo: pasta projects.
- As miniaturas da barra: pasta thumbnails (até 4 GB; Configurações > Aparência limpa).
- Registros (logs) dos últimos 14 dias: pasta logs.
Nada é enviado pela internet.

PARA REMOVER
- Se você ligou "Abrir vídeos pelo Windows" (Configurações > Avançado), desligue antes.
- Apague esta pasta. Para apagar também os seus dados, apague %APPDATA%\LingoReel.

LICENÇA
MIT. Veja o arquivo LICENSE. (c) Edie Johnny.
As licenças dos programas de terceiros incluídos estão em AVISOS-DE-TERCEIROS.txt
e no próprio app, em Configurações > Ajuda > Sobre.
"@

# The notices' introduction, in the language of the read-me; the licenses
# themselves stay as their authors wrote them (the app carries the same ones).
$noticesHeader = @"
LingoReel - avisos de terceiros
======================================

O LingoReel é distribuído sob a licença MIT (veja LICENSE.txt). Ele inclui
programas de terceiros, cada um sob a sua própria licença, reproduzida abaixo
como os autores pedem. Nenhum deles pertence ao autor do LingoReel.

"@

foreach ($required in @($app, $license)) {
    if (-not (Test-Path -LiteralPath $required)) {
        throw "File not found: $required. Run scripts\build.ps1 first."
    }
}

Write-Host "[1/3] Checking the version of the built app..."
$built = Get-FileProperty -Path $app -Name "ProductVersion"
if ($built -ne $fileVersion) {
    throw "The built app is version '$built', but the package would be '$fileVersion'. Run scripts\build.ps1 again."
}

Write-Host "[2/3] Assembling the package folder..."
$name = "LingoReel $fileVersion"
$staging = Join-Path $distDir $name
if (Test-Path -LiteralPath $staging) {
    Remove-Item -LiteralPath $staging -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $staging | Out-Null
Copy-Item -LiteralPath $app -Destination $staging
Copy-Item -LiteralPath $license -Destination (Join-Path $staging "LICENSE.txt")
if (-not (Test-ThirdPartyNotices -Root $root)) {
    throw "The third-party notices are out of date. Run scripts\update-notices.ps1."
}
$notices = $noticesHeader + "`n" + [IO.File]::ReadAllText((Get-NoticesPath -Root $root))
# Notepad reads UTF-8 with a BOM correctly on every Windows version.
$withBom = New-Object Text.UTF8Encoding($true)
[IO.File]::WriteAllText((Join-Path $staging "AVISOS-DE-TERCEIROS.txt"), ($notices -replace "`r?`n", "`r`n"), $withBom)
[IO.File]::WriteAllText((Join-Path $staging "LEIA-ME.txt"), ($readMe -replace "`r?`n", "`r`n"), $withBom)

Write-Host "[3/3] Compressing..."
$zip = Join-Path $distDir "$name.zip"
if (Test-Path -LiteralPath $zip) {
    Remove-Item -LiteralPath $zip -Force
}
Compress-Archive -Path (Join-Path $staging "*") -DestinationPath $zip
Remove-Item -LiteralPath $staging -Recurse -Force

$zipMb = [math]::Round((Get-Item -LiteralPath $zip).Length / 1MB, 1)
Write-Host ""
Write-Host "Package ready." -ForegroundColor Green
Write-Host "  $zip ($zipMb MB)"
