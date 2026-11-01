export const page = {
  name: "visits-form",
  kind: "form",
  route: "/members/{memberId}/visits",
  permission: "members.write",
  schema: {
    title: "visits-form.title",
    permission: "members.write",
    source: { operation: "getMember", failed: [{ status: 404, problem: "member-gone", message: "visits-form.failed.member-gone" }] },
    submit: { operation: "recordVisits", fields: [] },
    layout: "page",
    sections: [
      {
        fields: [
          { type: "text", name: "name", label: "visits-form.fields.name", required: false, minLength: 1, maxLength: 100, readOnly: true },
        ],
      },
    ],
    rows: [
      {
        name: "visitLog",
        loaded: "visitLog",
        title: "visits-form.rows.visitLog",
        fields: [
          { type: "date", name: "visitedOn", label: "visits-form.rows.visitLog.fields.visitedOn", required: true },
          { type: "text", name: "note", label: "visits-form.rows.visitLog.fields.note", required: false, maxLength: 100 },
          { type: "text", name: "id", label: "visits-form.rows.visitLog.fields.id", required: false, readOnly: true },
        ],
        sent: ["visitedOn", "note"],
        maximum: 5,
        minimum: 1,
        lockLoaded: true,
      },
    ],
    checks: [],
    failed: [{ status: 404, problem: "member-gone", message: "visits-form.failed.member-gone" }],
    events: [{ navigate: "member-view", with: { memberId: "id" }, message: "visits-form.onSubmitted" }],
  },
  routes: { "member-view": "/members/{memberId}" },
};
