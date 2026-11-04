# SBOM of specarch v1.5.0

- Tag: `v1.5.0`
- Commit scanned: `b28ec9dec86a8dcefb8c132f7a098bc19851822b`
- Date: 2026-11-04 03:45 UTC
- Result: findings

SBOM: specarch-1.5.0.cdx.json (CycloneDX, 507 components). Scan (syft 1.56.0, grype 0.121.1, osv-scanner 2.7.0, govulncheck v1.8.0): 2 findings not allowlisted: CVE-2026-118764/GHSA-g3hv-75wg-m249 in minimatch 10.2.6 (examples/library-lending/web-nextjs/pnpm-lock.yaml) (fixed in 11.0.1); CVE-2026-118764/GHSA-g3hv-75wg-m249 in minimatch 3.1.5 (examples/library-lending/web-nextjs/pnpm-lock.yaml) (fixed in 11.0.1).

## Files
- `specarch-1.5.0.cdx.json`: CycloneDX JSON, from syft
- `specarch-1.5.0.spdx.json`: SPDX JSON, from the same syft run
- `reports/grype.json`: grype sbom:specarch-1.5.0.cdx.json -o json
- `reports/osv-scanner.json`: osv-scanner scan source -r --allow-no-lockfiles --no-call-analysis=go --format json .
- `reports/osv-scanner.stderr.txt`: what osv-scanner wrote to stderr
- `reports/govulncheck-0.json`: govulncheck -format json ./... in .
- `reports/govulncheck-1.json`: govulncheck -format json ./... in examples/lending-desk/sources/code
- `tools.json`: the tools and their versions, each step's command and exit status
- `allowlist-applied.json`: the .dependency-allowlist.json entries that applied, with their reasons
- `osv-scanner.toml`: osv-scanner's ignores for this folder: the root's entries and the findings of v1.5.0 recorded here, so a whole-tree osv-scanner run does not count this release's findings as the current tree's

## Left out of the scan

- `.build`
- `.worktrees`
- `references`
- `sbom` (the records of the earlier scans)

## Run it again

`sbom-record v1.5.0` from the repository's root scans the tag's tree again and replaces this folder (the same day, a step whose inputs, tools and database are unchanged is taken from the scan cache; `AGENTQ_SCAN_CACHE=off` runs every step).
