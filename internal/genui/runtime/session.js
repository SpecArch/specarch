
// What the screens know of the person, read once per page: whether they
// are signed in, the permissions they hold and, when the session names
// them, their roles. The menu shows only the entries whose page opens to
// them; a field's hiddenUnless and readOnlyUnless read the permissions
// held (ADR-197).

/** The session, read once: null when it cannot be read. */
let read;
export function session() {
  read ??= call(SESSION.source, { parameters: {} })
    .then((response) => (response.ok ? response.json() : null))
    .then((answer) =>
      answer === null
        ? null
        : {
            signedIn: answer[SESSION.signedIn] === true,
            permissions: answer[SESSION.permissions] ?? [],
            roles: SESSION.roles ? answer[SESSION.roles] ?? [] : [],
          },
    )
    .catch(() => null);
  return read;
}

/** The permissions the person holds, signedIn among them once signed in;
    none while the session cannot be read. */
export async function held() {
  const person = await session();
  if (person === null) {
    return [];
  }
  return person.signedIn ? [...person.permissions, "signedIn"] : person.permissions;
}

/** Whether a page that needs a permission opens to the person. */
export function opens(permission, person) {
  if (permission === "public") {
    return true;
  }
  if (person === null || !person.signedIn) {
    return false;
  }
  return permission === "signedIn" || person.permissions.includes(permission);
}

/** The page of the person's home: that of the first of their roles, in
    the session's order, that has one. */
export function homeOf(person) {
  return (person?.roles ?? []).map((role) => HOMES[role]).find((page) => page !== undefined);
}

// The menu shows only the entries whose page opens to the person, and no
// empty group; it stays hidden until the session is read.
const menu = document.getElementById("menu");
if (menu) {
  menu.hidden = true;
  session().then((person) => {
    for (const entry of menu.querySelectorAll("[data-permission]")) {
      entry.hidden = !opens(entry.dataset.permission, person);
    }
    for (const group of menu.querySelectorAll(":scope > ul > li")) {
      group.hidden = [...group.querySelectorAll("[data-permission]")].every((entry) => entry.hidden);
    }
    menu.hidden = false;
  });
}
