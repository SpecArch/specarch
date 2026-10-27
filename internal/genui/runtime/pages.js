
// The message a page leaves for the page it opens, kept for that one load.
const CARRIED = "specarch-message";

/** The address of a page, with the parameters its route takes. */
export function href(page, parameters) {
  const query = new URLSearchParams();
  for (const name of PAGES[page].parameters) {
    const value = parameters[name];
    if (value !== undefined && value !== null) {
      query.set(name, String(value));
    }
  }
  const given = String(query);
  return given === "" ? PAGES[page].file : `${PAGES[page].file}?${given}`;
}

/** Opens a page, which shows the message once it has opened. */
export function go(page, parameters, message) {
  if (message) {
    try {
      sessionStorage.setItem(CARRIED, message);
    } catch {
      // Storage is off in this browser: the page opens without the message.
    }
  }
  location.assign(href(page, parameters));
}

/** Opens the screens' start, index.html, which opens the person's home
    from the session read anew and shows the message there. */
export function start(message) {
  if (message) {
    try {
      sessionStorage.setItem(CARRIED, message);
    } catch {
      // Storage is off in this browser: the home opens without the message.
    }
  }
  location.assign("index.html");
}

/** The parameters of this page's address. */
export function parameters() {
  return Object.fromEntries(new URLSearchParams(location.search));
}

/** The message the page that opened this one left, once; "" when none. */
export function arrived() {
  try {
    const message = sessionStorage.getItem(CARRIED) ?? "";
    sessionStorage.removeItem(CARRIED);
    return message;
  } catch {
    return "";
  }
}
