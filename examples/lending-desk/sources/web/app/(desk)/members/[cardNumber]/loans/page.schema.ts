// The loans of one member, as the screens library draws a list.
import type { PageSchema } from "@acme/screens";

export const schema = {
  kind: "list",
  title: "Loans of a member",
  entity: "Loans",
  permission: "loans.read",
  source: "listMemberLoans",
  columns: ["barcode", "loanedOn", "dueOn", "returnedOn"],
  compactColumns: ["barcode", "dueOn"],
} satisfies PageSchema;
