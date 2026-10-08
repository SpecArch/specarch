#!/bin/sh
# print.sh prints the grants the lending desk's permission check reads, as
# the JSON object tools/permissions/dump-permissions.sh takes. It starts a
# disposable PostgreSQL under Podman, applies the migrations and then the
# seed scripts, and runs lending/grants.sql, the query lending.Allowed
# runs. Run it from the folder above cmd.
#
# lending.Allowed runs on every request and reads no setting, so the
# object lists no gate.
#
# PODMAN names the command that runs podman, when it is not podman itself.
set -eu

image='docker.io/library/postgres:18.6-alpine@sha256:77f585114c32fbca283dc835b0596f4e52b51b4c6662d7810b2f4084f60a1873'
podman_cmd=${PODMAN:-podman}
name="lending-desk-permissions-$$"
cleanup() {
	$podman_cmd rm -f "$name" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

$podman_cmd run -d --rm --name "$name" \
	-e POSTGRES_PASSWORD=desk -e POSTGRES_USER=desk -e POSTGRES_DB=desk \
	"$image" >/dev/null
tries=0
until $podman_cmd exec "$name" pg_isready -q -U desk -d desk -h 127.0.0.1 2>/dev/null; do
	tries=$((tries + 1))
	if [ "$tries" -gt 60 ]; then
		echo "print.sh: the database did not start" >&2
		exit 1
	fi
	sleep 1
done

psql_in() {
	$podman_cmd exec -i "$name" psql -X -q -v ON_ERROR_STOP=1 -h 127.0.0.1 -U desk -d desk "$@"
}
for file in $(LC_ALL=C ls migrations/*.sql seeds/*.sql); do
	psql_in -f - <"$file" >/dev/null
done

grants=$(sed 's/;[[:space:]]*$//' lending/grants.sql)
psql_in -A -t <<SQL
SELECT '{' || chr(10) || '    "grants": [' || chr(10)
    || coalesce(string_agg('        ' || json_build_object('role', role, 'permission', permission)::jsonb::text, ',' || chr(10) ORDER BY role, permission), '')
    || chr(10) || '    ],' || chr(10) || '    "gates": []' || chr(10) || '}'
FROM ($grants) g;
SQL
