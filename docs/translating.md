# Translating LingoReel

LingoReel speaks Brazilian Portuguese today, and it is ready for more languages: every text the app shows comes
from a catalog, one per language, and adding a language means adding a catalog. No code changes.

## Where the texts are

Each language has a folder named by its [BCP 47](https://www.rfc-editor.org/info/bcp47) tag under
`internal/presentation/i18n/locales/`:

```
internal/presentation/i18n/locales/
  pt-BR/
    app.json      texts of the Go side: the video bar, notices on the video, messages
    screens.json  texts of the app's screens (keys starting with "ui.")
```

Both the Go side and the screens read the same files, which are embedded in the app when it is built.

`pt-BR` is the reference: it is complete, and a text missing in another catalog falls back to it.

## The format

Each file is a JSON object of keys and texts:

```json
{
  "notice.volume": "Volume {percent}%",
  "plan.goal": { "one": "meta de {count} volta", "other": "meta de {count} voltas" }
}
```

- **Keys** stay as they are: they are what the code asks for. Translate only the texts.
- **Parameters** in braces (`{percent}`, `{count}`, `{name}`) are filled in by the app. Keep every one of them, with the
  same name; their place in the sentence is yours to choose.
- **Plural forms** are an object with your language's
  [CLDR plural categories](https://cldr.unicode.org/index/cldr-spec/plural-rules) (`zero`, `one`, `two`, `few`,
  `many`, `other`). `other` is always required. The app picks the form by the `count` parameter.
- Numbers use the separator in `number.decimal` (`","` in Portuguese, `"."` in English).

## Adding a language

1. Copy the `pt-BR` folder to a folder named by your language's tag, such as `fr` or `en-US`.
2. Translate every text in both files. Keep the keys and the parameters.
3. Check your catalog: `go test ./tests/presentation/i18n/`. The test fails when a key is missing or extra, or when a
   text lost or gained a parameter, and names the key.
4. Build the app (`scripts\build.ps1`). The "Idioma da interface" field in Configurações › Geral appears as soon as there
   is more than one catalog; a choice there applies the next time the app opens. Until someone picks one, the app speaks
   the Windows display language when it has a catalog for it.
5. Open a pull request with your folder.

## The study pair

The interface language is separate from the languages people study: the subtitle on top is the language being studied,
and the one below is the translation, both chosen per profile. A new catalog changes only the language the app speaks.

Detecting a subtitle's language from its text uses common words, one file per language under
`internal/domain/media/words/`. A word list for your language helps the app recognize subtitle files whose names do not
say their language.
