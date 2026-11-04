# Outstanding bugs

Open bugs, one entry per bug, each as `BUG-NNN — <title>` with what was
seen, what was expected and how to repeat it. The next free number is
BUG-074. When the fix is on `main`, with `Fixes BUG-NNN` in its commit
message, the entry moves to `BUG_HISTORY.md` with the fixing commit.

## BUG-072 — Open Dependabot alert for minimatch in the library lending example's web lock

- Reported: 2026-11-03 by the owner, from the repository's security tab (alert 100), against 1.5.0
- Waiting on: eslint-plugin-react, eslint-plugin-import and eslint-plugin-jsx-a11y taking minimatch 11

Dependabot alert 100 (medium, GHSA-g3hv-75wg-m249, denial of service through globparts amplification) is open for `minimatch` in `examples/library-lending/web-nextjs/pnpm-lock.yaml`. The advisory covers every version below 11.0.1, so both copies in the lock are in it: 10.2.6, held by `@typescript-eslint/typescript-estree`, and 3.1.5, held by `eslint`, `@eslint/eslintrc`, `@eslint/config-array`, `eslint-plugin-import`, `eslint-plugin-react` and `eslint-plugin-jsx-a11y`. Every one is a lint-time tool of the example; none is in the built screens.

Expected: no open alert.

To repeat: `gh api 'repos/SpecArch/specarch/dependabot/alerts?state=open'`.

Why it stays open: no released version of `eslint-plugin-react` (7.37.5), `eslint-plugin-import` (2.32.0) or `eslint-plugin-jsx-a11y` (6.10.2) takes minimatch 11; each asks for `^3.1.2`. minimatch 11 has no default export, and those plug-ins call the package itself as a function, so a pnpm override to 11.0.4 breaks any of their rules given a glob option: `react/no-danger` with `customComponentNames` failed with `TypeError: minimatch is not a function` (`eslint-plugin-react/lib/rules/no-danger.js:83`). The owner chose on 2026-11-04 to leave minimatch as it is until those three plug-ins take minimatch 11; then move the lock to it and close this entry.
