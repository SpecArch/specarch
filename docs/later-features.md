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

## 3. A fresh environment for Kubernetes and for a VM

`generate deploy` writes Docker Compose files, Kubernetes manifests and
cloud-init for a VM, but only the Compose output can be rebuilt from
nothing for a full regression run: the fresh-environment mode refuses the
other two. This entry brings Kubernetes and VMs to the same level, so a
full regression can run on a clean copy of whichever target a deployment
names.

- **Reads**: the deployment's environment, its build, method and
  infrastructure, the seed the specification gives and the images by
  digest, as the Compose mode does.
- **Writes**: for Kubernetes, a script that creates a throwaway namespace
  or local cluster, applies the manifests, loads the seed, runs the full
  test package and removes it all; for a VM, the same around a new VM
  started from the cloud-init file on the named hypervisor.
- **Checked by**: the conformance cases of the Compose mode, repeated for
  each target, and one end-to-end run per target in CI.
- **Before it is started**: a project deploys to Kubernetes or to VMs and
  names the cluster or hypervisor the throwaway environment runs on.

## 4. An early warning when the schedule slips

`document progress` shows the size, the open work, the effort a change
records and a burn-up per release. Nothing says when the trend no longer
meets the date a release is planned for.

- **Reads**: the burn-up and effort `document progress` already computes,
  and a planned date per release.
- **Writes**: a warning in the progress document and in `problems` when
  the projected finish passes the planned date, naming the release, the
  gap in days and the open work that drives it.
- **Checked by**: a conformance case with a release on track and one that
  slips, beside the progress cases.
- **Before it is started**: a release can carry its planned date, and the
  owner decides how the projection is made (the recent rate of work closed,
  or the effort recorded against what is open).

## 5. Templates per project type

`specarch init` starts one shape of specification, with the lean profile
as the only option. A team that builds the same kinds of system again and
again would start from a template of that kind instead.

- **Reads**: a named template: the modules, the stages filled in, the
  roles, the environments and the decisions that kind of project always
  takes, with the questions it always has to ask.
- **Writes**: the starting specification, as `init` does now, from the
  template given with `init --template <name>`, and a list of the
  templates with `specarch help start`.
- **Checked by**: a conformance case per template that validates clean
  and asks its open questions in `gaps --next`.
- **Before it is started**: two or more finished projects of the same kind,
  so a template is taken from what they share rather than designed ahead of
  them.

## 6. Test results from spreadsheets and documents

`specarch log test-run` reads JUnit XML only. Results a customer reports in
a spreadsheet or a document have to be turned into JUnit XML by hand first.

- **Reads**: a results sheet in Excel, CSV or TSV through the reader
  `extract tables` uses, and a results table in a Word or PDF report
  through the `extract documents` readers; a mapping of columns to the
  test name, the outcome, the date and the remark.
- **Writes**: the same test-run records `log test-run` writes from JUnit
  XML, each result matched to its test or derived case, and a problem at
  every row that names no test.
- **Checked by**: conformance cases with a sheet and a document whose
  results equal a JUnit XML case's, so the three give the same records.
- **Before it is started**: a real results report, so the column mapping
  and the outcome words come from one and are not guessed.
