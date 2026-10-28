# Licenses: what can go into LingoReel

LingoReel is free and open source under the [MIT license](../LICENSE), made to help anyone learn languages: anyone may
use, study, change and share it, for any purpose. Everything it carries must keep that promise. This page is the rule
for every contribution.

## Your contribution

Code, text, icons and other files you contribute are under the MIT license, like the rest of the project. Only
contribute what you wrote yourself or what you may contribute under it.

## Code and files from others

- **What goes inside the app** (Go modules compiled into the executable, packages bundled into its screens, models
  embedded in it) must be under a permissive license: MIT, BSD-2-Clause, BSD-3-Clause, ISC, Apache-2.0, 0BSD or the
  Unlicense. MIT is preferred; when a piece is small, write our own instead of adding a dependency.
- **Never**: copyleft licenses (GPL, AGPL, LGPL or MPL compiled in), non-commercial licenses (CC BY-NC and the like),
  licenses with extra conditions, and anything whose license is missing or unclear. If nothing fits, we make our own.
- **AI models**: the weights have a license of their own, apart from the code that runs them, and so do the data they
  were trained on when its terms reach the weights. Check both on the model's own page before proposing it.
- **Optional downloads** (a voice language: ONNX Runtime and a speech model) are fetched only when the person asks,
  and go with their licenses beside them. They follow the same rule.
- **Programs the person installs** (VLC, PotPlayer, FFmpeg) are used as they are, never shipped with the app.
- **Media**: no real films, subtitles of real films, or song lyrics in the repository. Test files are made by
  `scripts/make-fixtures.ps1` or written by hand; screenshots use openly licensed films, credited in the README.

## How the rule is kept

- Every license that goes with the app is in its Help › About, written by `scripts/update-notices.ps1` from each
  piece's own license file. The check (`scripts/check.ps1`) refuses notices that no longer match what the app is built
  from, and a dependency whose license is not one of those allowed above.
- How the downloadable files were made is in [speech-packs.md](speech-packs.md).
- Each release says what goes with it, under which license, and how each condition is met.
