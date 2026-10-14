import type { PageSchema } from "@desk/components";

// The members list, which the component library renders from this file.
export default {
  kind: "list",
  title: "Members",
  entity: "Member",
  permission: "members.read",
  columns: ["name", "email"],
} satisfies PageSchema;
