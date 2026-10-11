import type { ListPageSchema } from "@/screens";

export const schema = {
  title: "directory.title",
  permission: "public",
  source: { operation: "listDirectory", path: "/directory" },
  columns: [{ field: "name", title: "directory.columns.name", sortable: false, compact: true }],
  filters: [],
  search: false,
  pageSize: 10,
  pageSizes: [10],
  actions: [],
  rowActions: [],
  empty: "directory.empty",
  filteredEmpty: "directory.filteredEmpty",
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
