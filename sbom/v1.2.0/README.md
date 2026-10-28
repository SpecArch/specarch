# SBOM of specarch v1.2.0

- Tag: `v1.2.0`
- Commit scanned: `13a7741636f5c3a996cded5502e9af89aeef6161`
- Date: 2026-10-28 10:35 UTC
- Result: findings

SBOM: specarch-1.2.0.cdx.json (CycloneDX, 500 components). Scan (syft 1.55.0, grype 0.121.0, osv-scanner 2.7.0, govulncheck v1.8.0): 4 findings not allowlisted: CVE-2026-94482/GHSA-p3m5-59mp-jv5r in next 16.4.0 (examples/library-lending/web-nextjs/pnpm-lock.yaml, ./examples/library-lending/web-nextjs/pnpm-lock.yaml) (fixed in 16.4.1); CVE-2023-54404/GHSA-mrhx-c7xc-m3w5 in zod 4.6.5 (examples/library-lending/web-nextjs/pnpm-lock.yaml, ./examples/library-lending/web-nextjs/pnpm-lock.yaml) (fixed in 4.6.6); GHSA-2464-fhr4-w9xw in next 16.4.0 (examples/library-lending/web-nextjs/pnpm-lock.yaml, ./examples/library-lending/web-nextjs/pnpm-lock.yaml) (fixed in 16.4.1); GHSA-673q-mjm6-r5xr in next 16.4.0 (examples/library-lending/web-nextjs/pnpm-lock.yaml, ./examples/library-lending/web-nextjs/pnpm-lock.yaml) (fixed in 16.4.1).

## Files
- `specarch-1.2.0.cdx.json`: CycloneDX JSON, from syft
- `specarch-1.2.0.spdx.json`: SPDX JSON, from the same syft run
- `reports/grype.json`: grype sbom:specarch-1.2.0.cdx.json -o json
- `reports/osv-scanner.json`: osv-scanner scan source -r --allow-no-lockfiles --format json .
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

`sbom-record v1.2.0` from the repository's root scans the tag's tree again and replaces this folder.
