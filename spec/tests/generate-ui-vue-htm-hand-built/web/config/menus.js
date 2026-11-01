/** The application's menu: each group with its entries, each entry the page it opens by its route, shown to who holds the page's permission. */
export const menus = [
  {
    title: "menu.people.title",
    items: [
      { title: "menu.people.items.all", route: "/members", permission: "members.read" },
      { title: "menu.people.items.invite", route: "/invitations", permission: "public" },
      { title: "menu.people.items.removals", route: "/removals", permission: "members.approve" },
    ],
  },
];
