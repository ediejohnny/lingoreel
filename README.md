# LingoReel

Aplicativo para Windows 10/11 que ajuda a estudar idiomas com filmes e séries. Ele abre o vídeo
no seu player preferido e desenha, por cima, duas legendas sincronizadas: a do idioma estudado e a
tradução em português, que pode ficar sempre visível ou aparecer só quando você pedir.

> **Estado:** em desenvolvimento (versões `0.x`). A primeira versão estável será a `1.0.0`.

## Recursos planejados

- Leitura das legendas internas de arquivos `.mkv`, sem alterar o arquivo original.
- Leitura de legendas externas (`.srt`, `.ass`, `.ssa`, `.vtt`) da pasta do vídeo e das
  subpastas `Subs`/`Subtitles`, para quando o filme não traz a legenda que você quer.
- Duas legendas simultâneas, com sincronização e deslocamento (offset) independentes.
- Tradução sob demanda: mostrar, ocultar ou segurar uma tecla para revelar.
- Suporte a mpv, VLC e PotPlayer, com o player preferido escolhido por você.
- Janela, tela cheia, vários monitores e escalas de DPI de 100% a 200%.
- Edição de legendas com o vídeo pausado, com desfazer/refazer.

## Desenvolvimento

Requisitos: Windows 10/11 64-bit, Go 1.26+ e PowerShell 5.1+. Para `go test -race`, um `gcc`
no `PATH`.

| Comando | O que faz |
|---|---|
| `powershell -NoProfile -ExecutionPolicy Bypass -File scripts\check.ps1` | Formatação, análise estática e testes |
| `powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build.ps1` | Compila os executáveis em `build\bin` |

As versões seguem o [Versionamento Semântico](https://semver.org/lang/pt-BR/).

## Contribuindo

Contribuições são bem-vindas! Abra uma issue para conversar sobre a ideia ou envie um pull request.
Antes de enviar, rode o `scripts\check.ps1`: ele precisa passar sem erros.

## Licença e créditos

O LingoReel é distribuído sob a [licença MIT](LICENSE): você pode usar, copiar, modificar e
distribuir o projeto, inteiro ou em partes, inclusive em projetos comerciais, desde que mantenha o
aviso de copyright e o texto da licença.

Se este projeto for útil para você, **dê os créditos** mencionando o LingoReel e o autor,
Edie Johnny, com um link para o repositório. Não é uma obrigação da licença, mas é um gesto ético
que ajuda o projeto a continuar.
