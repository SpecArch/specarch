/**
 * Every text the screens show, by string key, in the specification's
 * language, en.
 */
export const strings = {
  "screens.email": "Type an email address, such as name@example.org.",
  "screens.failed": "This cannot be done right now. Try again in a moment.",
  "screens.maxLength": "Use at most {count} characters.",
  "screens.minLength": "Use at least {count} characters.",
  "screens.pattern": "This is not in the form asked for.",
  "screens.required": "Fill this in.",
  "screens.rule.pattern": "It matches {pattern}.",
  "second-factor.failed.second-factor-refused": "The code is wrong, or it came too late. Type the code the app shows now, or sign in again.",
  "second-factor.fields.code": "Code from the authenticator app",
  "second-factor.onSubmitted.200": "You are signed in.",
  "second-factor.title": "Confirm it is you",
  "sign-in.failed.sign-in-refused": "The email address or the password is wrong.",
  "sign-in.fields.email": "Email address",
  "sign-in.fields.password": "Password",
  "sign-in.onSubmitted.200": "You are signed in.",
  "sign-in.title": "Sign in",
} as const;

export type StringKey = keyof typeof strings;

function collect(value: unknown, into: Record<string, string>): void {
  if (typeof value === "string") {
    if (Object.hasOwn(strings, value)) {
      into[value] = strings[value as StringKey];
    }
  } else if (Array.isArray(value)) {
    value.forEach((item) => collect(item, into));
  } else if (typeof value === "object" && value !== null) {
    Object.values(value).forEach((item) => collect(item, into));
  }
}

/** The texts a page's schema names, with the screens' own. */
export function texts(schema: unknown): Readonly<Record<string, string>> {
  const into: Record<string, string> = {};
  for (const [key, text] of Object.entries(strings)) {
    if (key.startsWith("screens.")) {
      into[key] = text;
    }
  }
  collect(schema, into);
  return into;
}
