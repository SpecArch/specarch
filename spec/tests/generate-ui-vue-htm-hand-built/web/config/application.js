/**
 * Where the screens call the service and in which dialect, how they read
 * the person's session, where they sign in, each role's home, how a page's
 * route is written in the address bar, and the languages.
 */
export const application = {
  title: "Members",
  service: "",
  dialect: "standard",
  session: { operation: "getSession", signedIn: "open", permissions: "granted" },
  signIn: "/sign-in",
  routing: { mode: "hash" },
  languages: ["en", "en-GB"],
};
