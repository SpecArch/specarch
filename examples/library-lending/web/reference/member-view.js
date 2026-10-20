// The page member-view: Member. It shows a member read through getMember,
// opens loan-form and member-loans, and runs deactivateMember on it.

import {
  MEMBER_VIEW_LOADED, MEMBER_VIEW_FAILED,
  DEACTIVATE_MEMBER_SUCCEEDED, DEACTIVATE_MEMBER_FAILED,
} from "./events.js";
import { view } from "./view.js";

view({
  key: "id",
  source: { operation: "getMember", path: (parameters) => `/members/${encodeURIComponent(parameters.memberId)}` },
  sections: [
    {
      title: "Member",
      fields: [
        { name: "cardNumber", label: "Card number" },
        { name: "fullName", label: "Full name" },
        { name: "email", label: "Email address" },
        { name: "tier", label: "Tier" },
      ],
    },
    {
      title: "Membership",
      fields: [
        { name: "joinedOn", label: "Joined on" },
        { name: "membershipEndsOn", label: "Membership ends on" },
        { name: "outstandingFees", label: "Outstanding fees" },
      ],
    },
    {
      title: "Contact",
      fields: [
        {
          name: "address", label: "Address", parts: [
            { name: "city", label: "City" },
            { name: "postcode", label: "Postcode" },
            { name: "street", label: "Street" },
          ],
        },
        {
          name: "phones", label: "Phone numbers", list: true, parts: [
            { name: "label", label: "Label" },
            { name: "number", label: "Number" },
          ],
        },
      ],
    },
  ],
  actions: [
    {
      label: "Lend a book",
      page: "loan-form",
      with: () => ({}),
    },
    {
      label: "Lend copies",
      page: "member-loans",
      with: (record, parameters) => ({ memberId: parameters.memberId }),
    },
    {
      label: "Deactivate",
      operation: "deactivateMember",
      method: "POST",
      path: (record) => `/members/${encodeURIComponent(record.id)}/deactivate`,
      when: (record) => record.status === "active",
      confirm: "Deactivate this member? They will borrow nothing more.",
      reason: { name: "reason", label: "Reason", minLength: 1, maxLength: 500 },
      message: "The member is deactivated.",
      succeeded: DEACTIVATE_MEMBER_SUCCEEDED,
      failed: DEACTIVATE_MEMBER_FAILED,
    },
  ],
  messages: {
    failed: "This cannot be done right now. Try again in a moment.",
  },
  // What each operation's refusals say, by status: the page's failed states.
  refusals: {
    getMember: {
      "404": "There is no member with this card.",
    },
    deactivateMember: {
      "404": "There is no member with this card.",
    },
  },
  loaded: MEMBER_VIEW_LOADED,
  failed: MEMBER_VIEW_FAILED,
});
