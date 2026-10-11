import { cookies } from "next/headers";
import { languageCookie, languages, type Language } from "./strings";

/** The language the person chose, kept in its cookie across sessions, or the specification's. */
export async function chosenLanguage(): Promise<Language> {
  const chosen = (await cookies()).get(languageCookie)?.value;
  return languages.find((language) => language === chosen) ?? languages[0];
}
