#!/bin/sh
# Writes the CycloneDX SBOM of a release and scans it, from the repository
# root:
#
#	sh scripts/sbom.sh <version> [<out-dir>]
#
# The SBOM covers what a release ships: the Go module (go.mod and go.sum,
# which go install reads) and the Swift package's resolved dependencies. It
# is read from the committed files of HEAD, never the working tree, so it
# names what the tag holds. It is scanned with grype and osv-scanner, and
# govulncheck checks the module's code.
#
# Everything it writes goes to <out-dir>, by default sbom/local/<version>/
# under the folder it is run from (created if missing):
#
#	specarch-<version>.cdx.json	the CycloneDX SBOM
#	grype.txt, osv-scanner.txt	each scanner's report on the SBOM
#	govulncheck.txt			govulncheck's report on the code
#	licences.txt			the licence of each component
#
# A run first removes those five files from <out-dir> (nothing else in it),
# so a run that fails leaves only the reports of the steps it ran. The
# release records in sbom/<tag>/ (sbom/v1.3.0 and the like) are written by
# the release step, not by this script: it refuses an <out-dir> inside one
# of them. sbom/local/ is gitignored; a local scan is not a record.
#
# Any finding fails the script, naming the scanner and its report: a
# release with a finding is not published until the owner has decided on
# it.
set -u

version=${1:-}
out=${2:-sbom/local/$version}
# Exactly three numbers joined by dots: 1.2.0, never 1.2, 1.2.3. or 1..2.3.
case $version in
*[!0-9.]* | .* | *. | *..* | *.*.*.*) bad=1 ;;
*.*.*) bad= ;;
*) bad=1 ;;
esac
case $bad in
1)
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

# The release records are refused by what <out-dir> resolves to, not by how
# it is spelled: sbom//v1.3.0, sbom/./v1.3.0, sbom/local/../v1.3.0, a
# symbolic link into one, and sbom/V1.3.0 or SBOM/v1.3.0 on a disk that
# ignores case are all refused. Each folder of the resolved path is compared
# with sbom/ by identity (-ef), and the name under it, lowercased, with
# v<digit>. The folders made only to find that out are taken away again
# (rmdir removes a folder only when empty).
here=$(pwd -P)
found=$out
while [ ! -d "$found" ]; do
	found=$(dirname "$found")
done
mkdir -p "$out" || exit 1
resolved=$(cd "$out" && pwd -P) || exit 1
dir=$resolved
while [ "$dir" != / ]; do
	parent=$(dirname "$dir")
	name=$(basename "$dir" | tr '[:upper:]' '[:lower:]')
	case $name in
	v[0-9]*)
		if [ "$parent" -ef "$here/sbom" ]; then
			made=$out
			while [ "$made" != "$found" ]; do
				rmdir "$made" 2>/dev/null
				made=$(dirname "$made")
			done
			echo "sbom: $out is inside a release record ($resolved); give another <out-dir>" >&2
			exit 2
		fi
		;;
	esac
	dir=$parent
done

# Only the files this script writes are removed, never the folder: a failed
# run then leaves only the reports of the steps it ran, not an earlier run's.
for name in specarch-$version.cdx.json grype.txt osv-scanner.txt govulncheck.txt licences.txt; do
	rm -f "$out/$name"
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

# Runs a scanner, prints its output and keeps it in <out-dir>/<report>,
# with this folder's path taken out so the report names repository paths
# only. The scanner's exit status is kept.
report() {
	file=$out/$1
	shift
	"$@" >"$tmp/report" 2>&1
	status=$?
	sed "s|$here/||g; s|$tmp/||g" "$tmp/report" >"$file"
	cat "$file"
	if [ $status -ne 0 ]; then
		echo "sbom: report in $file" >&2
	fi
	return $status
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
step "scan with grype $(grype version 2>/dev/null | awk '/^Version:/ {print $2}')" report grype.txt grype --fail-on negligible "sbom:$sbom"
step "scan with $(osv-scanner --version 2>/dev/null | head -1)" report osv-scanner.txt osv-scanner scan source -L "$sbom"
step "check the module's code with govulncheck" report govulncheck.txt govulncheck ./...
step "list the licences of the components" report licences.txt licences
echo "sbom: done; the SBOM and the reports are in $out/"
