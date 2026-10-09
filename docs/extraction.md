# Extraction

How an existing system gets its as-built specification, and what goes wrong
when that is done carelessly. This is the method behind the sixth rule in
the README: existing code enters by extraction, and from then on changes go
spec first.

## The method

1. Find the authoritative source for each surface. The place the running
   system actually reads a fact from, not the place the documentation says it
   lives. The table below lists the usual ones.
2. Extract mechanically. Each surface is read by a script or a query into the
   as-built spec. Nothing is typed in from a document or from memory.
3. Verify by regenerating. Run the generators on the as-built spec and
   compare their output with the source: routes against the router, tables
   against the catalogue, generated code against the committed code. Record
   every difference.
4. Only then review by hand. A reviewer reads the spec, fixes what the
   extraction got wrong, and marks it the master. From that point a
   difference between spec and code is a defect in the code, and the gates in
   `docs/sync-gates.md` keep it that way.
5. Record what the meta-model could not hold. Every concept the code has that
   the spec cannot express is listed with the extraction. Those lists are the
   input to the next meta-model version.
6. Write what the source does not say as open questions, never as guesses.
   Every element carries its `origin`: stated, with the citation of where
   the source says it; inferred, with the reason; or decided, once a
   stakeholder has answered. `specarch gaps` lists what is still open and
   which outputs can already be made; code is generated only once the
   questions are answered and the owner has approved the documents.
   `docs/refinement.md` is the design of that path, and
   `docs/from-sources.md` the procedure an agent follows when the system
   has both documents and code.

| Surface | Authoritative source | How to read it |
|---|---|---|
| endpoints | the built router | list its registered method and path pairs |
| data model | the database catalogue after every migration is applied | dump tables, columns, keys, indexes, checks |
| permissions | wherever the running authorisation check reads its rules | often tables filled by seed scripts, not a policy file |
| events | the publisher calls and the topic or channel registry | list topics, then the payload type each publisher sends |
| pages | the schema objects the component library renders from | read them as data |
| algorithms | the code, with the numbers from an existing test or a real record | the record becomes the first worked example |
| decisions | commit history, review threads, chat | the only surface that is written by hand |

Until its readers are built, extraction is a manual method with this
checklist, and each surface's reader is a small script kept in the
project's repository. `specarch extract <source>`, the verb that goes from
existing code or documents to a specification, is designed in
`spec/design/commands/extract.yaml` and built one reader at a time in the
steps of "Building extract" below. The Go build reads the sources
`outline`, `database`, `router`, `documents`, `openapi`, `permissions`, `pages`, `workflows`, `go` and `swift`, and `specarch merge` joins their trees; a source not built yet is answered with status
2, and the Swift build has no extract or merge verb.

## Building extract

The first real project to need `specarch extract` is a production service
written in Go behind an HTTP router, with its data in Postgres under
versioned migrations, a web interface on a file-system router, and several
hundred pages of design documents that have drifted from the code. The
verb is built for it in the steps below, each a work item of its own that
changes the specification of `specarch` first, adds its conformance cases,
and grows `examples/lending-desk/` with the sources it reads.

Every reader shares these rules:

- A reader reads one surface and writes a partial specification tree into
  `--out`: a root file with `tracksOrigin: true`, one source per document
  file and one per code repository, and every element `origin: stated`
  with `cites`, or `origin: inferred` with `why`. What the surface does not
  say is a question, never a value.
- What the meta-model cannot hold prints a line and is a `could` question
  in the tree, citing where the source says it (its file and line when the
  reader knows the line), so it reaches the problems file with every other
  problem. A thing a `must` question already asks for gets no second
  question; its line names that question.
- A code source is one repository, and its `edition` is the full hash
  of the commit the reader read, which the reader prints. It is the
  newest of the commits that last changed each path read, so a commit
  elsewhere in the repository leaves the output as it was. Git runs in
  the repository that holds the path, a submodule's own when the path is
  in one. Only tracked files are read; a path with changes not committed,
  or with files that are neither tracked nor ignored, is refused, since no
  commit names what was read, and so is a shallow clone, whose history
  cannot name the last change. A dump made from code, such as a
  catalogue or a route table, is itself committed and names the commit
  and the path it was made from; that commit is its edition, and a dump
  older than the last change to that path is refused as stale.
- Standard output is one line per object the meta-model could not hold,
  one line naming the commit read, and every count with what was counted
  and how.
- The same sources at the same commit give byte-identical output: no
  dates, no run times, no order taken from a map or the file system.
- A fact found in more than one place is reported at every place, with a
  question asking which one the running system uses, unless the reader
  can tell.
- A comment saying a file is generated from another source and must not
  be edited names that source; the reader reports it, so it is read and
  reconciled too.

Merging is a verb of its own, `specarch merge --out <folder> <tree>...`,
since its input is specifications and not a surface: each tree is
extracted and checked on its own, then merged, and the merge is reviewed
apart from the readers. The same entity or operation from two trees
becomes one element with both citations. Where the sources disagree, or
only one of them has an element, the merge follows the table of
`docs/from-sources.md`, section 3.2, whose priority rule step 4 brings
to this: the question is `must` when the element concerns security (a role, a
permission, an operation, a limit, or a field that is personal or a
credential) or when the source that has it is one the system's owners
gave to parties outside, such as a published interface or a contract;
otherwise it is `should`. An undocumented route the code serves is the
usual way an open endpoint is found, so it is never left at `should`.

Steps, in order:

1. Built. The usage text marks `extract` as designed, not built, in both
   builds, and both builds refuse it with the same message.
2. Built. The shared core, the outline reader and the database reader,
   under the requirement SA-44, with `extract.yaml` holding the rules
   above. `extract outline` writes a source with its files as clauses and
   no elements, for a surface no reader reads yet, such as workflow
   definitions in a format other than BPMN 2.0, so that `specarch gaps`
   lists it as not read rather than
   losing it. `extract database` reads a catalogue dump into entities:
   columns, types, nullability, defaults, primary and foreign keys,
   unique keys and checks; a check the expression subset can say becomes
   its expression, and a list of allowed values becomes the field's enum.
   An index, a view, a type with no row in the type-rendering idiom and a
   check the subset cannot say each print a line. SpecArch ships the
   catalogue query, `tools/catalogue/catalogue.sql`, and
   `tools/catalogue/dump-catalogue.sh`, which starts a disposable
   PostgreSQL under Podman, applies every migration and writes the dump
   with the commit it was made from; the dump is committed beside the
   code, so a run can be repeated without a database. The script is run
   by hand; CI reads the committed dump and needs no Podman. The lending
   desk's migration checks that the due date is fourteen days after the
   loan date, so the loan period is in the catalogue, and its dump is
   `examples/lending-desk/sources/catalogue/catalogue.json`. Conformance
   cases build a throwaway git repository with a fixed author, committer
   and dates, so the hashes they record never change.
3. Built. The router reader, `extract router`: one operation per method
   and path pair, with its path parameters and the permission it checks,
   from a route table the running router prints, so that only routes
   actually registered appear. SpecArch defines the route table's format
   (ADR-044): a JSON object with the version of the format, the router's
   folder, the commit that last changed it, and every route as its
   method, its path with each parameter written `{name}`, the one
   permission it checks or null, and its handler's name. The project
   prints the routes with a printer of its own, which builds the router
   as the server does; `tools/routes/dump-routes.sh` runs it in a
   committed folder and adds the version, the path and the commit. An
   operation is named after its handler; its summary, its responses, its
   path parameters' values and its permission's description are must
   questions, and so is a route that checks no permission. HEAD,
   OPTIONS and a path with a wildcard print a line. The lending desk's
   printer is `cmd/routetable`, and its dump is
   `examples/lending-desk/sources/routes/routes.json`; its four routes
   come out as four operations with their parameters and permissions,
   validated and byte-identical.
4. Built. `specarch merge`, with the priority rule above, written into
   `docs/from-sources.md` section 3.2 as well, and the optional key
   `givenOutside` on a source, marking it as given to parties outside,
   in the 0.1 design schema (ADR-045). Each tree is validated on its own
   first. Elements are matched by name, an operation by path and method,
   a field by entity and name; citations are joined, and a key the trees
   give different values is left out with a `must` question citing both.
   The rows for an element only one side has apply between the code side
   and the documents side, and only where the other side has elements of
   the same section or entity. Code sources of one repository read at
   different commits become one, at the newest commit, once git shows
   every path each tree read unchanged up to it. The database and router
   trees of the lending desk merge into one specification at commit
   1334b2b with the readers' eight questions, which validates with no
   errors, `specarch gaps` reads, and a second run writes byte for byte;
   CI repeats it. A release tag after this step lets the first project
   use the database and router readers.
5. Built. A mark on an element that the project describes and compares
   but does not generate, naming the stakeholder that owns it: `ownedBy`
   on the element's mapping in the implementation file, one element at a
   time, since one file often holds a single part another team owns; an
   optional key in the 0.1 implementation schema (ADR-046). Generators
   that build code or data skip it and everything under its pointer, and
   still write what refers to it, such as a foreign key; the tests target
   keeps it, since a test checks rather than builds. The comparisons and
   the gates still check it, and the technical specification names the
   owner beside the mapping. The SQL snapshot keeps an owned table's shape
   and lists it as owned, so handing a table over writes no drop. The
   lending desk's manual gives the book records to the catalogue team,
   whose mapping of Book says so: `generate sql` and `generate openapi`
   leave Book out, the loans table keeps its foreign key to books, and
   `validate` and `gaps` read Book as before; CI runs both generators
   twice and compares. A mark naming no stakeholder is the error
   `stakeholder` in both builds.
6. Built. The documents reader, `extract documents`, for Markdown
   (ADR-048), after date arithmetic in the expression subset (ADR-047):
   a date plus or minus `duration("P14D")`, so that the catalogue's
   check on the loan period is an expression and only its message a
   question. The source gets its outline as clauses: every heading and
   every paragraph that starts with a section number, a parent only
   when it holds text of its own. A sentence that makes a commitment
   (shall, must, will, should, a number of something, a time of day,
   once or twice, a time limit) becomes a requirement with
   `origin: stated` citing its clause, and its kind a question; other
   prose produces no element and shows in the coverage. A commitment
   sentence whose subject is only a pronoun becomes a question, never a
   guess. A table headed Field gives an entity's fields; a field's
   sensitivity is read from a table column headed sensitivity, or from
   a sentence that calls the field personal or a credential; otherwise
   it is left for a question. `specarch merge` joins a requirement that
   gives a number of days to the one code check whose name holds the
   words of its subject and that moves a date by days: the same number
   makes the check satisfy the requirement, a different one leaves the
   statement out with a `must` question citing both. The lending desk
   manual gives its outline and four commitments, and merged with the
   database and router trees gives the loan period, 21 days against 14,
   as one `must` question citing clause 3.3 and the loans table,
   validated and byte-identical; CI repeats it. PDF and slide decks
   follow once a reader for them passes the dependency rules.
7. Built. The OpenAPI reader, `extract openapi`, for OpenAPI 3.0 and
   3.1, in YAML or JSON, with no dependency beyond the YAML library
   (ADR-049): operations, parameters, request and response schemas, as a
   document source whose clauses are the JSON pointers read. A component
   schema of type object is an entity, its primary key a question, when
   an operation's 201 answers it or a path with a parameter answers it;
   any other is a schema under `schemas`, with no key to ask for
   (ADR-058); 3.0's nullable, example and boolean exclusive bounds are
   written in the 3.1 form; a security scheme is not a permission, so every operation's
   permission is a question, and so is a number with no width the
   meta-model holds. What the meta-model cannot hold prints a line.
   A document whose property names are snake_case is read with
   `info.wireNames: snake_case`, each name in camelCase (ADR-062); a
   snake_case name in a document whose other names are camelCase, or one
   that would not go back on the wire unchanged, is left out with a line. `specarch merge` leaves out a tree's question when
   another tree gives every key it blocks, and reports a documents-side
   source none of whose paths the code serves as a placeholder. The
   lending desk's OpenAPI document,
   `examples/lending-desk/sources/openapi/openapi.yaml`, has the four
   routes and a renewal the router does not serve; merged with the
   router tree it gives the summaries, responses and parameter values,
   writes the renewal's refusal body as a schema with no key question,
   takes the permissions from the router, and asks one question naming
   `/loans/{loanId}/renew`, validated and byte-identical; CI repeats it.
   A sample document whose paths the router serves none of is reported
   as a placeholder.
8. Built. The permissions reader, `extract permissions`: roles and the
   permissions each grants, from the tables and seed scripts the running
   check reads (ADR-050). SpecArch defines the permission table's format,
   as it does the route table's: the version of the format, the folder
   the check is built from, the commit that last changed it, every grant
   as a role and a permission, and every check that runs only when a
   setting is present, and so passes everything when the setting is
   empty, by the check's name and the setting's. The project's own
   printer reads the grants where the running check reads them and
   declares those checks; `tools/permissions/dump-permissions.sh` runs it
   in a committed folder and adds the version, the path and the commit.
   Each role and each permission granted is written, their descriptions
   must questions, and each declared check prints a line and is a `must`
   question. `specarch merge` asks a question two trees ask only once, and
   reports a permission a role grants that no operation, command or page
   checks. The lending desk's roles move from a Go map to a roles table
   and a seed script, which its check reads with `lending/grants.sql`;
   its printer, `cmd/permissiontable/print.sh`, runs the migrations and
   the seeds in a disposable PostgreSQL under Podman and runs that query,
   and its dump is `examples/lending-desk/sources/permissions/permissions.json`.
   Its one role and three permissions come out validated and
   byte-identical, and a seed that grants a permission no route checks is
   reported by the merge; CI repeats both.
9. Built. The pages reader, `extract pages`, for a file-system router
   tree such as an `app/` folder (ADR-057): one page per folder that
   holds `page.tsx`, `page.ts`, `page.jsx` or `page.js`, read from the
   files git tracks, a folder `[name]` the parameter `{name}` and a
   folder in parentheses a route group that adds nothing to the route.
   Two folders giving one route are refused; a catch-all segment, a
   parallel or intercepting route, a private folder and another page
   file each print a line, and a route handler's methods are asked for
   (step 16). A page whose folder holds
   `page.schema.ts`, the schema its component library renders from,
   takes its content from it: the file is read in the subset of
   TypeScript that is also JSON5, the form the TypeScript generator of
   ADR-051 writes, and its keys named as the design's page keywords give
   the kind, title, entity, permission, columns, fields, sections and
   filters. The operations it names are the router's, so the page's
   source and submit are left out with a must question that carries the
   name under `names`, and `specarch merge` writes the name at the key
   once another tree declares that operation. The entity is written by name with the fields the page
   shows, each known only by name, and a must question asks its primary
   key and their types; `specarch merge` takes an element known only by
   name as not given, so the database tree answers that question, and
   `validate` covers everything under a field known only by name that a
   must question blocks. The lending desk's
   `examples/lending-desk/sources/web/app/`, beside its code so that its
   dumps stay current, has a route group, a dynamic segment and one list
   page with a schema file: its three pages come out with the parameter
   and that page's fields, validated and byte-identical, and merged with
   the database tree the entity's question is answered; CI repeats it. In
   the whole example (step 10) the list page's source is joined to
   `listMemberLoans`, which the router and the OpenAPI document declare.
10. Built. The whole example: `examples/lending-desk/extract.sh <out folder>`
    runs every reader on the example's sources, each into a tree of its
    own: the manual, the OpenAPI document, the catalogue, the route table,
    the permission table and the app folder. It merges the trees in that
    order, validates the result and writes its gaps, each step's output
    beside the trees.
    It works inside the out folder and names what it writes relative to
    it, so two out folders side by side get the same bytes. The manual's
    table of a member's details has a phone number, personal, that the
    members table does not; the router serves `GET /members/{cardNumber}`,
    which checks no permission and which neither the manual nor the
    OpenAPI document mentions. `specarch gaps` lists the phone number as
    a `must` question citing clause 2 and the members table, and the
    route as two: the router's, on its permission, and the merge's,
    undocumented, from code. The members table keeps a member's address
    in `address_street`, `address_city` and `address_postcode`, and the
    OpenAPI document gives `Member` an `Address`; since the document's
    `Member` and the table's `Members` are two entities, no tree gives
    `Members` an address, and the merge asks whether the three columns
    are one value (ADR-077). The merged specification validates with no
    errors, and CI runs the script twice and compares the folders. A
    release tag after this step.

11. Built. The workflows reader, `extract workflows`, for BPMN 2.0 XML
    read with the standard library (step 12 of `docs/meta-model-0.2.md`).
    Each process is a workflow in the sequential subset of ADR-054: user
    tasks are approvals with their potential owners as roles and a timer
    as the deadline, an exclusive gateway after an approval with one flow
    to an end event is its refusal, and service tasks are operation steps.
    BPMN names the operations a workflow calls but does not declare them,
    so the trigger and each step's operation are left out with a must
    question that carries the name under `names`; `specarch merge` writes
    the name at the key once another tree declares that operation, and
    prints the question as joined. Everything outside the subset prints a
    line with its file and line and is asked for. The lending desk gains
    a write-off that a desk supervisor approves within two days:
    `sources/workflows/write-off.bpmn`, the routes for the request, which
    answers 202, and for the write-off, and the supervisor's role in the
    seeds. End to end, the merge joins the trigger to the router's
    operation, documented with its 202 in the OpenAPI document, and the
    write-off step to the router's, and the merged specification
    validates with no errors; the approval's permission and the subject
    stay must questions, since BPMN has neither.

The steps from 12 read source code itself, by a parser of each language,
for what no running system prints (ADR-075). `docs/reading-code.md` sets
out per language and framework what is read from source and what stays
with a printer, the code-facts dump, and how a parsed tree merges with the
printed ones; the dxlib services are ADR-076. Each step adds its readers'
idioms, a generic example under `examples/` and conformance cases, and
changes `extract.yaml` first.

12. Built. dxlib's emitted document (ADR-076). `extract openapi` reads
    an operation that carries `x-dxlib-endpoint-type` in dxlib's dialect:
    the one name in `x-dxlib-privileges` is its permission, stated and
    declared, with its description and the role that grants it must
    questions; none, more than one, a name that is not lower-case words
    joined by dots, `public`, which dxlib grants like any other privilege,
    and a value that is not a list of names are each a must question,
    never written as public. The security an operation is given prints a
    line, since it is not its permission. The
    conformance case reads a document in that dialect with each of these.
    A name in dxlib_module's capitals is mapped to a permission name by
    the fixed rule of ADR-076, inferred, and a document wholly in the
    dialect is a printed code source (step 13).
13. Built. The shared core of a parsed reader and Go on dxlib's
    endpoints. `reading: printed` and `reading: parsed` on a code source
    are in the 0.1 design schema, which both validator builds share;
    router, database and permissions write `printed`, and an OpenAPI
    document wholly in dxlib's dialect is a printed code source. `specarch
    merge` writes the rows of `docs/reading-code.md` for operations: the
    parsed tree's question whether the running system registers an
    operation is left out where a printed tree has it, an operation only
    the parsed tree declares is a must question citing its line, and one
    only the printed tree has a could question. `extract go` reads a
    module's tracked files with `go/parser`: dxlib's `NewEndPoint` calls
    with literal values become operations citing the registration's and
    the handler's `path:line`, with the handler, the middleware chain and
    the privileges in the citation, the parameters the handler reads and
    the problems it answers; a registration in a loop, behind a
    condition or computed, a parameter read the endpoint does not
    declare and a reason that names no problem are must questions;
    `NewWSEndPoint` and `RegisterHandler` print a line. dxlib_module's
    privilege names map to permission names by the fixed rule of
    ADR-076 in both readers. `examples/notice-board` is a service on
    dxlib with the document dxlib emitted from it committed; its
    `extract.sh` merges the two readings, and each operation has two
    citations.
14. Built. Go on dxlib's tables, seeds and configuration. `extract go`
    writes each table `NewModelDBTable` declares with a literal schema and
    name as the entity `extract database` writes for it, its columns'
    types by dxlib's data types as the catalogue writes them, and leaves
    the field keys the catalogue reads to it; dxlib's table constructors
    give a paging list endpoint its search, order and filter whitelists,
    cited at the operation with a line, since the meta-model holds none.
    dxlib_module's role and privilege inserts and `RolePrivilege...Insert`
    calls give roles and their grants, mapped by the rule of ADR-076 with
    the endpoints' privileges as one surface, and `extract permissions`
    maps a permission table's names by the same rule; a grant of
    EVERYTHING is a `must` question in both. The environment read by a
    literal name and the files dxlib's `NewConfiguration` names, read as
    data, give settings under `configuration`. A gate on a setting is
    found in the middlewares an endpoint's chain names, since dxlib names
    them at the registration, and is asked in the words the permission
    table reader uses. `specarch merge` extends the rows of
    `docs/reading-code.md` to entities, roles and their grants, and joins
    a gate both readings ask by the check's name. `examples/notice-board`
    gains its notice table, its seeds, a configuration file and a session
    check a setting switches off.
15. Built. Go on `net/http` and the common routers (ADR-081). `extract
    go` follows the routers a module makes, is given or keeps in a
    field, from each function no call names, through the calls that pass
    them, joining the literal prefixes of groups, routes, mounts,
    subrouters and `ServeMux` subtrees: `http.ServeMux` patterns, chi,
    gin, echo and gorilla/mux. A type of the module with `ServeMux`'s
    `Handle` method is read as a `ServeMux`, so a route printer's
    recorder is one. Each route registered with literal values outside
    a loop or a condition is an operation named as extract router names
    one; handlers give the path parameters they read, the JSON body they
    decode into a struct of the module, by encoding/json's rules, with
    go-playground/validator's and gin's tag rules that have a keyword,
    and the dependencies their client calls name by host; flags are
    settings. The implementation file names the project's checks under
    `bindings.http.permissionChecks`, given to extract go with
    `--implementation`; a check's literal permission, wrapping a
    handler, as middleware or called in it, is the operation's, and a
    gate on a setting is found in it. The lending desk's server
    registers literal `ServeMux` patterns through `Server.Require`, its
    printer hands the same function a recorder, and `extract.sh` merges
    the Go reading with the route table: each of the seven operations
    has both citations and no key differs. Responses a handler writes
    and models with a table name are left for a later step.
16. Built. The file-system routers that need no parser (ADR-084), in
    the pages reader. The folder's name tells the router: `app` and any
    other name the App Router, `server` Nuxt's server, and `pages`
    Next.js's Pages Router or Nuxt's pages as the nearest `package.json`
    names `next` or `nuxt`; when it names neither or both, `.vue` files
    decide and a must question asks. Pages come from files in `pages/`,
    `index` a folder's own route, with `_app`, `_document`, the error
    pages, catch-all and optional segments, a Nuxt parent that wraps a
    nested route and a second file on one route each a line. Route
    files, `route.ts` in `app/`, `pages/api/` and Nuxt's `server/api/`
    and `server/routes/`, give paths: a Nuxt file whose name ends in a
    method is that operation, and every other route file is a must
    question citing it, asking for its methods. `middleware.ts` or
    `proxy.ts` beside `app/` or `pages/`, Nuxt's global route
    middleware and `server/middleware/` are a must question citing the
    file on every page's and operation's permission. Conformance cases
    read an App Router with middleware, a Pages Router with `proxy.ts`,
    Nuxt's pages and server, and a folder whose framework is guessed;
    the lending desk's web folder gains `middleware.ts`.
17. Built. Swift (ADR-087): the code-facts dump format (`codeFacts:
    1`), `tools/code-facts/dump-swift.sh` and `readers/swift/` on
    SwiftSyntax 604.0.0, pinned exactly, Apache-2.0, its SBOM scan clean.
    `extract swift` reads a committed dump, refused when another
    SwiftSyntax version made it, and writes SwiftUI screens with their
    titles, fields and navigate actions, `TabView` tabs as menu entries,
    SwiftData and Core Data models as entities, `Codable` types as
    schemas, Vapor routes with the project's named permission check,
    URLSession clients as dependencies, and the settings `Info.plist`,
    its `.xcconfig` values and the code's reads give. `examples/reading-list`
    is an iPhone app and its Vapor server, each read from its dump; its
    `extract.sh` merges the two trees. A screen's call joined to the
    operation it calls, by method and path, waits for a question that
    can name a method and path.
18. Built. JavaScript and TypeScript (ADR-089): `readers/javascript/` on
    the TypeScript compiler 6.0.3, pinned exactly, Apache-2.0, with no
    dependency of its own, its SBOM scan clean, run by
    `tools/code-facts/dump-javascript.sh` over the tracked files of a
    committed folder and nothing else, so no `node_modules` is read.
    `extract javascript` reads a committed dump, refused when another
    compiler version made it, and writes Express and Fastify routes with
    the permission the project's named check gives, through `use`,
    `register` and `route`; a request body from a validation schema the
    handler applies, Fastify's JSON Schema, the declared `Request` or
    generic, or the fields the handler reads; zod, yup and joi object
    schemas and enums; the types routes declare as schemas and enums;
    React Router's routes as pages with their headings, the fields
    react-hook-form and Formik bind and their links as navigate actions,
    text from the default locale's message catalogue; fetch and axios
    calls as dependencies; and `process.env` and `import.meta.env` as
    settings. A JSDoc type is read as stated only under `checkJs`, and
    is otherwise a should question quoting the comment.
    `examples/room-booking` is one Express service in TypeScript and in
    plain JavaScript; its `extract.sh` reads both and writes the
    questions only one asks, which are the ones types answer. A call to
    the system's own path, a project's client wrapper, a component
    library's inputs and ORM models wait for later steps.
19. Built. Next.js content on the JavaScript reader (ADR-090): `extract
    pages --facts` reads the App Router's and the Pages Router's files
    through the dump of the folder that holds them, with the checks
    `--implementation` names. A route file's operations are the methods
    its code serves, exported by name or compared with `req.method`,
    each with the permission a check wrapping it, called in its branch or
    called in middleware gives, and the body a validation schema checks;
    a server action a page's form submits to is an operation at the
    page's route with a must question on that path; `middleware.ts`'s
    literal matcher limits its question to the routes it may cover;
    `generateStaticParams` with literal values is named in the question
    on a route's parameters; and `page.schema.ts` is read as the compiler
    reads it, its consts resolved. The lending desk's web folder is read
    through its dump.
20. Built. Vue and Nuxt (ADR-091): `@vue/compiler-sfc` 3.5.43 in the
    JavaScript reader, pinned exactly, MIT with an MIT, BSD and ISC tree,
    its SBOM scan clean; a `.vue` file's scripts keep their lines and its
    template's elements are facts. The implementation file names a UI
    library's components under `bindings.ui.components`, checked by both
    validators. `extract javascript` reads vue-router's routes as pages
    with their headings, `v-model` fields, mapped PrimeVue fields and
    columns, and the permission its guards' check names; `extract pages
    --facts` reads Nuxt's pages with `definePageMeta`, their named and
    global middleware, and Nitro handlers with their checks, validated
    bodies and route parameters. `$fetch` and `useFetch` are clients.
21. Dart and Flutter: `readers/dart/` on the analyzer package; go_router
    and Navigator, form fields, `json_serializable` and `freezed` models,
    retrofit, dio and `http` clients, `shelf_router` and dart_frog routes,
    `fromEnvironment` settings.

## What goes wrong

Each of these was met in practice.

### The obvious source is wrong

In one system the authorisation library's adapter was a stub that granted
nothing, and the real permissions lived in database tables that seed scripts
filled. Extracting from the library's policy file would have produced a
confident, wrong model with every check passing against it. Trace the running
check to where it reads before extracting anything.

### Prose specifications lie quietly

A data-model document listed tables that no migration had ever created, and
nobody had noticed because nothing compared the two. Documentation is a list
of questions to put to the running system, never a source to copy from.

### Template files mislead

A service template ships with a sample OpenAPI file, and most services built
from the template still carry it unchanged: a handful of example paths, none
of them served, next to the real routes the router serves. A tester handed
that file tests endpoints that do not exist and misses the ones that do. A
placeholder spec is worse than none, so the extractor flags one: an OpenAPI
file none of whose paths the router serves is reported as a placeholder, and
the route-table gate refuses it.

### Moving every handler at once

Putting every working handler of a service onto a generated server interface
in one change puts a rewrite in front of the next release, and the rewrite
loses. New endpoints go spec first from the day the gate is on. Existing
handlers move service by service, each when it is next touched, behind the
route-table gate and its shrinking allowlist.

### Typing data changes into a live system

A new role and its permissions, a backfill, a correction: each was run by
hand against production at some point. They are safer emitted from the spec
as a guarded script, with preconditions that raise, a dry run by default,
postconditions that check exact counts and no effect on a second run. That
emitter reads the `guard` on an operation or a command.

## What extraction asked of the meta-model

Extracting real systems asked for four concepts, each one that only the
validator or an emitter can check, because a keyword that no tool checks
is a defect.

- Trace, built: every named object carries `satisfies`, the requirements
  it meets, and every test `verifies`. Fields, parameters, responses,
  actions and worked examples trace through the object that holds them.
- Guard, built: a precondition and an exact record count on a data change,
  on an operation or a command; its emitter is the guarded script above.
- Separation of duties, meta-model 0.2: sets of permissions that one
  holder must never have together; the validator checks that no role
  grants them (`docs/meta-model-0.2.md`, step 1).
- Workflows, meta-model 0.2: a request that finishes after people approve
  it (trigger, form, steps) as an object of its own, not an annotation on
  an endpoint (`docs/meta-model-0.2.md`, step 3).
