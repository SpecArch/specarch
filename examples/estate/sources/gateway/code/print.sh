#!/bin/sh
# print.sh prints the gateway table of what the gateway loads: the
# services of services.tsv, each with the address its setting holds in the
# environment named, from environments/<environment>.env, or with none
# when no environment is named. It is the printer
# tools/gateways/dump-gateway.sh runs (ADR-142).
#
#   print.sh [<environment>]
set -eu

environment=${1:-}
envfile=
if [ -n "$environment" ]; then
	envfile=environments/$environment.env
	if [ ! -f "$envfile" ]; then
		echo "print.sh: no $envfile" >&2
		exit 1
	fi
fi
awk -F '\t' -v environment="$environment" -v envfile="$envfile" '
function quoted(s) { gsub(/\\/, "\\\\", s); gsub(/"/, "\\\"", s); return "\"" s "\"" }
BEGIN {
	if (envfile != "") {
		while ((getline line < envfile) > 0) {
			if (line ~ /^#/ || line !~ /=/) continue
			k = substr(line, 1, index(line, "=") - 1)
			address[k] = substr(line, index(line, "=") + 1)
		}
	}
	print "{"
	print "    \"gateway\": \"api\","
	print "    \"claim\": \"aud\","
	print "    \"environment\": " (environment == "" ? "null" : quoted(environment)) ","
	print "    \"forwards\": ["
	n = 0
}
/^#/ || NF < 3 { next }
{
	url = ($3 in address) ? quoted(address[$3]) : "null"
	row[n++] = "        {\"value\": " quoted($1) ", \"name\": " quoted($2) ", \"setting\": " quoted($3) ", \"url\": " url "}"
}
END {
	for (i = 0; i < n; i++) print row[i] (i < n - 1 ? "," : "")
	print "    ]"
	print "}"
}
' services.tsv
