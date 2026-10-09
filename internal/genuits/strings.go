package genuits

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/SpecArch/specarch/internal/genui"
)

// languageCode is a language as BCP 47 writes one, such as id or pt-BR.
var languageCode = regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)

// placeholder is a value a text has filled in, such as {count}.
var placeholder = regexp.MustCompile(`\{[a-z]+\}`)

// translationFile is the file in the output folder that holds a
// language's texts, which the project writes and the generator only reads.
func translationFile(language string) string {
	return "strings." + language + ".json"
}

// metadataOf is a page's metadata: its title in the chosen language.
func metadataOf(pageName string) string {
	return fmt.Sprintf("export async function generateMetadata(): Promise<Metadata> {\n  return { title: stringOf(%q, await chosenLanguage()) };\n}\n\n", pageName+".title")
}

// translations reads settings.translations, the languages besides the
// specification's, each from its file in the output folder. Every key the
// screens use must have an entry in each, and a key no screen uses is
// reported and left out.
func (g *gen) translations(keys map[string]string, existing []genui.File) ([]string, map[string]map[string]string, bool) {
	raw, named := g.settings["translations"]
	if !named {
		return nil, nil, true
	}
	languages := list(raw)
	if languages == nil {
		g.problem("error", "/", "the ui target's settings.translations is not a list; list the languages besides %s, such as [id]", text(g.settings["language"]))
		return nil, nil, false
	}
	files := map[string]string{}
	for _, f := range existing {
		files[f.Path] = f.Content
	}
	ok := true
	var codes []string
	out := map[string]map[string]string{}
	for _, l := range languages {
		code := text(l)
		file := translationFile(code)
		switch {
		case !languageCode.MatchString(code):
			g.problem("error", "/", "the ui target's settings.translations names %q, which is not a language as BCP 47 writes one, such as id or pt-BR", code)
			ok = false
			continue
		case code == text(g.settings["language"]) || out[code] != nil:
			g.problem("error", "/", "the ui target's settings.translations names %s twice, or as the specification's own language; name each other language once", code)
			ok = false
			continue
		}
		content, found := files[file]
		if !found {
			g.problem("error", "/", "the ui target's settings.translations names %s, and %s is not in the output folder; write it, an object with the %s text of each of the %d keys the screens use", code, file, code, len(keys))
			ok = false
			continue
		}
		var entries map[string]any
		if err := json.Unmarshal([]byte(content), &entries); err != nil {
			g.problem("error", "/", "%s is not a JSON object of string keys and texts: %v", file, err)
			ok = false
			continue
		}
		texts := map[string]string{}
		for _, k := range sortedKeys(entries) {
			t, isText := entries[k].(string)
			_, used := keys[k]
			switch {
			case !used:
				g.problem("warning", "/", "%s has an entry for %s, which no screen uses; it is left out", file, k)
			case !isText || strings.TrimSpace(t) == "":
				g.problem("error", "/", "%s gives %s no text; give it the %s text of %q", file, k, code, keys[k])
				ok = false
			default:
				if missing := missingPlaceholders(keys[k], t); missing != "" {
					g.problem("error", "/", "%s gives %s without %s, which the %s text fills in; keep it in the translation", file, k, missing, text(g.settings["language"]))
					ok = false
				}
				texts[k] = t
			}
		}
		for _, k := range sortedKeys(anyMap(keys)) {
			if _, has := entries[k]; !has {
				g.problem("error", "/", "%s has no entry for %s, which the screens use, so a page would show the key; add its %s text of %q", file, k, code, keys[k])
				ok = false
			}
		}
		codes = append(codes, code)
		out[code] = texts
	}
	return codes, out, ok
}

// missingPlaceholders are the placeholders of a text its translation leaves out.
func missingPlaceholders(text, translated string) string {
	var missing []string
	for _, p := range placeholder.FindAllString(text, -1) {
		if !strings.Contains(translated, p) {
			missing = append(missing, p)
		}
	}
	return strings.Join(missing, ", ")
}

// stringsFile is strings.ts: every text the written pages and the screens
// show, by key, in the target's language and in each translation, and the
// cookie that keeps the language a person chose.
func (g *gen) stringsFile(existing []genui.File) (string, bool) {
	all := map[string]string{}
	for k, v := range screenStrings {
		all[k] = v
	}
	for k, v := range g.strings {
		all[k] = v
	}
	codes, translated, ok := g.translations(all, existing)
	if !ok {
		return "", false
	}
	language := text(g.settings["language"])
	keys := sortedKeys(anyMap(all))
	var b strings.Builder
	b.WriteString("/**\n * Every text the screens show, by string key: in the specification's\n")
	fmt.Fprintf(&b, " * language, %s, and in each translation, read from its file beside this one.\n */\n", language)
	every := append([]string{language}, codes...)
	quoted := make([]string, len(every))
	for i, c := range every {
		quoted[i] = quote(c)
	}
	fmt.Fprintf(&b, "export const languages = [%s] as const;\n\n", strings.Join(quoted, ", "))
	b.WriteString("export type Language = (typeof languages)[number];\n\n")
	b.WriteString("/** The cookie that keeps the language a person chose, across sessions. */\n")
	fmt.Fprintf(&b, "export const languageCookie = %s;\n\n", quote(g.name("language-choice", "cookie")))
	b.WriteString("export const strings = {\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "  %s: %s,\n", quote(k), quote(all[k]))
	}
	b.WriteString("} as const;\n\nexport type StringKey = keyof typeof strings;\n\n")
	b.WriteString("/** The texts in each other language, an entry for every key. */\n")
	fmt.Fprintf(&b, "const translations: { readonly [L in Exclude<Language, %s>]: Readonly<Record<StringKey, string>> } = {", quote(language))
	if len(codes) == 0 {
		b.WriteString("};\n")
	} else {
		b.WriteString("\n")
		sort.Strings(codes)
		for _, c := range codes {
			fmt.Fprintf(&b, "  %s: {\n", key(c))
			for _, k := range keys {
				fmt.Fprintf(&b, "    %s: %s,\n", quote(k), quote(translated[c][k]))
			}
			b.WriteString("  },\n")
		}
		b.WriteString("};\n")
	}
	fmt.Fprintf(&b, stringsTail, quote(language))
	return b.String(), true
}

const stringsTail = `
/** The texts of a language. */
function textsOf(language: Language): Readonly<Record<StringKey, string>> {
  return language === %s ? strings : translations[language];
}

/** The text of a key in a language. */
export function stringOf(key: StringKey, language: Language): string {
  return textsOf(language)[key];
}

function collect(value: unknown, from: Readonly<Record<StringKey, string>>, into: Record<string, string>): void {
  if (typeof value === "string") {
    if (Object.hasOwn(strings, value)) {
      into[value] = from[value as StringKey];
    }
  } else if (Array.isArray(value)) {
    value.forEach((item) => collect(item, from, into));
  } else if (typeof value === "object" && value !== null) {
    Object.values(value).forEach((item) => collect(item, from, into));
  }
}

/** The texts a page's schema names, with the screens' own, in a language. */
export function texts(schema: unknown, language: Language): Readonly<Record<string, string>> {
  const from = textsOf(language);
  const into: Record<string, string> = {};
  for (const key of Object.keys(strings) as StringKey[]) {
    if (key.startsWith("screens.")) {
      into[key] = from[key];
    }
  }
  collect(schema, from, into);
  return into;
}
`

// languageFile is language.ts: the language a person chose, read on the
// server from its cookie, or the specification's.
func (g *gen) languageFile() string {
	return `import { cookies } from "next/headers";
import { languageCookie, languages, type Language } from "./strings";

/** The language the person chose, kept in its cookie across sessions, or the specification's. */
export async function chosenLanguage(): Promise<Language> {
  const chosen = (await cookies()).get(languageCookie)?.value;
  return languages.find((language) => language === chosen) ?? languages[0];
}
`
}
