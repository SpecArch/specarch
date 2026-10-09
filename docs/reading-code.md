# Reading source code

What `specarch extract` reads from a system's source code, per language and
per framework, what it leaves to a printer the running system writes, and
how a tree read from source merges with the trees of the other readers.
The rules every such reader shares are ADR-075; the dxlib services are
ADR-076. The build steps are in `docs/extraction.md`, Building extract,
from step 12.

## Summary

- A surface the running system can print stays with its printer: the
  routes it registers, the grants its check reads, the tables after every
  migration. Source is read for what nothing prints: where a thing is
  declared, what a handler reads and answers, screens and how one leads
  to the next, client calls, settings read, models declared in code.
- Each language is read by its own parser: Go by `go/parser` inside
  `specarch`; Swift by SwiftSyntax, JavaScript and TypeScript by the
  TypeScript compiler, Vue by `@vue/compiler-sfc`, Dart by the analyzer
  package, each in a small program under `readers/` that writes a
  committed code-facts dump.
- A reader writes a fact only where the syntax says it as a literal or
  through an idiom of a library it knows. Anything else on a surface it
  reads is a question at that file and line, never a value.
- Every element cites `path:line` at the commit read. A parsed tree and a
  printed tree of one surface merge into one element with two citations;
  where they differ, or only one has it, a question says so.
- Order: dxlib services' emitted document, then Go source (dxlib first,
  then `net/http` and the common routers), then the file-system routers
  that need no parser (Next.js `pages/` and route files, Nuxt `pages/` and
  `server/`), then Swift, then JavaScript and TypeScript with the content
  of Next.js files, then Vue and Nuxt, then Dart.

## The shared rules

These are ADR-075, restated for a reader of this document.

**Printed first, parsed beside it.** ADR-044 and ADR-050 chose a printer
over a reader of Go for routes and grants, because a parser sees one way
of writing a registration and misses the loop, the library and the
setting. That holds. A parsed reader adds the citations and the facts a
printer cannot give, and where a project has no printer yet its routes and
grants are written with a must question each, asking whether the running
system has them.

**The language's own parser.** It accepts what the compiler accepts and
nothing else. tree-sitter was weighed: its grammars follow each language
at their own pace, its Go bindings need cgo, which ends the single static
binary and its cross builds, and it gives no types. A program under
`readers/<language>/` runs the parser in that language's toolchain, which a
project in that language already has.

**The code-facts dump.** The program writes a JSON object in a format
SpecArch defines: `codeFacts: 1`, the language, the parser's name and
version, the folder read, the commit that last changed it, and every fact
as its kind, file, line, column and literal values, sorted by file, line
and column. `tools/code-facts/dump-<language>.sh` runs it in a committed
folder, refuses changes not committed, and adds the version, the path and
the commit. The dump is committed beside the code, is stale once that
folder changes, and is refused when another parser version made it than
the release pins. `specarch extract` turns facts into elements, so the
rules of extract live in one place, in Go. Go itself needs no dump: the
standard library's parser runs inside `specarch`.

**Tracked files only.** A module cache, `node_modules`, a pub cache or a
Swift package checkout is in no commit, so it is not read. A type, a
constant or a function declared there is not known, and what it would give
is a question.

**Literal or known idiom, else a question.** Each reader has a table of
the idioms of the libraries it knows: the calls that register a route, a
page, a permission check, a model. A fact is written when the syntax gives
it as a literal through one of them. On a surface it reads, anything else
is a question at that file and line: a path, a method, a permission or a
destination computed; a registration in a loop or behind a condition; a
call through a helper the table does not know; a spread; a dynamic import.
It is `must` when it concerns a route, a permission, a role, a gate on a
setting, an entity's key or a personal or credential field, and `should`
otherwise. A file that imports a library the reader knows and produces
nothing on a surface it reads prints a line, so it shows in the coverage.

**Citations.** Every element is `origin: stated` and cites `path:line`
under the code source, whose outline lists the files read; a citation's
clause falls under the file it starts with, so `specarch gaps` still
counts per file, and the problems file places each note on its line.

**How it was read.** A code source carries `reading: printed` for a dump
the running system made and `reading: parsed` for a parsed reader.

## How a parsed tree merges

`specarch merge` already joins an element two trees give into one element
with both citations, and leaves a key they give different values out with
a `must` question citing both. Between a printed and a parsed tree of one
surface it adds:

| The printed tree | The parsed tree | What is written |
|---|---|---|
| has it | has it | one element, both citations; a key they disagree on left out with a `must` question |
| does not have it | has it | a `must` question: declared at `path:line`, not in the running system; the code is never reached, or only under a setting |
| has it | does not have it | a `could` question citing the dump: registered by code the reader does not follow |
| answers the parsed tree's question whether the running system has it | asks it | the question left out |

A parsed reader asks whether the running system has an element as a
`must` question that blocks that element alone, which is how the merge
knows it. The rows compare operations, entities and roles. A role both
trees give grants what the printed tree grants, since that is what the
check reads, and a grant only one side makes takes the row it falls
in:

| The permission table | The seeds | What is written |
|---|---|---|
| grants it | grant it | the grant, with both citations |
| does not grant it | grant it | a `must` question blocking the role's permissions and the permission: the seed did not run, or something removed the grant |
| grants it | do not grant it | a `could` question: granted by a seed the reader does not follow, or by hand |

A gate on a setting is asked by every reader in the same words, so the
merge joins a printed and a parsed tree's gate on one check into one
question with both citations. The other surfaces join these rows in
their readers' steps. A source that one tree read printed and another
parsed is merged with no `reading`, since it was read both ways.

Against a documents tree a parsed tree is the code side, as every code
tree is (`docs/from-sources.md`, 3.2).

Per reader, what it meets in the trees that exist:

- **Routes** from Go, Express, Fastify, Vapor, Nitro, `shelf_router` or
  Next.js route files meet the route table, dxlib's document and an
  OpenAPI document by method and path; a parsed route adds `path:line` and
  the handler, and a permission read from a known check is compared.
- **Grants** from seed calls meet the permission table by role and
  permission; a seed in code that the running tables do not hold is the
  `must` question of the table above, since the seed did not run or
  something removed it.
- **Gates on a setting** (a check that lets everything through while a
  setting is empty) found in source meet the gates the permission table
  declares by the check's name: one question, two citations. A gate found
  in source and not declared is a `must` question saying the printer does
  not declare it; one declared and not found is a `could` question.
- **Models** declared in code meet the catalogue by table and column where
  the code names its table; the catalogue is the source of the data model,
  and the struct adds its wire names and the validation it declares. A
  struct that names no table is a schema, not an entity.
- **Screens** from SwiftUI, Flutter, Vue or React meet the pages reader by
  route where they have one, and a client call's method and path joins a
  page's `source` or `submit` to an operation, as a page schema's names do
  now (ADR-057): the question carries the method and path under `names`,
  and merge writes the operation once a tree declares it. A call to a path
  no tree declares stays a `must` question; one to another system names a
  dependency.
- **Settings** read in code meet the configuration files read as data by
  name; the file gives the value and the code says it is read.

## Go

Read inside `specarch` with `go/parser`, `go/ast` and `go/token` from the
standard library, with no dependency and nothing to scan. Syntax only:
`go/types` needs the module cache and `go list`, so a name declared outside
the module's tracked files is not known. Names declared inside the module
are resolved through `go.mod`'s module path and the import paths, by syntax.

### Go on dxlib

dxlib-based services come first among the Go readers (ADR-076).

| Surface | Source | Why |
|---|---|---|
| endpoints: method, URI, parameters and their dxlib types, content type, responses, rate-limit group, content-length ceiling | the OpenAPI document dxlib emits (`OpenAPIAsJSON`), read by `extract openapi` | it runs the service's Define hooks, so a loop, a module or a setting is already resolved; its dialect is the one SpecArch's dxlib generator writes |
| an endpoint's privileges | the same document, `x-dxlib-privileges` | one lower-case dotted name is the permission; a name in dxlib_module's capitals (`ORDER.CREATE`, `GLOBAL.SET_MAINTENANCE_MODE`, `EVERYTHING`) maps by the fixed rule below, inferred; none, more than one, `public`, a name the rule cannot map, two names that give one permission and a value that is not a list of names are each a `must` question, since dxlib lets any one through and an operation has one permission |
| where an endpoint is registered, its handler, its middleware chain | source: `NewEndPoint`, `NewWSEndPoint`, `RegisterHandler` calls | the document names the chain (`x-dxlib-middlewares`) and not where the endpoint is registered or which handler runs; only the chain's code says whether a caller must sign in |
| parameters a handler reads | source: `GetParameterValueAs...` calls on the request | only the body says which are read; a getter on a name the endpoint does not declare is a `must` question, since it fails at run time |
| problems a handler answers | source: dxlib's refusal and problem calls with a literal status and reason | known only when a request runs; each literal status becomes a response, its reason the problem type's last segment |
| validation | declared types' bounds from the document; a handler's own check from source | a check in the body is code; one the reader recognises (a length, a range, an empty test) is stated, any other a `should` question at its line |
| tables and fields | the catalogue dump after every migration (step 2); `NewModelDBTable` from source adds the table, its columns' names, their types by dxlib's data types, nullability and the key, named as the catalogue reader names them so the two meet | the catalogue is what the database holds, and its other keys (references, uniques, defaults) are left to it; a service that builds its DDL from the model can emit it instead, see the dxlib changes below |
| list endpoints' search, filter and order whitelists | source: the last three arguments of `NewDXTableSimple` and dxlib's other table constructors, at the endpoint whose handler is that table's `RequestSearchPagingList` | nothing at run time lists them; the meta-model holds no whitelist on an operation, so they are cited at it with a line, and one that is not a literal list is a `should` question |
| type registry | the dxlib rows of the type-rendering idiom (`idioms/type-rendering/`) | dxlib's type names are a fixed table, read by name; an unknown name is a line |
| roles, privileges and grants | a permission table from dxlib_module's tables (ADR-050), its names mapped by the rule below; seed calls from source, inserts into the `Role` and `Privilege` tables with a literal `nameid` and `RolePrivilege...Insert` with a role id the same function assigns from a role insert, add `path:line` and descriptions | the tables are what the check reads; a seed that did not run is not a grant |
| the privilege EVERYTHING | the permission table | a `must` question, never expanded |
| gates on a setting | declared by the permission printer; found in source in the middlewares an endpoint's chain names: an early `return nil` while a setting read from the environment is empty or false | the printer declares, the parser finds candidates, and the merge compares; dxlib names a chain at the registration, so no implementation file names the check |
| configuration | the files dxlib's `NewConfiguration` names, read as data, with its defaults and sensitive keys; the environment read by a literal name (`os.Getenv`, dxlib's `GetEnvDefaultValue...`), from source | the file holds the value and wins over the code's default, as dxlib merges them; the code says which keys matter and which are secret; what each is for is a `must` question |
| idempotency | none | dxlib declares no idempotency on an endpoint, so nothing is written and the question of `docs/test-generation.md` stays open |
| WebSocket endpoints | the document's extension | printed as a line and a `could` question; the meta-model has no socket |

**The privilege rule.** A privilege named in dxlib_module's form, words
in capitals and digits joined by underscores in segments joined by dots,
is the permission of that name in lower case with every underscore a dot:
`GLOBAL.SET_MAINTENANCE_MODE` is `global.set.maintenance.mode`,
`EVERYTHING` is `everything`. The permission is declared `origin:
inferred`, the rule its `why`, citing the operation, the grant or the
seed that names it. `extract openapi`, `extract permissions` and `extract
go`, for its endpoints and its seeds alike, apply it. A name the rule
makes into no permission name (a segment that starts with a digit after
an underscore) or into `public`, and two names of one surface that give
one permission (`REPORT_RUN`, `REPORT.RUN`), are `must` questions, since
dxlib checks them as different privileges; the Go reader takes its
endpoints' and its seeds' names as one surface. A grant of `EVERYTHING`,
in the permission table or a seed, stays a `must` question on the role's
permissions, never expanded: a role granted it holds every permission,
which no list of grants can say (ADR-076).

**The printed document.** A document every operation of which is in
dxlib's dialect is what the running service printed, so `extract openapi`
writes its source as a code source with `reading: printed`, and the merge
compares the Go reader's tree with it as it compares a route table. A
service on dxlib's `DXApp` writes each API's document under
`DXLIB_OPENAPI_DUMP=<folder>`; any other runs a small program of its own
that defines the API and prints `OpenAPIAsJSON` (dxlib `api/OPENAPI.md`,
section 6), as `examples/notice-board/sources/testdata/cmd/openapi` does.

Changes in dxlib and dxlib_module that would help, for their own queues:

1. **dxlib: emit the model as a catalogue.** For a service that builds its
   DDL from `NewModelDBTable`, a writer of the declared tables in the
   catalogue dump's format (`tools/catalogue/catalogue.sql` in this
   repository), with no database.
2. **dxlib_module: print the permission table.** A function that reads the
   role, privilege and grant tables the check reads and writes SpecArch's
   permission table (ADR-050), with the gates the module's middleware has
   on a setting declared by name.
3. **dxlib: declare idempotency on an endpoint**, if the owner wants it
   checked: an endpoint field and an `x-dxlib-` key for the header that
   carries the key. Optional; nothing in SpecArch waits on it.

### Go on `net/http` and the common routers

| Surface | Source | Why |
|---|---|---|
| routes | the route table (ADR-044) first; source for `path:line` and the handler | the printer sees every registration |
| routes from source | `http.ServeMux` patterns (`"GET /loans/{id}"`), chi (`r.Get`, `r.Route`, `r.Mount`), gin and echo (`GET`, `Group`), gorilla/mux (`HandleFunc(...).Methods(...)`) with literal paths, on a router the module makes, is given or keeps in a field, or a value of a type of the module with `ServeMux`'s `Handle` method | each library's registration idiom; a group or mount with a literal prefix is joined, any other prefix is a `must` question |
| path parameters | `r.PathValue`, `chi.URLParam`, `c.Param` with a literal name | compared with the path's own parameters; a name the path lacks is a `must` question |
| request bodies | `json.NewDecoder(...).Decode(&v)` and `Bind`, into a struct declared in the module | the struct's fields and `json` tags give the schema; a struct outside the module is a question |
| validation | struct tags of a validator the reader knows (`validate:"required,max=50"`) | each rule with a meta-model keyword is written; any other a `could` question |
| permission checks | a call to a function the implementation file names as the project's check (`bindings.http.permissionChecks`: its package, its name and the argument that carries the permission), with a literal permission, wrapping a handler, as a route's or a group's middleware, or in the handler | a check behind a helper is not found by syntax; the project names its helper once (ADR-081) |
| gates on a setting | inside that named check: an early return that lets the request through, guarded by a test that a configuration value is empty or false | a candidate, compared with the printer's declaration |
| models | the catalogue; structs with a table name (GORM's `TableName`, sqlc's generated structs) add wire names | the database holds the data model |
| API clients | `http.NewRequest` and `http.Get` with a literal method and URL | an operation of another system; a computed URL is a `should` question |
| configuration | `os.Getenv`, `flag.String` and the like with a literal name | each a setting by name; its meaning a `should` question, a name with secret, token, key or password in it a `must` one on whether it is a secret |

## File-system routers that need no parser

The pages reader (step 9) reads Next.js's `app/` folders without a parser,
since the folders are the router. Four more routers are folders too, and
come before the JavaScript reader because they need nothing but git:

- Next.js `pages/`: one page per file, `[name]` a parameter,
  `index` the folder's own route, `_app`, `_document` and the error pages
  left out with a line, `pages/api/` files operations, each method a
  `must` question until the JavaScript reader reads the handler.
- Next.js `app/` route files (`route.ts` and the like): one path per file,
  its methods a `must` question until the JavaScript reader reads the
  exported `GET`, `POST` and the rest.
- Next.js `middleware.ts`, named `proxy.ts` from Next.js 16, beside `app/`
  or `pages/`: its presence is a `must` question on every page and
  operation it may cover, until the JavaScript reader reads its `matcher`
  and checks.
- Nuxt `pages/`, `server/api/` and `server/routes/`: `[id].vue` a
  parameter, `[...slug].vue` a catch-all printed as a line (as the pages
  reader does), `index.vue` the folder's own route, `loans.vue` beside a
  `loans/` folder with an index page the parent of a nested route and a
  line; `server/api/loans/[id].get.ts` an operation whose method is in the
  file name, a file with no method a `must` question as for Next.js.
  Global route middleware (`middleware/*.global.ts`) and
  `server/middleware/` are asked as for `middleware.ts`.
- A folder named `pages` is Next.js's or Nuxt's as the nearest
  `package.json` names `next` or `nuxt`; when it does not say, the reader
  reads `.vue` files as Nuxt's and asks.

## Swift

Read by SwiftSyntax (Apache-2.0, the parser the Swift toolchain's own
tools use) in `readers/swift/`, a Swift package that pins swift-syntax
exactly with `Package.resolved` committed (ADR-087). SwiftSyntax has no
type checker and the alternative, `swiftc -dump-ast`, needs a full build
and prints no stable format, so a type the source does not write is a
question. `tools/code-facts/dump-swift.sh` runs the reader on the tracked
Swift files of a committed folder; the facts it writes are syntax (imports,
types, properties, enum cases, functions, the calls its idiom table names,
with their literal arguments and closures), and `specarch extract swift`
turns them into elements. Property lists, `.xcconfig` files and Core Data
models are read as data, in Go, at the dump's commit.

| Surface | Source | Why |
|---|---|---|
| screens | source: a `View` that `WindowGroup` shows, a `TabView` tab, or `NavigationLink`, `navigationDestination`, `sheet`, `fullScreenCover` or `popover` opens is a page; any other view is a part of one and no element | the navigation calls are what make a view a screen; a screen has no route of its own, so its route is asked with its kind |
| navigation | source: a literal destination is a `navigate` action of the screen it is in, its label the link's literal title; a `TabView` tab is a menu entry | a destination chosen at run time is a `should` question; a flow needs a task and an actor the code does not give |
| a screen's fields | source: `TextField`, `SecureField`, `TextEditor`, `Toggle`, `Picker`, `DatePicker`, `Stepper` and `Slider` bound to `$model.property` | fields by name; the entity where the bindings, or a `@Query`, name one SwiftData model |
| models | `.xcdatamodeld` read as data for Core Data; source for SwiftData `@Model` classes (entities), `Codable` types (schemas) and `Codable` enums of strings | the Core Data model is a file of data; `CodingKeys` give wire names; a word-wide `Int`, a `Float`, a `Decimal` and a relation's key are questions |
| server routes (Vapor) | the route table where the project prints one; source: `app.get("loans", ":id", use:)`, `grouped` and `group` with literal segments, and a `RouteCollection` under its `register(collection:)` | as for Go |
| permission checks | source: a middleware a group is given, made by a check the implementation file names | an iPhone app checks nothing for the server, so a screen's permission is asked |
| API clients | source: `URLRequest` with a literal `httpMethod`, and URLSession's `data`, `download` and `upload` given a URL from literal parts | a part interpolated from one value becomes `{name}` with a `should` question naming it; each names a dependency by host |
| configuration | `Info.plist` and `.xcconfig` read as data; `Bundle.main.object(forInfoDictionaryKey:)`, `infoDictionary[...]`, `ProcessInfo`'s environment and Vapor's `Environment.get` from source | the files hold the values; the bundle's own keys and the entitlements are not settings the app reads |

## JavaScript and TypeScript

One reader, `readers/javascript/`, running the TypeScript compiler (Apache-2.0,
no dependencies of its own) on Node, which parses JavaScript, JSX,
TypeScript and TSX, ES modules and CommonJS (ADR-089). A program is built
over the project's tracked files only, with no `node_modules`, so the
checker resolves the project's own types and the types that come with
TypeScript, and a type from a package is a question. The TypeScript
compiler API was chosen over tree-sitter for that type checker and
because it is the parser every TypeScript tool uses. The reader pins
6.0.3, the last release whose compiler API is stable; the 7.x releases
are a native port with one binary per platform and only unstable entry
points. `tools/code-facts/dump-javascript.sh` runs it on a committed
folder, and its compiler host refuses to read any file it was not
given, so a package's names are known only by the import that names
them; the idiom table keys on that import. The facts are syntax with what
the checker resolves: a name's import and declaration, and types as the
source states them, from TypeScript or from JSDoc.

| Surface | Source | Why |
|---|---|---|
| back-end routes | Express (`app.get`, `router.post`, `app.use('/prefix', router)`), Fastify (`fastify.get`, `route({method, url})`, `register` with a literal `prefix`), with literal paths | the registration idioms; `:id` becomes `{id}`, a regular expression path is a line |
| request and response bodies | the declared types of a handler's request and reply (`Request<Params, Res, Body>`, Fastify's generic) and a validation schema the reader knows (zod, yup, joi, Fastify's JSON Schema) | a schema of a validator is stated in both languages |
| permission checks | middleware the implementation file names, called with a literal permission | as for Go |
| models | the catalogue; zod, yup and joi object schemas a module-level variable holds, and the types a route declares; ORM declarations (Prisma's `schema.prisma` read as data, TypeORM and Sequelize decorators and `define` calls) add names in a later step | the database holds the data model; a validation schema is checked at run time, so it is stated in both languages |
| React screens | the file-system routers above; React Router's `createBrowserRouter` and `<Route path>` with literal paths, the component a route shows a page with that route and its heading the title; plain pages whose files an implementation file names, once it has a key for them | the router is what makes a component a screen |
| a screen's fields | form controls bound to a model property by a known form library (react-hook-form `register('name')`, Formik's `Field`, `getFieldProps` and `useField`); component library inputs an implementation file maps, once it has a key for them | fields by name, in the order of the source |
| API clients | `fetch`, axios and the instances `axios.create` makes, with a literal method and URL, or a template literal whose parts are literals, consts and one value per segment; the project's client wrapper once the implementation file has a key for it | another host is a dependency; a path of the system's own is a should question until a question can carry a method and a path, which the merge then joins to the operation |
| configuration | `process.env.NAME` and `import.meta.env.NAME` | as for Go |
| strings | next-intl, i18next and the like: `t('loans.title')` with a literal key, the catalogue `messages/<locale>.json` read as data | a page's title from the default locale; a computed key is a `should` question; other locales a line, since the meta-model holds one language |

**What plain JavaScript loses without types.** The parser reads the same
syntax, and the facts it cannot give become questions at the entry:

- a request or response body's shape when no validation schema states it:
  the properties a handler reads (`req.body.title`) are fields known only
  by name, and their types a `must` question;
- every field's type and nullability in a model or a form with no schema;
- a union of literal strings that TypeScript would read as an enum;
- a component's props, so a field a page passes to a component library is
  known by name only;
- what an API client function returns;
- which exported function is a handler where the file name does not say;
- the target of a `require` or a dynamic `import()` with a computed
  argument, and an export assigned at run time (`module.exports[name]`),
  each a question at its line.

JSDoc types are read as stated only where the project's `jsconfig.json`
or `tsconfig.json` has `checkJs` on, since only then does the compiler
check them against the code; otherwise each is a `should` question quoting
the comment. TypeScript's `number` has no width either, so a width is a
question in both languages, as it is for an OpenAPI number with no format.

### Next.js, read with the JavaScript reader

Once the folders of the earlier step are read, the JavaScript reader reads
their content: `extract pages` is given the dump of the folder that holds
the router with `--facts` (ADR-090), so one tree holds the routes the
folders give and what the code says of them:

- route handlers' and API routes' exported methods, each an operation;
- server actions (`'use server'` functions and files) called from a form,
  each an operation by the function's name at the route of the page that
  shows the form, method POST, with a `must` question on the path, since
  a server action has none of its own and Next.js posts it to that page;
- `middleware.ts`'s `matcher` with literal patterns, and the checks in it
  through a function the implementation file names; anything else is a
  `must` question on every route it may cover;
- `generateStaticParams` with literal values, named in the question on a
  route file's parameters and printed as a line on a page, which holds no
  parameter's values;
- `page.schema.ts` beyond the JSON5 subset the pages reader holds now, its
  imported constants resolved where they are in the files read.

## Vue and Nuxt

Vue single-file components are split with `@vue/compiler-sfc` (MIT) in
the JavaScript reader's program, their script read by the TypeScript
compiler and their template by `@vue/compiler-dom`. Its dependencies
(`@babel/parser`, `postcss`, `magic-string`, `source-map-js` and the like)
are MIT or BSD and are scanned with the reader when the step is built.
tree-sitter-vue was left for the same reasons as tree-sitter. Plain Vue with
vue-router is the same reader.

| Surface | Source | Why |
|---|---|---|
| pages | Nuxt `pages/` (the folder step), layouts by name from `definePageMeta({ layout })`; vue-router's `routes` array with literal paths | the router |
| guards | Nuxt route middleware (`middleware/` and `definePageMeta({ middleware })`), vue-router's `beforeEach` and `beforeEnter` through a check the implementation file names | a page's permission; any other guard a `must` question on the page |
| page metadata | `definePageMeta` with literal values: title, permission where the project's own key names it | a key the implementation file does not map is a line |
| operations | Nuxt `server/api/` and `server/routes/` files (the folder step), their handlers' `readBody`, `getQuery` and `getRouterParam` with literal names, `readValidatedBody` with a schema the reader knows | as for Express |
| a screen's fields | `v-model="form.title"` on inputs and on the components of a library the implementation file maps, such as PrimeVue's `InputText`, `Dropdown`, `Calendar` and `DataTable` columns (`<Column field="title">`) | fields by name; a list's columns from the table's columns |
| API clients | `$fetch`, `useFetch` and axios with a literal method and path | joined to operations by the merge |

## Dart and Flutter

Read by the analyzer package (BSD-3-Clause, the parser of the Dart SDK's
own tools) in `readers/dart/`, a Dart package with `pubspec.lock`
committed, using `parseString`, which gives syntax without resolving
packages; a resolved unit needs `pub get` and the pub cache, which is no
commit.

| Surface | Source | Why |
|---|---|---|
| screens and navigation | go_router's `GoRoute(path:, name:, builder:)` with a literal path, nested routes joined, `ShellRoute` and `StatefulShellRoute` as menus; `Navigator.push` with a `MaterialPageRoute` whose builder returns a literal widget | a `redirect` callback is a guard: a `must` question on the routes it covers, since it is code |
| a screen's fields | `TextFormField` and the other `FormField`s with a controller or `onSaved` naming a model property | a `validator` the reader recognises (an empty test, a length) is stated, any other a `should` question |
| models | `@JsonSerializable` and `freezed` classes, `@JsonKey(name:)` giving wire names | the code generator's input, not its output, since the output is generated and says so |
| API clients | retrofit's `@GET('/loans/{id}')` annotations; dio and `http` calls with a literal path or one interpolated value per segment | as for JavaScript |
| server routes | `shelf_router`'s `router.get('/loans/<id>', ...)`; dart_frog's `routes/` folders, read as a file-system router | as for Go |
| configuration | `String.fromEnvironment('NAME')` and the other `fromEnvironment` constructors; flavour files read as data | as for Go |

## Why this order

1. **dxlib's emitted document** first: most of the owner's Go services are
   on dxlib, the document exists, and reading `x-dxlib-privileges` is a
   change to a reader that is built.
2. **Go source** next, dxlib first: the first project to need extract is a
   Go service, the parser is in the standard library, and the reader runs
   inside `specarch` with no new toolchain, dump format or scan.
3. **The file-system routers that need no parser**: they extend the pages
   reader with git alone, and the first project's front end is Next.js.
4. **Swift** before JavaScript, as the owner ordered and as the current
   work on phone and Mac applications needs; it brings the code-facts
   format and the first program under `readers/`.
5. **JavaScript and TypeScript**, then **Next.js content** on it: the
   content of route files, server actions and middleware needs the
   JavaScript reader, so it cannot come earlier.
6. **Vue and Nuxt** after Next.js: they add a second parser on top of the
   JavaScript reader, and the pages reader already reads a Next.js tree.
7. **Dart and Flutter** last, as the owner ordered.

Moving 5 ahead of 4 would serve a Next.js front end sooner; the order
above keeps the owner's.
