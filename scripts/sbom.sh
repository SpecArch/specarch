#!/bin/sh
# Writes the CycloneDX SBOM of a release and scans it, from the repository
# root:
#
#	sh scripts/sbom.sh <version> [<out-dir>]
#
# The SBOM covers what a release ships: the Go module (go.mod and go.sum,
# which go install reads) and the Swift package's resolved dependencies. It
# is read from the committed files of HEAD, never the working tree, so it
# names what the tag holds. It is written to
# <out-dir>/specarch-<version>.cdx.json (default: the current folder) and
# scanned with grype and osv-scanner; govulncheck checks the module's code.
# Any finding fails the script, naming the scanner: a release with a
# finding is not published until the owner has decided on it.
set -u

version=${1:-}
out=${2:-.}
case $version in
[0-9]*.[0-9]*.[0-9]*) ;;
*)
	echo "sbom: give the release version, such as 1.2.0; got '$version'" >&2
	exit 2
	;;
esac

if [ ! -f go.mod ] || [ ! -d swift ]; then
	echo "sbom: run it from the repository root, not $(pwd)" >&2
	exit 2
fi
for tool in syft grype osv-scanner govulncheck python3; do
	if ! command -v $tool >/dev/null 2>&1; then
		echo "sbom: $tool is not on PATH; install it and run again" >&2
		exit 2
	fi
done

tmp=$(mktemp -d) || exit 1
trap 'rm -rf "$tmp"' EXIT
tmp=$(cd "$tmp" && pwd -P) || exit 1
sbom=$out/specarch-$version.cdx.json

step() {
	name=$1
	shift
	echo "sbom: $name"
	if ! "$@"; then
		echo "sbom: FAILED at: $name" >&2
		exit 1
	fi
}

# The files the SBOM is read from, as HEAD holds them.
export_files() {
	git archive HEAD go.mod go.sum swift/Package.swift swift/Package.resolved | tar -x -C "$tmp"
}

# Only the Go module and Swift package catalogers run: the default set also
# reads the build machine's own application bundles. syft names each file it
# read by its absolute path; the SBOM names it by its path in the
# repository, so no path of the build machine is published.
write_sbom() {
	(cd "$tmp" && SYFT_GOLANG_SEARCH_LOCAL_MOD_CACHE_LICENSES=true syft scan dir:. \
		--source-name github.com/SpecArch/specarch --source-version "v$version" \
		--override-default-catalogers go-module-file-cataloger,swift-package-manager-cataloger \
		-o cyclonedx-json="$tmp/sbom.json" -q) || return 1
	python3 - "$tmp" "$tmp/sbom.json" "$sbom" <<'EOF'
import json, sys
root, src, dst = sys.argv[1], sys.argv[2], sys.argv[3]
with open(src) as f:
    text = f.read()
text = text.replace(root.rstrip("/") + "/", "")
if root in text or "/private/" in text or "/var/folders/" in text:
    sys.exit("sbom: the SBOM still names a path of the build machine; see " + src)
with open(dst, "w") as f:
    json.dump(json.loads(text), f, indent=2)
    f.write("\n")
EOF
}

licences() {
	python3 - "$sbom" <<'EOF'
import json, sys
d = json.load(open(sys.argv[1]))
for c in d.get("components", []):
    if c.get("type") != "library":
        continue
    names = sorted({l.get("license", {}).get("id") or l.get("license", {}).get("name") or l.get("expression") or "?" for l in c.get("licenses", [])})
    print(f"  {c['name']} {c.get('version', '')}: {', '.join(names) or 'not found in the module cache; check by hand'}")
EOF
}

step "export go.mod, go.sum and the Swift package files of HEAD" export_files
step "write $sbom with syft $(syft version 2>/dev/null | awk '/^Version:/ {print $2}')" write_sbom
step "scan with grype $(grype version 2>/dev/null | awk '/^Version:/ {print $2}')" grype --fail-on negligible "sbom:$sbom"
step "scan with $(osv-scanner --version 2>/dev/null | head -1)" osv-scanner scan source -L "$sbom"
step "check the module's code with govulncheck" govulncheck ./...
echo "sbom: licences of the components"
licences
echo "sbom: done; attach $sbom to the GitHub release of v$version"
