// The texts the screens show: every string key in the chosen language,
// from strings.js beside the pages.

import { languages, strings } from "../strings.js";

export { languages };

/** The texts of a language: every key, in the specification's language when the language is not one strings.js holds. */
export function textsOf(language) {
  return strings[language] ?? strings[languages[0]];
}

/** The text of a key or a message { text, count, pattern }, its placeholders filled. */
export function say(texts, message) {
  if (typeof message === "string") {
    return texts[message] ?? message;
  }
  const text = texts[message.text] ?? message.text;
  return text.replace(/\{(count|pattern)\}/g, (_, name) => String(message[name] ?? ""));
}
