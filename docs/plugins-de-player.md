# Plugins de player

O LingoReel não toca o vídeo sozinho: ele abre o vídeo num **player externo** e desenha as
legendas por cima. Cada player é um **plugin**, um pacote Go que segue o mesmo contrato. O VLC é o
primeiro plugin e serve de exemplo: `internal/infrastructure/player/vlc`.

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

### `Launch` e `Running`

`Launch` abre o vídeo e só retorna quando ele estiver aberto. O `Running` devolvido controla o player:

- **Comandos:** tocar, pausar, buscar, mudar a velocidade, avançar quadro e escolher a faixa de áudio.
  - `Capabilities()` diz quais o player tem.
  - Comando que o player não tem **devolve erro**. Nunca finja que funcionou: o app mostra o botão
    desabilitado.
- **`Samples()`:** entrega observações (posição, duração, velocidade, estado). É fechado quando o
  player termina.
- **`ProcessID()`:** o processo do player. O overlay segue a janela dele.
- **`Err()`:** depois que `Samples()` fecha, diz se o player **caiu** (`playback.ErrPlayerLost`) ou
  se o usuário **fechou** (`nil`).
- **`Close()`:** encerra o player em tempo limitado. Chamar de novo não faz nada, e nenhum processo
  pode sobrar.

### Onde o player fica instalado

Para ser encontrado, o plugin também implementa `Hints() discovery.Hints`
(`internal/infrastructure/player/discovery`), com:

- os nomes do executável;
- as chaves do registro;
- os caminhos em Program Files e em `%LOCALAPPDATA%`;
- o nome na lista de programas instalados.

## Regras

- **Processo:** o player roda num Job Object, para nunca ficar órfão (veja `internal/infrastructure/windows/job`).
- **Legendas do player desligadas:** quem desenha as legendas é o LingoReel.
- **Controle local:**
  - só `127.0.0.1` ou pipe nomeado, nada de rede;
  - porta e senha aleatórias, por sessão;
  - argumentos passados separados, nunca uma linha de comando montada com texto.
- **`context`:** todo trabalho que espera respeita o `context` recebido.
- **Sem estado global:** o plugin é um valor comum, como `vlc.Plugin{Logger: ...}`.

## Passo a passo

1. Crie o pacote `internal/infrastructure/player/<seu-player>` com um tipo `Plugin`.
2. Implemente `Descriptor`, `Launch` e `Hints`.
3. Registre o plugin com uma linha na lista de plugins do ponto de composição (`plugins()` em
   `internal/spdev/players.go` e no app).
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
