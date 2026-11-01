# A fourth UI generator: the web with no build step, on Vue and htm

Three generators write the screens of a specification: plain JavaScript
with no framework (`docs/ui-design.md`), TypeScript on Next.js and Carbon
(`docs/ui-nextjs-carbon.md`) and SwiftUI for the iPhone
(`docs/ui-swiftui.md`). A browser application in production was built
another way, and mirrored its specification by hand. This document is the
design of the generator that writes for that way, and what it settled.
In short:

1. A generator, `specarch-gen-ui-vue-htm`, for a `ui` target of platform
   web and framework `vue-htm` in an implementation file whose language
   is JavaScript. It writes, into the target's output folder, the
   application's menus, pages and actions as plain data files, one data
   file per page in one fixed shape, the shell and the components once,
   one `api.call`, the strings, the theme, a vendor manifest and the
   derived tests. Nothing is compiled: the files are loaded as written.
2. What draws each page is the `ui-components` idiom's rendering for
   `vue-htm`, the same parts and the same keys as `nextjs-carbon` and
   `swiftui`. The shipped rendering draws with HTML elements on Vue's
   runtime-only build and htm alone; the table the list page draws its
   rows through is named by the `list-page` part, so a project's own
   table replaces it by an override of that part.
3. The pages are read once, by `internal/genscreens`, which the TypeScript
   and SwiftUI generators read them through (ADR-147). The three accept
   and refuse the same design, and differ only in what they write.
4. The vendored libraries are the project's: each file is declared under
   `libraries.<name>.files` with its subresource integrity, `index.html`
   loads it with that integrity, and the generator checks the hash
   against the file in the output folder and never fetches anything.

The decisions are ADR-218; the requirement is SA-115; the cases are
`spec/tests/generate-ui-vue-htm` and the ones beside it.

## The stack as the project built it

The application had no Node.js, no package manager and no build step.
Its reasons hold for any project that wants the browser to run what is
in the repository, byte for byte:

- **ES2020 modules loaded as written.** A file in the repository is the
  file the browser runs; a review reads what ships. There is no bundle to
  diff, no source map, no transform between the commit and the browser.
- **Vue 3's runtime-only build.** The full build compiles templates in
  the browser with `new Function`, which a content security policy
  without `unsafe-eval` refuses. The runtime-only build has no compiler,
  so the components are written as render functions, and nothing is
  evaluated at run time.
- **htm for the markup.** A render function written as `h()` calls is
  hard to read; htm's tagged templates read as HTML and compile to `h()`
  calls by parsing the template string, with no `eval` and no compiler
  step, so the markup stays readable under the same policy.
- **A strict content security policy.** Neither `unsafe-eval` nor
  `unsafe-inline`: no inline script, no inline style, no event attribute
  in the document. The design declares the policy on its application
  route (ADR-216); the screens are what loads under it.
- **Every third-party library a pinned file committed beside the code**,
  loaded with its integrity, so a changed file is refused by the browser
  (W3C Subresource Integrity) and the review sees the bytes.
- **A component library's virtual table for the lists**, since the lists
  held thousands of rows.
- **The application's menus, pages and actions as plain JavaScript data
  files** that the pages read: a page named its operation and its
  permission, the menu named its pages, and a table of actions said how
  each operation was called. These mirrored the specification's pages,
  menus and operations, and every change was made twice.

The last point is what the generator removes: the data files, the pages
and the manifest follow the specification, and `generate --check` fails a
commit that changed one without the other.

## How the generator fits

**One implementation file per deliverable.** The browser application is
its own deliverable, so it has its own implementation file,
`<name>.javascript.specarch-implementation.yaml`, with `stack.language`
JavaScript and a `ui` target of platform web and framework `vue-htm`.
`specarch generate ui` finds `specarch-gen-ui-vue-htm` by the framework,
before a plug-in named for the language, and runs it once per output
folder. Its mappings carry `ownedBy` for what the application does not
build.

**The output is a folder a server hands out as it is.** The target's
`output` is that folder. The generator writes `index.html`, `app.js`,
`api.js`, `transport.js`, `strings.js`, `theme.css`, `config/`, `pages/`,
`screens/`, `tests/` and `vendor/manifest.js`, and refuses to write over a
file in the folder it did not write. The project writes what is its own:
the vendored files under `vendor/` (or any path `files` names), a custom
page's screen under `custom/`, a form's hooks under `hooks/`, and a table
of its own when it overrides the idiom. The generator never writes a
`package.json`, a lock file or a configuration, since the stack has none.

**The data files.** `config/menus.js` holds the menu: each group with its
title key and its entries, each entry the title key, route and permission
of the page it opens. `config/pages.js` holds the pages by name: kind,
title key, route with its `{parameters}`, permission and the data file.
`config/actions.js` holds every operation the pages call, by operationId:
its method and path in the standard dialect, the permission it needs,
and how it travels, the handshake whose envelope it needs, the stream and
its media type, and in the dxlib dialect each path parameter's name in
the body. `config/application.js` holds the title, where the service is
and its dialect with the handshakes, how the session is read (the
session's operation and the properties of its answer), the sign-in route,
each role's home, how a route is written in the address bar, and the
languages. These are the files the project kept by hand; they are data,
not loader functions, because data is what `generate --check` compares
and what a test imports under node.

**One data file per page, in one shape.** `pages/<page>.js` is
`export const page = { name, kind, route, permission, schema, routes }`
for every kind: task, list, form, view, content and custom. `schema` is
the page as the `ui-components` idiom names it for `vue-htm`, the same
keys the TypeScript schema has, but each call reduced to its operation's
name: the method, path, handshake, wire names and stream of a call move
to `config/actions.js`, so a call is said once and `api.call` reads it by
name. `routes` are the routes of the pages the page's events and actions
lead to. The file is compared by value (ADR-093), as a TypeScript schema
is: a project that wrote it by hand with its keys in another order, its
own quotes and a comment of its own, passes `--check` as long as the data
is the same (`generate-ui-vue-htm-hand-built`).

**One api.call.** `api.js` exports `api`, whose `call(action, payload)`
sends the action's operation as the service's dialect serves it, through
`transport.js`, the module the plain JavaScript screens call through
(ADR-164): in the dxlib dialect every call is a POST at `/<operationId>`
with its parameters in one body, the action-style pipeline the project
had, with dxlib's envelope sealed for an operation that names a handshake;
in the standard dialect the operation's method at its path. `upload`
streams a file, `save` keeps a download, `body` and `property` read an
answer, `pathOf`, `streamOf` and `mediaTypeOf` give what a page needs of
an action to fill a path from a record, save a download or accept a
file. The pages are the same in either dialect; the transport decides.

**The shell and the components.** `app.js` at the root imports the
pages' data once and mounts the shell from `screens/app.js`: the title,
the language choice, the menu, the notice an event left, and the page
the route opens, drawn by the component of its kind behind the guard.
The components under `screens/` are written once, on Vue's runtime build
and htm alone, through `screens/vue.js`, which takes the globals `Vue`
and `htm` that `index.html` loaded and binds `html` to `h`. The router
(`screens/router.js`) writes a route as a path under the design's
application route when it declares one, since the server answers the
document at every page route, and as a fragment after `#` otherwise, so
a folder served as it is still works; `index.html` names its files from
the application route through `base` in the first case. The session is
read once through `api.call`. A form's hooks and a custom page's screen
are modules the project writes, imported when their page opens, so a
module that is missing or fails is said on that page alone and the
application is never blank, which a build step would have caught and
this stack cannot.

**The table.** The `list-page` part names `table` and `tableModule`;
`screens/table.js` re-exports that component from that module as `Table`,
and the list page draws its rows through it. The shipped table,
`DataTable`, is a plain HTML table; the contract is written at the part:
props `columns` (each its field, its title as text, whether it sorts and
whether it stays on a compact screen), `rows`, `sort`, `rowKey`, `label`
and `actionsLabel`, the event `sort`, and the slots `cell` and `actions`.
A project wraps its component library's virtual table in a module that
honours the contract and overrides the part with its name and module
(`generate-ui-vue-htm-table-override`); nothing else changes, and the
generator never names a component library.

**The vendored files.** `libraries.<name>.files` in the implementation
file maps each file a library ships, by its path under the output folder,
to its subresource integrity, `sha256-`, `sha384-` or `sha512-` and the
digest in base64; `settings.vendor` names the libraries `index.html`
loads, in order, `vue` and `htm` among them, each with files. The files
live on the library because the library already carries the version and
the licence a manifest row needs. The generator refuses an integrity not
in that form or of another length, one that differs from the file in the
output folder, naming both hashes, a library `settings.vendor` names
without files, and `vue` or `htm` missing from it; a file not in the
output folder yet is a warning naming it. It never fetches anything: the
project pins what it ships. `vendor/manifest.js` lists every vendored
file with the library's name, version and licence, the file, its
integrity and whether `index.html` loads it, so a review and a CI step
read what ships from one place.

**Strings, theme, tests.** `strings.js` holds every text in the
specification's language and each translation `settings.translations`
names, read from `strings.<language>.json` in the output folder with the
same checks as the other generators; the chosen language is kept under a
key in the browser's local storage, read by `screens/language.js`, and
the document's `lang` follows it. `theme.css` is the plain JavaScript
generator's: the tokens as custom properties, the dark mode under
`prefers-color-scheme`, and the rules every screen shares;
`screens/screens.css` adds the screens' own rules by class, and no style
sits in the markup. Each list, form and view that needs a permission
gets `tests/<page>.test.js`, and the menu `tests/menu.test.js`, under
`node:test`, run on the data files and `screens/access.js` alone, so they
need no browser, no Vue and no htm.

**What has no form here.** `settings.server`, the TypeScript generator's
server routes, is refused: these screens are files, and the service
answers directly. A list the server pages whole (`settings.server.pages`)
is a server's, so a list page whose operation does not page is refused as
the TypeScript generator refuses it without that setting; the case owns
that page to another stakeholder. The plain JavaScript generator's event
bus has no place: a page's events are navigation through the router.

## The example

`examples/library-lending/web-vue/` is the library lending example's
screens on this stack, generated from
`spec/implementation/javascript/library-lending.javascript.specarch-implementation.yaml`:
Vue 3.5.43 (MIT) and htm 3.1.1 (Apache-2.0), committed under `vendor/`
with their licences beside them and their hashes in the implementation
file; the project's own files beside the generated ones are the loan
form's hook under `hooks/` and the Indonesian texts. CI runs the derived
tests with `node --test`, parses every generated file, checks each
vendored file against the manifest's integrity, and runs
`web-vue/smoke`, which mounts the generated application on the vendored
Vue and htm over a stand-in for the browser with the service stubbed and
opens each page kind: the list with its sort, a row action's confirmation
and reason, the view with a value's parts, a form that refuses an empty
required field and then sends, the loan form's hook, a task page, the
guard's refusal, a route with no page, and the language choice. The screens
the generator is for are a private project's, outside this repository;
nothing of it is here.

## Left out

Left out on purpose: the project's own identity provider, session
storage and server, which are its own; a component library, which the
project brings through the idiom; a mode other than dark, which the web
has no media query for, as in the plain JavaScript generator; and the
handshake and streams beyond what `transport.js` already carries. The
real screens were not copied: the generator is checked against the
example and the conformance cases.
