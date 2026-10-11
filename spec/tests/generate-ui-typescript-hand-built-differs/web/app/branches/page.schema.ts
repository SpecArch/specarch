import type { ListPageSchema } from "@/screens";

export const schema = {
  title: "branch-list.title",
  permission: "members.read",
  source: { operation: "listAllBranches", path: "/branches/all" },
  columns: [{ field: "name", title: "branch-list.columns.name", sortable: false, compact: true }],
  filters: [],
  search: false,
  pageSize: defaultPageSize,
  pageSizes: [10, 20, 25, 50, 100],
  actions: [],
  rowActions: [],
  empty: "branch-list.empty",
  filteredEmpty: "branch-list.filteredEmpty",
  failed: [],
  wire: {
    page: "page",
    pageSize: "pageSize",
    sort: "sort",
    search: "search",
    items: "items",
    totalItems: "totalItems",
    totalPages: "totalPages",
  },
} satisfies ListPageSchema;
