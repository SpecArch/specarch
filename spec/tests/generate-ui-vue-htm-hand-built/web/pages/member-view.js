export const page = {
  name: "member-view",
  kind: "view",
  route: "/members/{memberId}",
  permission: "members.read",
  schema: {
    title: "member-view.title",
    permission: "members.read",
    source: { operation: "getMember", failed: [{ status: 404, problem: "member-gone", message: "member-view.failed.member-gone" }] },
    sections: [
      {
        fields: [
          { name: "name", label: "member-view.fields.name" },
          { name: "joinedOn", label: "member-view.fields.joinedOn", hiddenWhen: ["==", ["field", "status"], ["value", "inactive"]] },
        ],
      },
    ],
    actions: [{ label: "member-view.actions.member-edit", navigate: "member-edit", permission: "members.write" }],
    operations: [
      {
        label: "member-view.actions.suspendMember.label",
        operation: "suspendMember",
        parameters: { memberId: "id" },
        permission: "members.write",
        when: ["==", ["field", "status"], ["value", "active"]],
        confirm: "member-view.actions.suspendMember.confirm",
        reason: { property: "why", label: "member-view.actions.suspendMember.reason", maxLength: 200 },
        message: "member-view.actions.suspendMember.message",
        failed: [{ status: 404, problem: "member-gone", message: "member-view.failed.member-gone" }],
      },
    ],
  },
  routes: { "member-edit": "/members/{memberId}/edit" },
};
