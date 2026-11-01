export const page = {
  name: "removal-form",
  kind: "form",
  route: "/members/{memberId}/removal",
  permission: "members.write",
  schema: {
    title: "removal-form.title",
    permission: "members.write",
    submit: { operation: "requestRemoval", fields: ["why"] },
    layout: "page",
    sections: [{ fields: [{ type: "text", name: "why", label: "removal-form.fields.why", required: true, minLength: 1, maxLength: 200 }] }],
    rows: [],
    checks: [],
    failed: [],
    events: [{ status: 202, navigate: "members-list", message: "removal-form.onSubmitted" }],
  },
  routes: { "members-list": "/members" },
};
