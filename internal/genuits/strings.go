package genuits

import (
	"fmt"
	"sort"
	"strings"

	"github.com/SpecArch/specarch/internal/genui"
)

// stringsFile is strings.ts: every text the written pages and the screens
// show, by key, in the target's language and in each translation, and the
// cookie that keeps the language a person chose.
func (g *gen) stringsFile(existing []genui.File) (string, bool) {
	all := map[string]string{}
	for k, v := range screenStrings {
		all[k] = v
	}
	for k, v := range g.Strings {
		all[k] = v
	}
	codes, translated, ok := g.Translations(all, existing)
	if !ok {
		return "", false
	}
	language := text(g.Settings["language"])
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
	fmt.Fprintf(&b, "export const languageCookie = %s;\n\n", quote(g.Name("language-choice", "cookie")))
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
