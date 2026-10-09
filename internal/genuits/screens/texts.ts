import type { Message } from "./schema";

/** The texts a page shows, by string key. */
export type Texts = Readonly<Record<string, string>>;

/** The text of a key or a message, its placeholders filled. */
export function say(texts: Texts, message: string | Message): string {
  if (typeof message === "string") {
    return texts[message] ?? message;
  }
  const text = texts[message.text] ?? message.text;
  return text.replace(/\{(count|pattern)\}/g, (_, name: "count" | "pattern") => String(message[name] ?? ""));
}
