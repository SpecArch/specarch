# SBOM of specarch v1.4.0

- Tag: `v1.4.0`
- Commit scanned: `db7c0327726bec5bcb29478d1cd7a31ba74a5ead`
- Date: 2026-10-30 21:01 UTC
- Result: clean

SBOM: specarch-1.4.0.cdx.json (CycloneDX, 506 components). Scan (syft 1.55.0, grype 0.121.0, osv-scanner 2.7.0, govulncheck v1.8.0): clean.

## Files
- `specarch-1.4.0.cdx.json`: CycloneDX JSON, from syft
- `specarch-1.4.0.spdx.json`: SPDX JSON, from the same syft run
- `reports/grype.json`: grype sbom:specarch-1.4.0.cdx.json -o json
- `reports/osv-scanner.json`: osv-scanner scan source -r --allow-no-lockfiles --no-call-analysis=go --format json .
- `reports/osv-scanner.stderr.txt`: what osv-scanner wrote to stderr
- `reports/govulncheck-0.json`: govulncheck -format json ./... in .
- `reports/govulncheck-1.json`: govulncheck -format json ./... in examples/lending-desk/sources/code
- `tools.json`: the tools and their versions, each step's command and exit status
- `allowlist-applied.json`: the .dependency-allowlist.json entries that applied, with their reasons

## Left out of the scan

- `.build`
- `.worktrees`
- `references`
- `sbom` (the records of the earlier scans, #346)

## Run it again

`sbom-record v1.4.0` from the repository's root scans the tag's tree again and replaces this folder (the same day, a step whose inputs, tools and database are unchanged is taken from the scan cache; `AGENTQ_SCAN_CACHE=off` runs every step).
