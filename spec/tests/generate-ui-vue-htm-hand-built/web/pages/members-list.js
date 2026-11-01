// The members list, as the team wrote it before the generator did.

export const page = {
    schema: {
        wire: {
            totalPages: 'totalPages',
            totalItems: 'totalItems',
            sort: 'sort',
            search: 'search',
            pageSize: 'pageSize',
            page: 'page',
            items: 'items'
        },
        title: 'members-list.title',
        source: {
            operation: 'listMembers'
        },
        select: {
            with: {
                memberId: 'id'
            },
            navigate: 'member-view'
        },
        search: true,
        rowActions: [{
            when: ['==', ['field', 'status'], ['value', 'active']],
            reason: {
                property: 'why',
                maxLength: 200,
                label: 'members-list.actions.suspendMember.reason'
            },
            permission: 'members.write',
            parameters: {
                memberId: 'id'
            },
            operation: 'suspendMember',
            message: 'members-list.actions.suspendMember.message',
            label: 'members-list.actions.suspendMember.label',
            failed: [{
                status: 404,
                problem: 'member-gone',
                message: 'members-list.failed.member-gone'
            }, {
                problem: 'default',
                message: 'members-list.failed.default'
            }],
            confirm: 'members-list.actions.suspendMember.confirm'
        }, {
            with: {
                memberId: 'id'
            },
            when: ['==', ['field', 'status'], ['value', 'active']],
            permission: 'members.write',
            navigate: 'member-edit',
            label: 'members-list.actions.member-edit.label'
        }],
        permission: 'members.read',
        pageSizes: [10, 20, 25, 50],
        pageSize: 25,
        filters: [{
            title: 'members-list.filters.status',
            query: 'status',
            options: ['active', 'inactive'],
            field: 'status'
        }, {
            title: 'members-list.filters.joinedOn',
            query: 'filter[joinedOn]',
            field: 'joinedOn',
            date: true
        }],
        filteredEmpty: 'members-list.filteredEmpty',
        failed: [{
            problem: 'default',
            message: 'members-list.failed.default'
        }],
        empty: 'members-list.empty',
        columns: [{
            title: 'members-list.columns.name',
            sortable: true,
            field: 'name',
            compact: true
        }, {
            title: 'members-list.columns.status',
            sortable: false,
            field: 'status',
            compact: false
        }, {
            title: 'members-list.columns.joinedOn',
            sortable: true,
            field: 'joinedOn',
            compact: false
        }],
        actions: [{
            permission: 'members.read',
            navigate: 'invitation',
            label: 'members-list.actions.invitation'
        }]
    },
    routes: {
        'member-view': '/members/{memberId}',
        'member-edit': '/members/{memberId}/edit',
        invitation: '/invitations'
    },
    route: '/members',
    permission: 'members.read',
    name: 'members-list',
    kind: 'list'
}
