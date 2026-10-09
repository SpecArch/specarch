import type { FieldHook } from "@/screens";

/** A copy is lent today, by the desk's own calendar, unless the desk records an earlier day. */
export const lentOn: FieldHook = () => {
  const today = new Date();
  const month = String(today.getMonth() + 1).padStart(2, "0");
  const day = String(today.getDate()).padStart(2, "0");
  return `${today.getFullYear()}-${month}-${day}`;
};
