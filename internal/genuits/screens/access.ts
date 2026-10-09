/** What the screens know of the person: whether they are signed in, and the permissions they hold. */
export interface Session {
  readonly signedIn: boolean;
  readonly permissions: readonly string[];
}

/** Whether a page opens, sends the person to sign in first, or is refused. */
export type Decision = "allowed" | "sign-in" | "refused";

/** The decision for a page that needs a permission, the one the menu reads too. */
export function decide(permission: string, session: Session): Decision {
  if (permission === "public") {
    return "allowed";
  }
  if (!session.signedIn) {
    return "sign-in";
  }
  return session.permissions.includes(permission) ? "allowed" : "refused";
}

/** An entry of the menu, and the permission of the page it opens. */
export interface MenuEntry {
  readonly title: string;
  readonly route: string;
  readonly permission: string;
}

/** A group of the menu. */
export interface MenuGroup {
  readonly title: string;
  readonly items: readonly MenuEntry[];
}

/** The menu as a person sees it: only the entries whose page opens to them, and no empty group. */
export function visibleMenu(menu: readonly MenuGroup[], session: Session): MenuGroup[] {
  return menu
    .map((group) => ({ ...group, items: group.items.filter((item) => decide(item.permission, session) === "allowed") }))
    .filter((group) => group.items.length > 0);
}

/** How the screens read the session, the route that signs in, and the menu. */
export interface Application {
  readonly session: { readonly path: string; readonly signedIn: string; readonly permissions: string } | null;
  readonly signIn: string | null;
  readonly menu: readonly MenuGroup[];
}
