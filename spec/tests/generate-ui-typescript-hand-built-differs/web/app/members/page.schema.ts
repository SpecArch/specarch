import type {ListPageSchema} from '@/screens';

// The members list, as the team wrote it.

export const schema = {
  "title": 'members-list.title',
  permission: "members.read",
  source: { operation: "listMembers", path: "/members" },
  columns: [
    { field: "name", title: "members-list.columns.name", sortable: true, compact: true },
    { field: "status", title: "members-list.columns.status", sortable: true, compact: false },
    { field: 'joinedOn', title: 'members-list.columns.joinedOn', sortable: true, compact: false }
  ],
  filters: [
    { field: "status", query: "status", title: "members-list.filters.status", options: ["active", "inactive"] },
    { field: "joinedOn", query: "filter[joinedOn]", title: "members-list.filters.joinedOn", date: true },
  ],
  search: true,
  pageSize: 25,
  pageSizes: [10, 20, 25.0, 50], /* the sizes offered */
  actions: [{ label: "members-list.actions.invitation", navigate: "invitation", permission: "members.read" }],
  rowActions: [
    {
      label: "members-list.actions.suspendMember.label",
      operation: "suspendMember",
      method: "POST",
      path: "/members/{memberId}/suspend",
      parameters: { memberId: "id" },
      permission: "members.write",
      when: ["==", ["field", "status"], ["value", "active"]],
      confirm: "members-list.actions.suspendMember.confirm",
      reason: { property: "why", label: "members-list.actions.suspendMember.reason", maxLength: 200 },
      message: "members-list.actions.suspendMember.message",
      failed: [
        { status: 404, problem: "member-gone", message: "members-list.failed.member-gone" },
        { problem: "default", message: "members-list.failed.default" },
      ],
    },
    {
      label: "members-list.actions.member-edit.label",
      navigate: "member-edit",
      with: { memberId: "id" },
      permission: "members.write",
      when: ["==", ["field", "status"], ["value", "active"]],
    },
  ],
  select: { navigate: "member-view", with: { memberId: "id" } },
  empty: "members-list.empty",
  filteredEmpty: "members-list.filteredEmpty",
  failed: [{ problem: "default", message: "members-list.failed.default" }],
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
