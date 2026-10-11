import type { FormPageSchema } from "@/screens";

export const schema = {
  title: "removal-form.title",
  permission: "members.write",
  submit: { operation: "requestRemoval", method: "POST", path: "/members/{memberId}/removals", fields: ["why"] },
  layout: "page",
  sections: [{ fields: [{ type: "text", name: "why", label: "removal-form.fields.why", required: true, minLength: 1, maxLength: 200 }] }],
  rows: [],
  checks: [],
  failed: [],
  events: [{ status: 202, navigate: "members-list", message: "removal-form.onSubmitted" }],
} satisfies FormPageSchema;
