# A third UI generator: SwiftUI for the iPhone

The roadmap names SwiftUI the default for the iPhone, and the owner's own
apps are built there. This document is the design of the generator that
writes the same screens as the Next.js and Carbon one, for SwiftUI, and
the plan that builds it. In short:

1. A generator, `specarch-gen-ui-swiftui`, for a `ui` target of platform
   iphone and framework `swiftui` (the platform's default) in an
   implementation file whose language is Swift. It writes, into a Swift
   package the project writes the `Package.swift` of, per page a schema
   file that holds data only and a thin view, and per package the
   components, the routes, the application, the strings, the theme and
   the derived tests. The two web generators stay as they are.
2. What draws each page is the `ui-components` idiom's rendering for
   `swiftui`, the same parts and the same keys as `nextjs-carbon`. The
   shipped rendering draws with plain SwiftUI, so the example builds with
   no dependency; a project renders through its own components by
   overriding it, as any idiom is overridden today.
3. The pages are read once, by `internal/genscreens`, which the
   TypeScript generator reads them through too (ADR-147). The two
   generators accept and refuse the same design, and differ only in what
   they write.
4. The plug-in is found by the target's framework before the file's
   language: `specarch-gen-ui-swiftui`, then `specarch-gen-ui-swift`,
   then `specarch-gen-ui`.

The decisions were taken as the build made them (ADR-147), with the
review where the owner reads them; each step below says what it settled.

## Where SpecArch stands

`docs/ui-nextjs-carbon.md` built the design keywords a screen needs and
the generator that writes them for the web: task pages, lists, forms,
views, child rows, reasons, approvals, value objects, the menu and the
guard from one decision, strings in every language, the theme through
the `design-tokens` idiom, and the comparison with screens built by
hand. The plain JavaScript generator writes list pages. Nothing wrote
for the iPhone.

## How the generator fits

**One implementation file per deliverable.** The iPhone app is its own
deliverable, so it has its own implementation file,
`<name>.swift.specarch-implementation.yaml`, with `stack.language` Swift
and a `ui` target of platform iphone; `framework` may be left out, since
the platform's default is `swiftui`. `specarch generate ui` finds
`specarch-gen-ui-swiftui` for it and runs it once per output folder.
Its mappings carry `ownedBy` for what the app does not build.

**The output is a Swift package.** The target's `output` is the package
folder; the project writes its `Package.swift` (a library product of the
generated module, platforms iOS 17 and macOS 14 so the package builds
and tests on a Mac with no simulator) and the app around it (the `App`,
its `NavigationStack`, its tint, and where it sets the service's URL).
The generator writes under `Sources/<module>/` and
`Tests/<module>Tests/`, the module being `settings.module` or `Screens`.
It never writes `Package.swift`, never adds a dependency, and refuses to
write over a file in the package it did not write.

**Per page, two files.** `Pages/<Page>Schema.swift` holds the page as
data: one `let` of the part's schema type, whose labels are the names of
the `ui-components` idiom for `swiftui`, nested values written as `.init`
so the literal names no type a project's override would have to supply,
dictionaries (a route's parameters, the fields a picker fills) as Swift
dictionaries, and rules as `Rule` literals, the same operator-first
arrays the web evaluates. The compiler checks the literal against the
component's type, which is Swift's `satisfies`. `Pages/<Page>View.swift`
is the view: it reads the route's parameters, hands the schema and its
texts to the component behind the guard, and gives it the routes its
events lead to. A form whose fields take hooks reads them from
`<Page>Hooks`, a type the project writes beside the page, so a missing
hook fails the build, as a missing `page.hooks.ts` fails `tsc`.

**Navigation.** Pages are an enum, `Route`, one case per page written,
with the route's parameters as associated values, and `Routes.swift`
gives the view each case opens, which the app's `NavigationStack`
follows through `navigationDestination`. An event to a page is a push of
its case; the page sign-in was asked for from is kept by the router,
not a query, so there is nothing a person could forge and no open
redirect to check; a message crosses a navigation in the router and the
`Notice` shows it on arrival. The web's event bus and Next.js's
`returnTo` have no place here.

**Per package.** `Application.swift` holds where the screens call the
service (a base URL the app sets at launch), how the session is read
(`settings.session`, the same keys as on the web), the page that signs
in, and the menu, each entry shown only to someone who holds the
permission of the page it opens; the guard and the menu ask one
function, as on the web. `Strings.swift` holds every text the pages and
the components show, by key, in the target's language and in each
translation `settings.translations` names, each read from
`strings.<language>.json` in the output folder as the web reads them,
with the same checks; the chosen language is kept under a key in the
app's defaults. `Theme.swift` lays the theme's colours over SwiftUI's
through the `design-tokens` idiom: a `Color` per part, the light mode's
value or the dark mode's by the environment's colour scheme. The
components under `Screens/` are written once, on SwiftUI and Foundation
alone.

**Derived tests.** Each list, form and view that needs a permission gets
`Tests/<module>Tests/<Page>Tests.swift`, and the menu `MenuTests.swift`,
in Swift Testing, as `specarch-gen-tests-swift` writes: the page is
refused to someone signed in without its permission, sends someone
signed out to sign in, and opens to someone who holds it; the menu shows
no entry without its page's permission.

**What has no SwiftUI form.** `settings.server`, the server routes that
forward the browser's calls, is refused for `swiftui`: an app calls the
service directly. The comparison of a schema built by hand as data
(ADR-093) reads the subset of TypeScript that is JSON5 and is
TypeScript's; a Swift file is compared byte for byte, and one built by
hand is compared without the header, as every generated file is.

**The reference screens.** As ADR-040 and ADR-067 did, the example's
sign-in and second-factor screens are written by hand in the Swift
package first, in a commit of their own, and the generator reproduces
them but for the header. The real screens the generator is for are the
owner's own apps, private; each repeats the check against its own screen
through its override, outside this repository.

## Building the generator

Each step is a commit of its own: the specification first, the
conformance cases, `docs/`, the history and both validator builds
current (the validator itself does not change; the framework and the
idiom's stacks are strings it reads already).

1. **The analysis shared, and the plug-in found by its framework.**
   `internal/genscreens` reads the pages for both generators, with the
   TypeScript goldens unchanged; `specarch generate` tries
   `specarch-gen-<target>-<framework>` first (`generate-framework-plugin`).
2. **The design.** ADR-147, this document, SA-78, the `swiftui`
   renderings of `ui-components` and `design-tokens`, and the idioms
   listing for a Swift file whose `ui` target is platform iphone
   (`idioms-lists-ui-framework`).
3. **The reference screens by hand.** `examples/library-lending/ios/`, a
   Swift package with its `Package.swift`, the app, and the sign-in and
   second-factor screens written by hand on plain SwiftUI; CI builds
   and tests it on the macOS job.
4. **Task pages.** `specarch-gen-ui-swiftui` writes the task pages, the
   components, the routes, the application and the strings; generated
   over the reference, the two screens differ only by the header
   (`generate-ui-swiftui`).
5. **Lists, forms and views.** With the guard, the menu, the derived
   tests, child rows, reasons, approvals, lookups and value objects, the
   same refusals as the web (`generate-ui-swiftui-refused`).
6. **Translations and the theme.** `Strings.swift` in every language
   and `Theme.swift`; the roadmap, the generated documents and the
   history.

## Left out

The owner's iPhone apps, whose screens the generator is for, are not in
this repository; the example's screens are the reference. Left out on
purpose: an app's own session storage (a keychain, a token kept across
launches), which is the app's; the handshake in the screens, a roadmap
item for every UI target; and Mac and iPad layouts beyond what SwiftUI
gives the same views.
