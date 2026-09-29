package media

import (
	"regexp"
	"strings"
)

// LanguageTag is a BCP 47 tag in its simple form: a lowercase base language
// ("en", "pt") optionally followed by a region or script ("pt-BR", "zh-Hans").
type LanguageTag string

// Base returns the language without region, such as "pt" for "pt-BR".
func (t LanguageTag) Base() string {
	base, _, _ := strings.Cut(string(t), "-")
	return base
}

// Region returns the part after the base language, such as "BR" for "pt-BR".
func (t LanguageTag) Region() string {
	_, region, _ := strings.Cut(string(t), "-")
	return region
}

// undeterminedLanguage is the ISO 639-2 code for an unknown language.
const undeterminedLanguage = "und"

// alpha3ToAlpha2 maps ISO 639-2 (bibliographic and terminology) codes to ISO 639-1.
var alpha3ToAlpha2 = map[string]string{
	"eng": "en", "por": "pt", "spa": "es", "fre": "fr", "fra": "fr", "ger": "de", "deu": "de",
	"ita": "it", "jpn": "ja", "kor": "ko", "chi": "zh", "zho": "zh", "rus": "ru", "ara": "ar",
	"dut": "nl", "nld": "nl", "pol": "pl", "tur": "tr", "swe": "sv", "nor": "no", "nob": "nb",
	"nno": "nn", "dan": "da", "fin": "fi", "gre": "el", "ell": "el", "heb": "he", "hin": "hi",
	"hun": "hu", "cze": "cs", "ces": "cs", "rum": "ro", "ron": "ro", "tha": "th", "vie": "vi",
	"ind": "id", "ukr": "uk", "cat": "ca", "hrv": "hr", "srp": "sr", "slo": "sk", "slk": "sk",
	"slv": "sl", "bul": "bg", "est": "et", "lav": "lv", "lit": "lt", "may": "ms", "msa": "ms",
	"per": "fa", "fas": "fa", "ice": "is", "isl": "is", "glg": "gl", "baq": "eu", "eus": "eu",
	"tam": "ta", "tel": "te", "ben": "bn", "urd": "ur", "mal": "ml", "kan": "kn", "mar": "mr",
}

// languageNames maps lowercase words found in track names to base languages.
var languageNames = map[string]string{
	"english": "en", "inglês": "en", "ingles": "en", "inglés": "en",
	"portuguese": "pt", "português": "pt", "portugues": "pt", "portugués": "pt", "brazilian": "pt",
	"spanish": "es", "español": "es", "espanol": "es", "espanhol": "es", "castellano": "es", "castilian": "es",
	"french": "fr", "français": "fr", "francais": "fr", "francês": "fr",
	"german": "de", "deutsch": "de", "alemão": "de",
	"italian": "it", "italiano": "it",
	"japanese": "ja", "japonês": "ja", "日本語": "ja",
	"korean": "ko", "coreano": "ko", "한국어": "ko",
	"chinese": "zh", "chinês": "zh", "mandarin": "zh", "中文": "zh",
	"russian": "ru", "russo": "ru", "русский": "ru",
}

// regionHint maps a pattern found in a track name to the region it implies for
// a base language.
type regionHint struct {
	base    string
	pattern *regexp.Regexp
	region  string
}

var regionHints = []regionHint{
	{"pt", regexp.MustCompile(`(?i)brazil|brasil|\bpt[-_ ]?br\b|\bbr\b`), "BR"},
	{"pt", regexp.MustCompile(`(?i)portugal|european|europeu|\bpt[-_ ]?pt\b`), "PT"},
	{"es", regexp.MustCompile(`(?i)latin|latino|latam|latinoam|\bes[-_ ]?419\b`), "419"},
	{"es", regexp.MustCompile(`(?i)spain|españa|espana|castilian|castellano|\bes[-_ ]?es\b`), "ES"},
	{"zh", regexp.MustCompile(`(?i)simplified|简体|\bchs\b`), "Hans"},
	{"zh", regexp.MustCompile(`(?i)traditional|繁體|\bcht\b`), "Hant"},
	{"en", regexp.MustCompile(`(?i)\b(us|usa|american)\b`), "US"},
	{"en", regexp.MustCompile(`(?i)\b(uk|british)\b`), "GB"},
}

var bcp47Pattern = regexp.MustCompile(`^([A-Za-z]{2,3})(?:[-_]([A-Za-z]{4}|[A-Za-z]{2}|[0-9]{3}))?`)
var nameWordPattern = regexp.MustCompile(`[\p{L}]+`)

// NormalizeLanguage returns the tag of a track from its declared BCP 47 tag,
// its ISO 639-2 code and its name, in that order of trust. The name only fills
// what the declared fields leave open, such as the region in "por" plus
// "Brazilian Portuguese". It returns "" when nothing identifies the language.
func NormalizeLanguage(bcp47, iso639, name string) LanguageTag {
	base, region := parseTag(bcp47)
	if base == "" {
		base, _ = parseTag(iso639)
	}
	if base == "" {
		base = languageFromName(name)
	}
	if base == "" {
		return ""
	}
	if region == "" {
		region = regionFromName(base, name)
	}
	if region == "" {
		return LanguageTag(base)
	}
	return LanguageTag(base + "-" + region)
}

// parseTag reads "pt-BR", "pt_BR", "por" or "en" into a base and region.
func parseTag(value string) (string, string) {
	trimmed := strings.TrimSpace(value)
	match := bcp47Pattern.FindStringSubmatch(trimmed)
	if match == nil {
		return "", ""
	}
	// The code must end the value or be followed by a separator, so a word
	// such as "english" is not read as the code "eng".
	if rest := trimmed[len(match[1]):]; rest != "" && rest[0] != '-' && rest[0] != '_' {
		return "", ""
	}
	base := strings.ToLower(match[1])
	if len(base) == 3 {
		if base == undeterminedLanguage {
			return "", ""
		}
		mapped, known := alpha3ToAlpha2[base]
		if !known {
			return "", ""
		}
		base = mapped
	}
	return base, canonicalRegion(match[2])
}

// canonicalRegion upper-cases country codes and title-cases scripts.
func canonicalRegion(region string) string {
	if len(region) == 4 {
		return strings.ToUpper(region[:1]) + strings.ToLower(region[1:])
	}
	return strings.ToUpper(region)
}

func languageFromName(name string) string {
	for _, word := range nameWordPattern.FindAllString(strings.ToLower(name), -1) {
		if base, found := languageNames[word]; found {
			return base
		}
	}
	return ""
}

func regionFromName(base, name string) string {
	for _, hint := range regionHints {
		if hint.base == base && hint.pattern.MatchString(name) {
			return hint.region
		}
	}
	return ""
}
