# Plugins de player

O LingoReel não toca o vídeo sozinho: ele abre o vídeo num **player externo** e desenha as
legendas por cima. Cada player é um **plugin**, um pacote Go que segue o mesmo contrato.

Há dois para servirem de exemplo: o **VLC** (`internal/infrastructure/player/vlc`), que desenha
dentro de uma janela do app e é controlado por um canal local, e o **PotPlayer**
(`internal/infrastructure/player/potplayer`), que desenha na própria janela e é controlado por
mensagens do Windows. Siga o que estiver mais perto do seu player.

Os plugins são compilados junto com o app, porque o pacote `plugin` do Go não funciona no Windows. Por
isso, criar um plugin é escrever um pacote e acrescentar uma linha na lista de plugins.

## O contrato

O contrato está em `internal/application/players`:

```go
type Plugin interface {
    Descriptor() player.Descriptor
    Launch(ctx context.Context, executable string, request player.LaunchRequest) (Running, error)
}
```

### `Descriptor`

Diz o que o usuário vê sobre o player:

- **`ID`:** curto, minúsculo e sem espaços (`vlc`, `mpv`, `mpc-hc`). É gravado nas configurações.
- **`Name`:** o nome mostrado na tela.
- **`Install.WingetID`:** o pacote do winget usado na instalação automática. Fica vazio se não houver pacote.
- **`Install.DownloadURL`:** a página oficial de download. Tem que ser `https`.
- **`Install.Benefit`:** uma frase, em português, com o motivo para escolher esse player. Aparece no botão (?).
- **`DrawsInHost`:** `true` quando o player consegue desenhar o vídeo dentro de uma janela do app
  (`LaunchRequest.Host`). É o caminho preferido:
  - a janela passa a ser do LingoReel, com fechar (X, Alt+F4), tela cheia, teclado e a barra
    de controles que aparece ao passar o mouse embaixo;
  - o player não deve tratar mouse nem teclado dentro dela;
  - com `Host` zero, ele abre a própria janela, e o overlay segue essa janela.

  Exemplos: o VLC usa `--drawable-hwnd` e o mpv usa `--wid`.
- **`FixedAudioTrack`:** `true` quando o player não aceita escolher a faixa de áudio. A tela da
  sessão então desabilita a escolha e explica que ela fica no menu do próprio player. O PotPlayer é
  assim.

### `Launch` e `Running`

`Launch` abre o vídeo e só retorna quando ele estiver aberto. O `Running` devolvido controla o player:

- **Comandos:** tocar, pausar, buscar, mudar a velocidade, avançar quadro e escolher a faixa de áudio.
  - `Capabilities()` diz quais o player tem.
  - O volume é opcional: quem declara `Volume` implementa `SetVolume` (`session.VolumePlayer`),
    de 0 a 1, em que 1 é o 100% do player.
  - Comando que o player não tem **devolve erro**. Nunca finja que funcionou: o app mostra o botão
    desabilitado.
- **`Samples()`:** entrega observações (posição, duração, velocidade, estado). É fechado quando o
  player termina.
- **`ProcessID()`:** o processo do player. O overlay segue a janela dele.
- **`Err()`:** depois que `Samples()` fecha, diz se o player **caiu** (`playback.ErrPlayerLost`) ou
  se o usuário **fechou** (`nil`).
- **`Close()`:** encerra o player em tempo limitado. Chamar de novo não faz nada, e nenhum processo
  pode sobrar.
- **`Reconnecting()`** (opcional, `players.Reconnector`): `true` enquanto o player está vivo mas não
  responde e o plugin tenta de novo. O painel mostra "Reconectando ao player…" em vez de encerrar.
- **`Window()` e `VideoArea()`** (opcional, `players.Windowed`): para um player que desenha na
  própria janela, dizem **qual** janela é a dele e **qual pedaço** dela tem a imagem. Sem isso, o
  overlay procura a maior janela do processo e cobre ela inteira. O PotPlayer precisa dos dois: a
  janela dele tem a lista de reprodução e a barra ao lado da imagem. As duas podem ser zero a
  qualquer momento (janela ainda não aberta, ou já fechada).

### Onde o player fica instalado

Para ser encontrado, o plugin também implementa `Hints() discovery.Hints`
(`internal/infrastructure/player/discovery`), com:

- os nomes do executável;
- as chaves do registro;
- os caminhos em Program Files e em `%LOCALAPPDATA%`;
- o nome na lista de programas instalados.

### Dois caminhos de controle

O plugin fala com o player do jeito que o player aceita:

- **Canal local** (VLC, mpv): uma interface HTTP em `127.0.0.1` ou um pipe nomeado, com porta e
  senha por sessão.
- **Mensagens de janela** (PotPlayer): `SendMessageTimeout(janela, WM_USER, código, valor)`, com
  `SMTO_ABORTIFHUNG`, que é o que distingue um player travado de um que respondeu. É o caminho do
  pacote `internal/infrastructure/player/potplayer`, e `win32.SendMessageTimeout` é o auxiliar.

Em qualquer um deles, o plugin **pergunta de novo com espera crescente** antes de dar a sessão por
perdida (RNF-013) e mostra isso em `Reconnecting()`.

## Regras

- **Processo:** o player roda num Job Object, para nunca ficar órfão (veja `internal/infrastructure/windows/job`).
- **Legendas do player desligadas:** quem desenha as legendas é o LingoReel.
- **Controle local:**
  - só `127.0.0.1`, pipe nomeado ou mensagens de janela, nada de rede;
  - porta e senha aleatórias, por sessão;
  - argumentos passados separados, nunca uma linha de comando montada com texto;
  - o caminho do vídeo vai absoluto, para o player nunca o ler como opção.
- **`context`:** todo trabalho que espera respeita o `context` recebido.
- **Sem estado global:** o plugin é um valor comum, como `vlc.Plugin{Logger: ...}`.

## Passo a passo

1. Crie o pacote `internal/infrastructure/player/<seu-player>` com um tipo `Plugin`.
2. Implemente `Descriptor`, `Launch` e `Hints`.
3. Registre o plugin com uma linha na lista `All` de `internal/infrastructure/player/plugins`.
   O app e a ferramenta de desenvolvimento usam essa mesma lista.
4. Escreva os testes em `tests/infrastructure/player/<seu-player>/` e passe pelo **kit de
   conformidade**:

   ```go
   func TestMeuPlayerPassaNoKit(t *testing.T) {
       plugintest.Descriptor(t, meuplayer.Plugin{})
       plugintest.Launch(t, meuplayer.Plugin{}, executavel, video)
   }
   ```

   O kit (`tests/infrastructure/player/plugintest`) confere:
   - que o descritor é válido;
   - que um `Launch` cancelado desiste rápido e não deixa processo;
   - que cada capacidade declarada funciona e as não declaradas dão erro;
   - que `Close` encerra tudo em tempo limitado, inclusive quando chamado duas vezes.
5. Rode `scripts\check.ps1`. Ele precisa passar sem erros.
