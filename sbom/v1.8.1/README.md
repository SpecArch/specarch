# SBOM of specarch v1.8.1

- Tag: `v1.8.1`
- Commit scanned: `4d6182879c365d15e78589c559f789356d759568`
- Date: 2026-11-24 21:13 UTC
- Result: findings

SBOM: specarch-1.8.1.cdx.json (CycloneDX, 502 components). Scan (syft 1.57.1, grype 0.121.3, osv-scanner 2.7.1, govulncheck v1.9.0): 7 findings not allowlisted: CVE-2026-119348/GHSA-pc58-hwjq-p385 in next 16.4.1 (examples/library-lending/web-nextjs/pnpm-lock.yaml) (fixed in 16.4.5); CVE-2026-119349/GHSA-rhh4-g77r-r9qj in next 16.4.1 (examples/library-lending/web-nextjs/pnpm-lock.yaml) (fixed in 16.4.5); CVE-2026-119790/GHSA-mfr8-8qww-v76h in next 16.4.1 (examples/library-lending/web-nextjs/pnpm-lock.yaml) (fixed in 16.4.5); CVE-2026-119347/GHSA-7qx7-33qx-rqx2 in next 16.4.1 (examples/library-lending/web-nextjs/pnpm-lock.yaml) (fixed in 16.4.5); CVE-2026-119346/GHSA-f3mh-rxg3-f5g2 in next 16.4.1 (examples/library-lending/web-nextjs/pnpm-lock.yaml) (fixed in 16.4.5); CVE-2026-119351/GHSA-vc7v-qv4w-wrv8 in next 16.4.1 (examples/library-lending/web-nextjs/pnpm-lock.yaml) (fixed in 16.4.5); CVE-2026-119350/GHSA-vvgp-ww4w-46vf in next 16.4.1 (examples/library-lending/web-nextjs/pnpm-lock.yaml) (fixed in 16.4.5). Accepted: CVE-2026-118764/GHSA-g3hv-75wg-m249 in minimatch 10.2.6 (examples/library-lending/web-nextjs/pnpm-lock.yaml) (.dependency-allowlist.json).

## Files
- `specarch-1.8.1.cdx.json`: CycloneDX JSON, from syft
- `specarch-1.8.1.spdx.json`: SPDX JSON, from the same syft run
- `reports/grype.json`: grype sbom:specarch-1.8.1.cdx.json -o json
- `reports/osv-scanner.json`: osv-scanner scan source -r --allow-no-lockfiles --no-call-analysis=go --format json .
- `reports/osv-scanner.stderr.txt`: what osv-scanner wrote to stderr
- `reports/govulncheck-0.json`: govulncheck -format json ./... in .
- `reports/govulncheck-1.json`: govulncheck -format json ./... in examples/lending-desk/sources/code
- `tools.json`: the tools and their versions, each step's command and exit status
- `allowlist-applied.json`: the .dependency-allowlist.json entries that applied, with their reasons
- `osv-scanner.toml`: osv-scanner's ignores for this folder: the root's entries and the findings of v1.8.1 recorded here, so a whole-tree osv-scanner run does not count this release's findings as the current tree's

## Left out of the scan

- `.build`
- `.worktrees`
- `references`
- `sbom` (the records of the earlier scans)

## Run it again

`sbom-record v1.8.1` from the repository's root scans the tag's tree again and replaces this folder (the same day, a step whose inputs, tools and database are unchanged is taken from the scan cache; `AGENTQ_SCAN_CACHE=off` runs every step).
