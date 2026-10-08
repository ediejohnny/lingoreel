# Translating LingoReel

LingoReel speaks American English and Brazilian Portuguese, and it is ready for more languages: every text the app
shows comes from a catalog, one per language, and adding a language means adding a catalog. No code changes.

## Where the texts are

Each language has a folder named by its [BCP 47](https://www.rfc-editor.org/info/bcp47) tag under
`internal/presentation/i18n/locales/`:

```
internal/presentation/i18n/locales/
  en-US/
    app.json      texts of the Go side: the video bar, notices on the video, messages
    screens.json  texts of the app's screens (keys starting with "ui.")
    help.json     the documentation in Settings › Help › Documentation (keys starting with "ui.help.")
  pt-BR/
    app.json
    screens.json
    help.json
```

Both the Go side and the screens read every `.json` file in the folder, and the files are embedded in the app when it
is built. How the texts are split between files is only for the translator's comfort: a key may live in any of them.

Two catalogs have a special role:

- **`pt-BR` is the reference.** It is complete, and a text missing in another catalog falls back to it.
- **`en-US` is the default.** When Windows speaks a language the app has no catalog for, the app opens in English.

A catalog matches by its base language too: `pt-BR` serves a Windows in European Portuguese, and `en-US` one in British
English.

## The format

Each file is a JSON object of keys and texts:

```json
{
  "notice.volume": "Volume {percent}%",
  "plan.goal": { "one": "goal of {count} lap", "other": "goal of {count} laps" }
}
```

- **Keys** stay as they are: they are what the code asks for. Translate only the texts.
- **Parameters** in braces (`{percent}`, `{count}`, `{name}`) are filled in by the app. Keep every one of them, with the
  same name; their place in the sentence is yours to choose.
- **Plural forms** are an object with your language's
  [CLDR plural categories](https://cldr.unicode.org/index/cldr-spec/plural-rules) (`zero`, `one`, `two`, `few`,
  `many`, `other`). `other` is always required. The app picks the form by the `count` parameter.
- Numbers use the separator in `number.decimal` (`","` in Portuguese, `"."` in English).
- `app.languageName` is your language's name in your language, as the language list in Settings shows it.
- Texts that name a button or a setting should use the same words as the catalog's label for it, so the documentation
  matches the screen.

## Adding a language

1. Copy the `en-US` folder to a folder named by your language's tag, such as `fr` or `es-MX`.
2. Translate every text in the three files. Keep the keys and the parameters.
3. Check your catalog: `go test ./tests/presentation/i18n/`. The test fails when a key is missing or extra, or when a
   text lost or gained a parameter, and names the key.
4. Build the app (`scripts\build.ps1`). Pick your language in Settings › General › App language: the whole app switches
   at once, with no restart. Until someone picks one, the app speaks the Windows display language when it has a catalog
   for it, and English when it has none.
5. Open a pull request with your folder.

## The study pair

The app language is separate from the languages people study: the subtitle on top is the language being studied, and
the one below is the translation, both chosen per profile. A new catalog changes only the language the app speaks.

Detecting a subtitle's language from its text uses common words, one file per language under
`internal/domain/media/words/` (today `de`, `en`, `es`, `fr`, `it` and `pt`). A word list for your language helps the
app recognize subtitle files whose names do not say their language.

The subtitle preview in Settings shows a short sentence in each line's language, from the table in
`internal/domain/media/sample.go`. A language missing there is previewed in English.
