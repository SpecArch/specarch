import type { ViewPageSchema } from "@/screens";

export const schema = {
  title: "member-view.title",
  permission: "members.read",
  source: {
    operation: "getMember",
    path: "/members/{memberId}",
    failed: [{ status: 404, problem: "member-gone", message: "member-view.failed.member-gone" }],
  },
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
      method: "POST",
      path: "/members/{memberId}/suspend",
      parameters: { memberId: "id" },
      permission: "members.write",
      when: ["==", ["field", "status"], ["value", "active"]],
      confirm: "member-view.actions.suspendMember.confirm",
      reason: { property: "why", label: "member-view.actions.suspendMember.reason", maxLength: 200 },
      message: "member-view.actions.suspendMember.message",
      failed: [{ status: 404, problem: "member-gone", message: "member-view.failed.member-gone" }],
    },
  ],
} satisfies ViewPageSchema;
