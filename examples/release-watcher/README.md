# Release watcher

A small example of long-running processes (ADR-211): a daemon that asks a
registry for new releases of a host's services and installs them, and a
program a timer starts each night to prune old releases. Both run under
systemd.

- `sources/units/` holds the two service units and the timer as the host
  runs them.
- `spec/` is the specification written from them: the two processes
  under `processes`, the jobs they run under `jobs`, the settings the
  watcher reads, the monitor that watches it, and every test the
  processes imply, on the full profile.
- `docs/` holds the technical specification and the operations guide
  `specarch document` writes from `spec/`.

To see what a reader takes from the units alone, run, from this folder
in a checkout where they are committed,

    specarch extract systemd --out /tmp/release-watcher sources/units/release-watcher.service sources/units/release-prune.service sources/units/release-prune.timer

It writes a process per service with what each unit states, citing its
line, and asks for what the units leave to the host: the restart limit
when none is written, what each exit status means, the settings'
descriptions and types, the schedule a calendar expression gives, and
what the environment file holds. `spec/` is that tree with the questions
answered: the timer's `OnCalendar=*-*-* 03:00:00` on a host whose clock is
UTC became the job prune's schedule `0 3 * * *`, and the secret the
environment file holds became the setting `registryToken`.

The tests show what a process is checked for: its start, each signal it
honours, a setting missing or wrong, each exit status, a restart after a
crash and the crash loop it is given up after. None asks for a
permission or a caller: nobody calls a process.
