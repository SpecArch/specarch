import type { Application } from "@/screens";

/**
 * Where the screens call the service, how they read the person's session,
 * where they sign in, and the menu.
 */
export const application: Application = {
  service: "/api",
  session: { path: "/sessions/current", signedIn: "open", permissions: "granted" },
  signIn: "/sign-in",
  menu: [
    {
      title: "menu.people.title",
      items: [
        { title: "menu.people.items.all", route: "/members", permission: "members.read" },
        { title: "menu.people.items.invite", route: "/invitations", permission: "public" },
        { title: "menu.people.items.removals", route: "/removals", permission: "members.approve" },
      ],
    },
  ],
};
