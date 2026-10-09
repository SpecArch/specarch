# Room booking

A small Express service that books meeting rooms, written twice: once in
TypeScript and once in plain JavaScript, line for line the same service.
It is kept to show how `specarch extract javascript` reads both through a
code-facts dump (ADR-089), and that the two readings differ only where
plain JavaScript gives no types:

- `sources/ts/` is the TypeScript service: a router mounted under
  `/api/bookings` whose routes are guarded by its own `requirePermission`
  check, a second router under `/api/rooms` that checks nothing, a zod
  schema the create handler applies to its body, the interfaces its
  handlers declare with Express's `Request` and `Response`, a union of
  strings as a booking's status, a call to a calendar service with
  `fetch`, and the settings it reads from `process.env`.
- `sources/js/` is the same service in JavaScript, as ES modules, with no
  `jsconfig.json`: the zod schema is the same, the declared types are
  gone, one handler states its types in a JSDoc comment the compiler does
  not check, and a JSDoc `@typedef` describes a booking.
- `sources/facts/ts.json` and `sources/facts/js.json` are the dumps
  `tools/code-facts/dump-javascript.sh` made of each folder, committed
  beside them, with the commit each was made at.
- `implementation/room-booking-checks.yaml` names the check, so that the
  routes read their permission through it.
- `differences.txt` is what `extract.sh` finds when it compares the
  questions of the two readings, with line numbers and the file
  extensions left out.

`extract.sh <out folder>` reads both dumps into a tree each, validates
them and writes the differences of their questions. Both give the same
seven operations, the same permissions, the `NewBooking` schema of the
zod schema, the calendar as a dependency and the same settings. The
TypeScript reading also gives the `Booking`, `Room` and `CancelRequest`
schemas and the `BookingStatus` enum its handlers declare, the cancel
route's body by its declared type, and asks the width of each number
those types hold. The JavaScript reading asks instead: the types of the
fields the cancel handler reads from its body, and whether the JSDoc
comment no compiler checks says what the handler takes and answers.

The dumps go stale when a file under `sources/ts` or `sources/js` changes;
run `tools/code-facts/dump-javascript.sh examples/room-booking/sources/ts
examples/room-booking/sources/facts/ts.json` (and the same for `js`) after
committing the change, then commit the dumps.
