package media_test

import (
	"strings"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/media"
)

const (
	englishDialogue = `I'm gonna have to think about it. That's sick! I'm not going there.
What are you doing here? You know what, just get out. They said the car was right there.
Why would you do that? Okay, okay. I have no idea how this works. Are you with me?
Your father was a good man. Yeah, I know. There is nothing we can do about it now.`
	portugueseDialogue = `Vou ter que pensar sobre isso. Que loucura! Eu não vou para lá. Não mesmo.
O que você está fazendo aqui? Você sabe o quê, só vai embora. Eles disseram que o carro estava aqui.
Por que você faria isso? Tudo bem, tudo bem. Eu não tenho ideia de como isso funciona. Você está comigo?
Seu pai foi um bom homem. Sim, eu sei. Não tem nada que a gente possa fazer agora. Vamos embora, então.`
	spanishDialogue = `Voy a tener que pensarlo. ¡Qué locura! No voy a ir allí. ¿Qué estás haciendo aquí?
Sabes qué, vete. Dijeron que el coche estaba ahí. ¿Por qué harías eso? Está bien, está bien.
No tengo idea de cómo funciona esto. ¿Estás conmigo? Tu padre fue un buen hombre. Sí, lo sé.
No hay nada que podamos hacer ahora. Vamos, pues. Muchas gracias. Estoy bien, nada más.`
)

func TestDetectLanguageRecognizesDialogue(t *testing.T) {
	t.Parallel()
	cases := map[string]media.LanguageTag{
		englishDialogue:                     "en",
		portugueseDialogue:                  "pt",
		spanishDialogue:                     "es",
		strings.ToUpper(portugueseDialogue): "pt", // capital-letter subtitles, like the real release
	}
	for text, want := range cases {
		if got := media.DetectLanguage(text); got != want {
			t.Errorf("DetectLanguage(%.30q…) = %q, want %q", text, got, want)
		}
	}
}

// Too little text, or two languages mixed half and half, must stay unknown:
// a wrong guess would put a file on the wrong line.
func TestDetectLanguageRefusesToGuess(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"vazio":           "",
		"só números":      "1 2 3 00:01:02,000 --> 00:01:03,000",
		"poucas palavras": "What? Não. Okay.",
		"metade e metade": englishDialogue + " " + portugueseDialogue,
		"outra língua":    strings.Repeat("Ich weiß nicht, was du meinst. ", 40),
		// Words that exist in two languages prove neither: "está", "vamos"
		// and "para" are Portuguese and Spanish, "bien" Spanish and French.
		"palavras comuns":   strings.Repeat("está vamos para bien ", 40),
		"uma palavra comum": strings.Repeat("para ", 60),
	}
	for name, text := range cases {
		if got := media.DetectLanguage(text); got != "" {
			t.Errorf("%s: DetectLanguage = %q, want unknown", name, got)
		}
	}
}

func TestDetectLanguageOnlyReadsTheBeginningOfHugeText(t *testing.T) {
	t.Parallel()
	// 5000 words of English, then a flood of Portuguese: the start decides.
	text := strings.Repeat("what are you doing here ", 1000) + strings.Repeat("você não está aqui ", 100000)
	if got := media.DetectLanguage(text); got != "en" {
		t.Fatalf("DetectLanguage = %q, want en (only the first words count)", got)
	}
}
