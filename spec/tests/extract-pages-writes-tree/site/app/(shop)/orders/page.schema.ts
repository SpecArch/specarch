/* The orders list, as the screens library draws it. */
import type { PageSchema } from "@example/screens";
import { onRefresh } from "./page.hooks";

export const schema = {
  kind: 'list',
  title: "Orders",
  entity: "Order",
  permission: "orders.read",
  source: "listOrders",
  columns: ["number", "placedOn", "total",],
  compactColumns: ["number", "status"],
  filters: ["placedOn"],
  actions: [{ label: "Refresh", kind: "operation", target: "listOrders" }],
  refresh: onRefresh,
} satisfies PageSchema<"list">;
