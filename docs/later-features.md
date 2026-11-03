# Later features

Features accepted for later. They are kept here so they are not lost while
the work goes to the features being built now. Nothing on this page is
built, and no release is promised for it. When one is started, its design
moves to its own file in `docs/`, and its entry here goes.

Each entry says what it is, what it reads, what it writes and what must be
true before it is started.

## 1. A Flutter screen generator

`specarch-gen-ui-flutter`: list, view, form and task pages in Dart and
Flutter, for a project whose screens are built there.

- **Reads**: the page analysis the SwiftUI and web generators share
  (`internal/genscreens`), so the pages are read once for every generator
  that writes a page as data. The analysis is not forked for Flutter: a
  page that reads differently in Flutter than in SwiftUI is a defect in
  the design, not a reason for a second reader.
- **Writes**: a Dart package the project writes the `pubspec.yaml` of,
  each page a Dart literal typed by its component's schema type, the
  navigation as one route table the app's navigator follows, the strings
  and the theme. The components are the idiom's `flutter` rendering, which
  a project overrides, found by the target's framework before the file's
  language, as the SwiftUI generator does (`docs/ui-swiftui.md`).
- **Same pages**: task pages such as sign-in, lists, forms and views; the
  role's home page and `navigate: callerHome`; sign-in through an OpenID
  Connect provider; calls through the same transport rules the other
  generators follow, under the names on the wire `wireNames` gives, with
  the handshake and the streams an operation declares.
- **Checked by**: a conformance case beside `spec/tests/generate-ui-swiftui`
  and a worked example beside `examples/library-lending/ios/`, and the test
  generator `specarch-gen-tests-dart` already in the tree for the Dart
  side.
- **Before it is started**: a project names Flutter as the target of its
  screens, and the owner decides the state-management package the pages
  use, since that choice is the one the generated code cannot leave open.

## 2. A Go generator for a framework other than dxlib

`specarch-gen-go-dxlib` writes the service for dxlib. A second Go target
would write the service for another framework, for a project that does not
use dxlib.

- **Reads**: the same design the dxlib target reads: operations, guards,
  permissions, rows rules, the handshake and stream declarations, jobs and
  the identity module. Nothing stack-specific enters the design for it; the
  framework's choices live in an implementation file
  (`implementation/go/<name>.go.specarch-implementation.yaml`).
- **Writes**: handlers, routing, the permission and rows checks and the
  hooks an operation's algorithm is wired to, in the framework's own
  conventions, with the algorithm bodies left hand-written as for dxlib.
- **Checked by**: the conformance cases of the dxlib target run against it,
  so the same specification generates a service that passes the same
  generated tests (`specarch-gen-tests-go`).
- **Before it is started**: a project names the framework. Until then no
  framework is chosen on a guess, since each one's routing, middleware and
  error model differ and the target's settings follow from them.
