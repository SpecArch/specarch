export const page = {
  name: "member-edit",
  kind: "form",
  route: "/members/{memberId}/edit",
  permission: "members.write",
  schema: {
    title: "member-edit.title",
    permission: "members.write",
    source: { operation: "getMember", failed: [{ status: 404, problem: "member-gone", message: "member-edit.failed.member-gone" }] },
    submit: { operation: "updateMember", fields: ["name", "status"] },
    layout: "page",
    sections: [
      {
        fields: [
          { type: "date", name: "joinedOn", label: "member-edit.fields.joinedOn", required: false, readOnly: true },
          {
            type: "text",
            name: "name",
            label: "member-edit.fields.name",
            required: true,
            minLength: 1,
            maxLength: 100,
            readOnlyWhen: ["==", ["field", "status"], ["value", "inactive"]],
          },
          { type: "select", name: "status", label: "member-edit.fields.status", required: true, options: ["active", "inactive"] },
        ],
      },
    ],
    rows: [],
    checks: [],
    failed: [{ status: 404, problem: "member-gone", message: "member-edit.failed.member-gone" }],
    events: [{ navigate: "member-view", with: { memberId: "id" }, message: "member-edit.onSubmitted" }],
  },
  routes: { "member-view": "/members/{memberId}" },
};
