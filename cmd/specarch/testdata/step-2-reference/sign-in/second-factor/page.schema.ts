import type { TaskPageSchema } from "@/screens";

export const schema = {
  title: "second-factor.title",
  submit: { operation: "confirmSecondFactor", method: "POST", path: "/sessions/second-factor" },
  fields: [{ type: "text", name: "code", label: "second-factor.fields.code", required: true, maxLength: 6, pattern: "^[0-9]{6}$" }],
  checks: [],
  failed: [{ status: 401, problem: "second-factor-refused", message: "second-factor.failed.second-factor-refused", field: "code" }],
  events: [{ status: 200, navigate: "members-list", message: "second-factor.onSubmitted.200" }],
} satisfies TaskPageSchema;
