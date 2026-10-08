import type { PageSchema } from "@desk/components";

// The sign-in page submits to the router's signIn and loads no record.
export default {
  kind: "task",
  title: "Sign in",
  entity: "Member",
  permission: "public",
  submit: "signIn",
  fields: ["email", "password"],
} satisfies PageSchema;
