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
- Três modos de estudo: Dupla, Tradução sob demanda (mostrar, ocultar ou segurar uma tecla para
  revelar) e Tradução sempre visível.
- Atalhos configuráveis: busca de 5, 10 e 30 s, passo de precisão, quadro a quadro, velocidade,
  repetir a frase e os últimos segundos.
- Suporte a mpv, VLC e PotPlayer, com o player preferido escolhido por você.
- Janela, tela cheia, vários monitores e escalas de DPI de 100% a 200%.
- Correção de legendas com o vídeo pausado: clique na fala para corrigir o texto ou os tempos,
  com desfazer/refazer (Ctrl+Z/Ctrl+Y). O texto original nunca se perde.
- Um arquivo de correções por vídeo (`Filme.dualsub.json`, ao lado do vídeo), que guarda as
  correções, a sincronização, as faixas e o modo escolhidos e os trechos salvos para estudo.
- Exportação da legenda corrigida e sincronizada em `.srt` ou `.ass`, para usar em outro player.
- Um único `.exe` portátil, sem instalação: o WebView2 vai embutido para o Windows 10 que não o tem.
- Abrir vídeos pelo menu do Windows ("Assistir com o LingoReel" e "Abrir com"), se você
  ligar a opção; o programa padrão de cada vídeo não muda.
- Registros diários dos últimos 14 dias e uma tela de diagnóstico para relatar problemas, sem o
  texto das legendas e sem enviar nada pela internet.

## Desenvolvimento

Requisitos: Windows 10/11 64-bit, Go 1.26+ e PowerShell 5.1+. Para `go test -race`, um `gcc`
no `PATH`.

| Comando | O que faz |
|---|---|
| `powershell -NoProfile -ExecutionPolicy Bypass -File scripts\check.ps1` | Formatação, análise estática e testes |
| `powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build.ps1` | Compila os executáveis em `build\bin` |
| `powershell -NoProfile -ExecutionPolicy Bypass -File scripts\package.ps1` | Monta o zip portátil em `build\dist` (depois do build) |
| `powershell -NoProfile -ExecutionPolicy Bypass -File scripts\dev.ps1` | Abre o app em modo de desenvolvimento (recarrega ao mudar o código) |

As versões seguem o [Versionamento Semântico](https://semver.org/lang/pt-BR/).

## Contribuindo

Contribuições são bem-vindas! Abra uma issue para conversar sobre a ideia ou envie um pull request.
Antes de enviar, rode o `scripts\check.ps1`: ele precisa passar sem erros. A interface segue o [Prettier](https://prettier.io/): `npm run format` na pasta `frontend` formata o código.

Quer usar outro player? Cada player é um plugin que segue o mesmo contrato: veja
[Plugins de player](docs/plugins-de-player.md).

## Licença e créditos

O LingoReel é distribuído sob a [licença MIT](LICENSE): você pode usar, copiar, modificar e
distribuir o projeto, inteiro ou em partes, inclusive em projetos comerciais, desde que mantenha o
aviso de copyright e o texto da licença.

Se este projeto for útil para você, **dê os créditos** mencionando o LingoReel e o autor,
Edie Johnny, com um link para o repositório. Não é uma obrigação da licença, mas é um gesto ético
que ajuda o projeto a continuar.
