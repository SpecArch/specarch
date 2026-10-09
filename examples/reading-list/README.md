# Reading list

An iPhone app on SwiftUI and SwiftData, and the Vapor server it calls,
kept to show how `specarch extract swift` reads Swift source through a
code-facts dump (ADR-087):

- `sources/app/` is the app: a `TabView` of a book list and the settings,
  a book's detail with its fields and a link to its reviews, a form in a
  sheet bound to a draft, SwiftData models, a `Codable` review whose
  `CodingKeys` give snake_case names, a client that calls the server with
  `URLSession`, and `Info.plist`, two `.xcconfig` files and the
  entitlements.
- `sources/server/` is the server: routes on the `Application`, a group
  guarded by its own `RequirePermission` check, a `RouteCollection`, a
  route registered in a loop, and a handler that reads a path parameter
  its route does not have.
- `sources/facts/app.json` and `sources/facts/server.json` are the dumps
  `tools/code-facts/dump-swift.sh` made of each folder, committed beside
  them, with the commit each was made at.
- `implementation/server-checks.yaml` names the server's check, so that
  the routes under the guarded group read their permission through it.

`extract.sh <out folder>` reads both dumps, merges the trees and validates
the result. The app gives its screens with their titles, fields and the
ways one opens another, its tabs as menu entries, its models, the server
as a dependency it calls, and its settings; the server gives its
operations, the permission its check names and the request bodies its
handlers decode. Everything the syntax does not say is a question at its
file and line: each screen's kind and route, the width of a Swift `Int`,
a SwiftData model's key, the server's routes as the running system
registers them.

The dumps go stale when a file under `sources/app` or `sources/server`
changes; run `tools/code-facts/dump-swift.sh examples/reading-list/sources/app
examples/reading-list/sources/facts/app.json` (and the same for the server)
after committing the change, then commit the dumps.
