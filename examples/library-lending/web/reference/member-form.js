// The page member-form: Register member. It sends its fields to
// createMember and opens member-view once it succeeds.

import { MEMBER_FORM_SUBMITTED, MEMBER_FORM_REFUSED } from "./events.js";
import { form } from "./form.js";

form({
  submit: {
    operation: "createMember",
    method: "POST",
    path: () => "/members",
    sent: ["fullName", "email", "tier", "address", "phones"],
  },
  checks: [],
  conditions: {},
  pickers: {},
  next: {
    success: { page: "member-view", with: { memberId: "id" }, message: "The member is registered." },
  },
  messages: {
    failed: "This cannot be done right now. Try again in a moment.",
    pattern: "This is not in the form it takes.",
    twice: "The two entries are not the same.",
  },
  // What the operations' refusals say, by status, and the field each is about.
  refusals: {
    submit: {
      "409": { message: "Another member already has this email address.", field: "email" },
    },
  },
  submitted: MEMBER_FORM_SUBMITTED,
  refused: MEMBER_FORM_REFUSED,
});
