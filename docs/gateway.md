# A gateway that forwards by a token claim: the element, its table, and the constants the Go reader resolves

A gateway in front of an estate's services registers routes of its own
only for its health and its metrics. Every other request, whatever its
method and path, it verifies the bearer token of, reads one claim, the
audience, looks the value up in a table of services and forwards the
request as received to the service the value names. Specifying such a
service with SpecArch showed three things it could not say or read. The
meta-model has no element for the forwarding: an entry under `paths` is
one method on one path with a permission and a handler, a path parameter
holds one segment, and the readers set a catch-all aside with a line, so
the one behaviour the service exists for was missing from its
specification, and the table of services came in through `extract
outline` and was listed by `gaps` as a surface not read. No reader reads
that table. And `extract go` asks a must question at a route registered
with `Match` when the method list is a package-level list, and at any
route whose path is a package-level constant, where Go's constant is as
fixed as the literal it stands for.

This file is the design: the standards consulted, the decision for each
of the three with its decision record, what stays out and why, and the
build items in order. Nothing here is built. The decision records,
ADR-141 to ADR-143, are proposed until the owner approves this design;
the item that builds each sets its record accepted. The words follow the
diagram keywords of `docs/diagrams.md`: a forwarded service is a
dependency with ADR-134's `kind` and `protocol`, the issuer of the tokens
is a dependency of kind `identityProvider`, and the host of each in an
environment is a server of the implementation file's deployment
(ADR-136). A role carries no kind (ADR-135): who may call is said by
`callers` and `clients` (`docs/caller-context.md`), as everywhere else.

Three tests are applied, as `docs/diagrams.md` applies them. The
compiler test (`docs/principles.md`): does it keep an incomplete or wrong
definition readable, with every problem in the one problems file and
marked at the entry, and does a file without the new keys read and render
as before? The acceptance test (`docs/acceptance.md`): does it make a
result fast to show, and a remark from someone looking at it cheap to
take back into the specification? The readers' test (ADR-044, ADR-050,
ADR-075): a reader writes nothing the code does not say, and the rules
stay in Go.

## Where each ask stands

| Ask | What is missing | Decision |
|---|---|---|
| 1. an element for a gateway that forwards by a token claim | a catch-all that picks its target from a claim has no place under `paths`, and the token check before the lookup has no place at all | `gateways`, a section of the design: the token it verifies, the claim it reads, the dependency each value selects, the refusals it answers itself, and no permission (ADR-141) |
| 2. a reader for the table of services | the table is a file of the project's own, read by nothing | the gateway table, `gatewayTable: 1`, which the project prints from what the gateway loads and `extract gateway` reads; `specarch check` compares it (ADR-142) |
| 3. `extract go` and `Match` with constants | `Match` is read with a literal list only, and a path that is a package-level constant is read as nothing, in both cases a must question; a fallback handler is read as nothing at all, with no question and no line | a package-level string constant is the literal it names, a package-level list assigned once is a literal list, and a fallback handler or a catch-all route is a must question (ADR-143; BUG-028 and BUG-029) |

## Standards first

The rule for new keywords (`docs/conventions.md`) is to take a standard's
word with its exact meaning where one exists, and to say why where none
does. The standards of a forwarding service and of the token it reads,
and what each gives:

- **RFC 9110, HTTP Semantics**, section 3.7, defines the intermediaries:
  a *proxy* the client chooses, a *gateway*, "a.k.a. reverse proxy", an
  intermediary that acts as an origin server for the outbound connection
  but translates received requests and forwards them inbound to another
  server or servers, and a *tunnel*. It fixes the directions too: every
  message flows from upstream to downstream, so the origin is downstream
  of a request and upstream of a response, and *inbound* is toward the
  origin server. Section 7.6 says what a gateway does to a message it
  forwards: it removes the hop-by-hop fields the `Connection` header
  names, adds `Via` to the request it forwards inbound, and passes the
  rest on.
- **RFC 7519, JSON Web Token**, names the claims. Section 4.1.3: `aud`
  identifies the recipients the token is intended for; each principal
  that processes the token must identify itself with a value in it or
  reject the token; in the general case `aud` is an array of strings,
  and a single string when there is one audience. Section 4.1.1: `iss`
  identifies the issuer. Section 2: a StringOrURI is any string, and one
  that contains a colon must be a URI.
- **RFC 8707, Resource Indicators**, section 2, gives the audience value
  its form: an absolute URI naming the resource the token is for, with no
  fragment and, preferably, no query. Section 3 says why a token names
  one resource: a resource that receives it cannot present it elsewhere.
- **RFC 9068, the JWT profile for access tokens**, section 4, has the
  resource server validate that `aud` holds a value it expects for
  itself, and section 2.2 requires `iss`, `exp`, `aud`, `sub`, `client_id`,
  `iat` and `jti`.
- **RFC 7517, JSON Web Key**, section 5, defines the JWK Set a verifier
  reads its keys from, and **RFC 8414, Authorization Server Metadata**,
  section 2, says where it is found: `jwks_uri` in the issuer's metadata,
  under the issuer identifier, which is a URL. **RFC 7518** names the
  signature algorithms (`RS256`, `ES256`, `EdDSA`), which the meta-model
  already takes as a token's `signature` (ADR-097). **RFC 8725, JWT Best
  Current Practices**, section 3.1 has the verifier check the algorithm it
  expects and section 3.9 the audience.
- **RFC 9457, Problem Details**, is the form of every refusal, as the
  catalogue under `errors` already has it.
- **OpenAPI 3** has no catch-all: a path template's parameter holds one
  segment, and a path item is a fixed template; `servers`, which would say
  where a request goes, is excluded from a design file. AsyncAPI has
  nothing of this.
- **The Kubernetes Gateway API** writes an `HTTPRoute` as rules whose
  matches are a path, headers, query parameters and a method, and whose
  `backendRefs` name the services; it has no match on a claim. **Envoy's
  JWT authentication filter** writes a provider with an `issuer`, its
  `audiences` and a `remote_jwks`, and routes by a claim only by copying
  it into a header first. **NGINX**, **Caddy** and Envoy call the server
  behind them the *upstream*, **HAProxy** and the Gateway API the
  *backend*. These are products' words.

Taken: `gateway` (RFC 9110), `claim` and the claim names (RFC 7519), the
audience value's form (RFC 8707, through ADR-109's `token_audience`),
`signature` (RFC 7518, through ADR-097), the refusals' form (RFC 9457,
through ADR-131). Not taken, with the reason at the key: `upstream`, whose
RFC 9110 meaning is a direction that changes with the message; `backend`
and `backendRefs`, a product's words; `audience`, which the meta-model
gives to an element's readership (ADR-119); `issuer` as a key, since the
token's `iss` claim says it; `proxy`, which is a server's kind (ADR-136),
the box in front of a service and not the service.

## 1. The element: `gateways`

A gateway is a design element because every client of the interface must
know it: which token to present, which audience to ask the issuer for
(RFC 8707's `resource` parameter), and what the gateway answers when the
token or the audience is wrong. It is declared once, under `gateways`,
keyed by a camelCase name as a middleware or a job is:

```yaml
gateways:
  api:
    description: Forwards every request no path of this specification answers, as received, to the service the token's audience names.
    token: serviceAccess
    claim: aud
    forwards:
      - { value: "https://loans.library.example", dependency: loans }
      - { value: "https://catalogue.library.example", dependency: catalogue }
    calls: [identity]
    middlewares: [audit-log]
    refusals:
      - { status: 401, problem: token-invalid, condition: The token is missing, is not signed by the issuer's key, or has expired. }
      - { status: 404, problem: service-unknown, condition: The audience names no forwarded service, or names two. }
      - { status: 502, problem: service-failed, condition: The forwarded service did not answer. }
      - { status: 504, problem: service-timed-out, condition: The forwarded service did not answer within its time limit. }

tokens:
  serviceAccess:
    description: The access token a client holds for one of the forwarded services, minted by the identity provider.
    format: jwt
    signature: RS256
    lifetime: PT15M
    claims:
      iss: { const: "https://identity.library.example" }
      aud: { description: The service the token is for, one of the gateway's forwards. }
      sub: { description: The person or the client the token was minted for. }

dependencies:
  identity:
    description: The identity provider whose keys sign the tokens the gateway verifies.
    timeout: PT2S
    kind: identityProvider
    protocol: https
  loans:
    description: The lending desk, which lends the books and takes them back.
    timeout: PT10S
    kind: system
    protocol: https
    specification: lending-desk
    version: 0.1.0
  catalogue:
    description: The catalogue service, which keeps the book records.
    timeout: PT10S
    kind: system
    protocol: https
    specification: catalogue-service
    version: 0.1.0
```

Of these, `description`, `token`, `claim` and `forwards` are required;
`calls`, `middlewares` and `refusals` are not.

**`token`** names, by its key under `tokens`, the token the gateway
verifies before it reads anything else: its format, its signature
algorithm and its claims are the token's own (ADR-097, 0.3's step 4),
and `token` is the word an operation uses for the token it accepts, with
that meaning. The signature algorithm is design because the verifier
must refuse any other (RFC 8725, 3.1), and the issuer is design because
the token says it: `iss` with `const`, RFC 8414's issuer identifier, a
URL. Where the keys come from in an environment, the issuer's `jwks_uri`,
is a deployment's fact: the identity provider's host is a server of the
implementation file's deployment that names the dependency (ADR-136),
and the setting the code reads it from is declared under `configuration`
with its value per deployment, as every setting is. The gateway therefore
`calls` the identity provider and declares nothing else about the keys.

**`claim`** names the claim whose value selects the service, one the
token declares under `claims`, RFC 7519's word. The value is read from
the verified token only, so a claim the gateway reads before the
signature is checked is not a form the element has. A claim whose value
is a list, as `aud` may be, selects a service when exactly one of its
values is a forward's; a list naming two forwards is refused, since a
request can go to one service and an ambiguity is never a guess.

**`forwards`** is the table, a list of rows, each a `value`, the claim's
value, and a `dependency`, the key of the dependency the request is
forwarded to. A list rather than a map keyed by the value, because the
values are URIs, which make poor keys for the JSON pointers the problems
file and the questions use, and because a row is a lookup's row and not
an element with a name; the rows of a deployment's `links` are written
the same way (ADR-137). The dependency is one of ADR-134's kind `system`
or `identityProvider`, the two kinds that are an HTTP origin a gateway
can forward to; a database, a broker and a file store are not. The
dependency's `timeout` is the time the gateway waits for the forwarded
service, after which it answers its time-out refusal; its `protocol` is
how the service is reached; its `specification` and `version`, when it
is a module of the estate, are what the technical specification links
the row to. Where the service is in an environment is the implementation
file's: a server of the deployment with `dependency: loans` and its URL
(ADR-136), and the setting whose value the code reads. Two rows with one
value, a row whose dependency is not declared or is of another kind, and
an empty list are the error `gateway` at the entry.

**`calls`** lists, as an operation's `calls` does, the dependencies the
gateway calls besides the ones it forwards to: the identity provider
whose keys it fetches. The forwards are calls as well, and the diagram
and the checks count both; a forward's dependency written under `calls`
too is the error `gateway`, one way to say one thing. Every dependency
forwarded to or called gives the gateway the derived cases a dependency
gives an operation, `dependency fails <name>` and `dependency times out
<name>` (SA-29), critical by nature.

**`middlewares`** lists the middlewares the gateway runs before its own
steps, in order, with ADR-131's rules unchanged: a name not declared is
the error `middleware`, and once the specification declares its
middlewares a gateway that lists none is the warning `middleware_missing`
until it writes `[]`. A project that verifies the token in a declared
middleware lists it here, with the token refusals on that middleware;
`token` still names the token, since a middleware's description cannot
be checked against a format.

**`refusals`** are what the gateway answers itself, in place of the
forwarded service's answer, in the form a middleware's refusals have
(ADR-131): a `status`, a `problem` under `errors` and a `condition`. The
problems are checked as a middleware's are (`problem`). They are written
and not derived because the statuses are the project's choice: one
gateway answers 404 for an unknown audience, so that the service's
existence is not disclosed, and another 403.

**What is forwarded.** Every request that no operation under `paths`
answers, with any method and any path, as received: the method, the
target, the headers but the hop-by-hop ones, and the body (RFC 9110,
7.6). Nothing says so in the file, because `paths` already carves the
space: an operation answers its route, the gateway takes the rest, and
a path the gateway must not forward is an operation that answers it,
closed with the permission `nobody` when nobody may reach it (ADR-096). A
`prefix` or a list of methods on the gateway would be a second way to
carve the same space, and no system asked for a gateway that forwards
under one prefix or some methods; it is left out until one does, and the
shape admits it then.

**No permission.** An operation, a command, a page and a job each name
one permission, fail-closed (SA-5). A gateway names none, and the schema
has no key for one, because the gateway decides nothing: it answers no
request of its own, and the service it forwards to checks the permission
of every path it serves, as it would without the gateway. The gate is
the token, signed by the issuer and naming a forwarded service; a request
with no such token is refused, which is what fail-closed means here, and
the derived cases make it a test. The caller-context's rule that every
hop's route has a permission of its own (`docs/caller-context.md`, the
identity module) is about a hop with routes of its own that forwards a
person's call to the one that answers; a gateway has no routes, and the
request's permission is the forwarded service's at every path. A gateway
is not a hop of the delegation chain either: it presents nothing of its
own, and the service sees the client's token as sent, so `via` at the
service names the client and not the gateway.

**The audience rule.** ADR-109 has every accepted access token declare
`aud` with `const` or `enum` naming the service that accepts it
(`token_audience`), after RFC 9068, section 4. A gateway that forwards
by `aud` accepts tokens minted for the services behind it, and RFC 7519,
section 4.1.3, is met the way a gateway meets it: the principal that
processes the token identifies itself with a value in `aud`, and the
gateway identifies itself with every value in its table, on the
services' behalf, while each service still checks that `aud` names
itself when the request reaches it. The table is therefore the token's
fixed audience, and the token declares `aud` as a typed claim with no
`const` or `enum`: `token_audience` is satisfied by the forwards of a
gateway whose `claim` is `aud`, and `const` or `enum` written on that
`aud` is the error `token_audience`, a second way to say the list, as
`aud` on a Txn-Token is (ADR-109). Such a token is the services' and not
this system's: it is not marked `default`, no operation of this
specification names it with `token`, and the rule `gateway` reports
either. The alternative, writing the `enum` on the claim and checking
that it and the table agree, would write one list twice and give the
validator a disagreement to report instead of a fact to read. A gateway
whose `claim` is another, a tenant's name say, leaves ADR-109 as it is:
such a gateway is the token's audience, `aud` names it with `const`, and
an operation of this system may accept the token too.

**In an estate.** The umbrella run of `docs/caller-context.md` (its
build item 4) refuses two modules that accept one `aud` value on access
tokens, and a gateway's forwards are exactly the audiences of the
modules it fronts, each of which declares `aud` with `const` naming
itself. A token a gateway forwards by `aud` is therefore left out of
that refusal, and the positive fact is checked in its place: when a
forward's dependency names a `specification` and that module is given
in the run, the forward's `value` must be one of the `aud` values the
module's access tokens declare, the error `gateway` at the row in the
umbrella run; when the module is not given, the forward counts as an
unresolved reference in the summary line, as every reference into a
specification not given does (SA-65), and is no error. That turns a
false refusal into the one check a reader cannot do by eye: that the
gateway's table and the services' audiences agree.

**The validator**, in both builds, reports as the rule `gateway` at the
entry: a `token` that names no declared token; a `claim` the token does
not declare; an empty `forwards`; a value written twice; a `dependency`
not declared, or of kind `database`, `messageBroker` or `fileStore`; a
forward's dependency listed under `calls`; and, when the claim is `aud`,
the token marked `default` or named by an operation. `token_audience`
(ADR-109) treats a token a gateway forwards by `aud` as fixed by the
forwards and refuses `const` or `enum` on its `aud`. `middleware`,
`middleware_missing`, `dependency` and `problem` reach a gateway as they
reach an operation. In the umbrella run, `gateway` also reports a
forward whose value is none of the audiences the module its dependency
names accepts, and the one-audience refusal of the caller-context's
item 4 leaves out a token a gateway forwards by `aud`.

**The derived cases** (`docs/test-generation.md`), with the gateway a
subject of tests as a job is (SA-35): the golden `forwarded to
<dependency>` per forward, a request with a token for that service
reaching it as received; `refused with no token`, `refused with a bad
signature` and `denied with expired <token>`, frequent, in the words 0.3
gives a client's signature and a token's expiry; `refused with an unknown
<claim>`, occasional, a token for a service the table does not hold,
which for `aud` is ADR-109's token for another audience; `refused with a
<claim> naming two forwarded services`, rare, for `aud` and for any claim
whose schema is an array; and `dependency fails <name>` and `dependency
times out <name>` for every dependency forwarded to or called, critical.
Each refusal expected is the gateway's own, or a middleware's, by its
condition.

**The documents.** The technical specification's chapter 3 draws the
gateway in the context diagram (ADR-134): an edge from the system to
each forwarded dependency labelled with the gateway's name and the
claim, beside the operations and jobs that call it, and the identity
provider's edge labelled with the gateway among its callers. Chapter 6
gains a section per gateway: the token verified with its format,
algorithm and issuer, the claim, the forwards as a table of value,
dependency and specification, the middlewares, the refusals and the
derived cases, and a sequence diagram of one forwarded request, client
to gateway to identity provider for the keys to the service and back.
The test plan lists the gateway's cases with the operations'. `generate
openapi` writes nothing for a gateway, since OpenAPI has no catch-all,
and prints a line naming it, so the document stays the operations' and
nobody looks for the forwarding in it. The deployment guide and the
handover document say nothing new: each forwarded service's host is a
server of the deployment (ADR-136), and the setting whose value the code
reads is a setting as any other.

**The implementation file.** A package says it implements the gateway
under `implements` (`implements: ["#/gateways/api"]`), so `designRef`
admits `gateways` as ADR-137 has it admit `jobs`; the server that runs
the package runs the gateway, and the link check of ADR-137 counts the
gateway's forwards and calls as it counts an operation's calls, so a
gateway server with no link to a forwarded service's server is the
warning `link_missing`. Which function forwards and which verifies the
token is one stack's choice and stays in the code; naming them on the
http binding, as `permissionChecks` names the permission check
(ADR-081), is what a code reader of the forwarding would need, and waits
for that reader (Left out).

Acceptance test: the gateway's section is the first page the services'
teams read, and a remark on it ("the catalogue is reached under another
audience") changes one row. Compiler test: a gateway with a wrong row is
an error at the row and the section still draws the other forwards; a
file with no `gateways` reads and renders as before, and so does a token
with no gateway.

## 2. The gateway table and `extract gateway`

The gateway loads its table from a file of its own format, which is why
`extract outline` is all that reads it today. A reader of that file would
put the rules of extract in a second place, which the readers' test
forbids, and would read one project's format (ADR-118, the workflow
table's reason). The pattern of the route table (ADR-044), the permission
table (ADR-050) and the workflow table (ADR-118) is taken: SpecArch
defines the format, the project's printer writes it from what the gateway
loads, so what is read is what runs, and `extract gateway` reads it.

**The format**, `gatewayTable: 1`, wrapped by
`tools/gateways/dump-gateway.sh <folder> <dump> <printer> [<argument>...]`
around the project's printer as the other dump scripts wrap theirs, with
the folder's `path`, the `commit` that last changed it and, when the
code is a submodule, the `repository` (ADR-098). The printer writes one
object: `gateway`, the gateway's name as the project calls it; `claim`,
the name of the claim it reads; `environment`, the environment the
addresses were resolved in, or null when the printer ran with the
placeholders unresolved; and `forwards`, one object per row, `value`, the
claim's value, `name`, the project's name for the service or null,
`setting`, the name of the environment variable or setting the address
is read from or null, and `url`, the address as resolved or null. A table
with any other key, or without one of these, is refused, as a route
table is.

**The reader** writes, under `--out`, the gateway named by the table,
`origin: stated`, citing the dump and each row, with `claim` and
`forwards`; `description` is a must question, `token` a must question
blocking it, asking which token the gateway verifies, its format,
algorithm and claims, since the table shows a lookup and not a verifier
(the rule of `docs/caller-context.md`, The readers); `refusals` a should
question. Each row gives a forward and declares its dependency: named by
the row's `name` in camelCase, else by a fixed rule from the value, the
host's first label for a URI and the value itself otherwise, with a could
question to rename it; `kind: system`; `protocol` from the URL's scheme,
or a should question when the URL is null; `description` and `timeout`
must questions blocking each. Two rows that name one dependency are a
must question at the gateway, since the table does not say whether one
service answers two audiences. A row's `setting` is a setting under
`configuration`, named by the camelCase rule `extract go` uses for an
environment variable (`LOANS_URL` is `loansUrl`), `secret: false`, its
schema a string of format `uri` and its description that it holds the
address of the forwarded service, with a should question to confirm it is
no secret, must when its name holds secret, token, key or password, as
`extract go` asks. With `--implementation-out` and an `environment`, the
reader writes an implementation fragment (ADR-121) holding that
deployment's servers, one per row with a URL, `name`, `url` and
`dependency` (ADR-136); without an environment the addresses are not
written and a line says so, since no deployment names where they hold.
A value written twice is refused, as the schema refuses it in the tree.

**`merge`** answers the Go reader's question at the fallback handler
(ADR-143) with the table's gateway, by the rule that a question of one
tree whose every blocked key another tree gives is left out. **`check`**
(SA-72) compares a specification with its gateway table as it compares
the route table: a forward in the specification the table does not hold
is `forward_not_served` at the row, one in the table the specification
does not hold is `forward_not_specified` at the table's row, and a claim
that differs is `claim_differs` at the gateway, each an error the
allowlist may name.

The alternative, a second version of the route table with a catch-all
entry (`"method": "*"`, `"path": "/*"`, the handler and the claim), lost:
a route table's row is one operation with one permission, the readers of
the route table would need a second shape for one row, and the forwards
are not routes. Reading the project's file directly lost for ADR-118's
reason.

## 3. The Go reader: the constants it does not resolve, and the fallback it drops

Read with the 0.7.0 build, a Go module whose router registers `Match`
with a package-level `var` holding a literal list of `net/http` method
constants and a path from a package-level `const`, a `Match` with a
literal list and a constant path, a `Match` with both literal, a `GET`
with a constant path, and a `NoRoute` handler gives two operations and
ten questions: the three registrations through a constant or the
package-level list are must questions, "the reader does not read as a
literal" and "not a literal list", the two literal ones are read, and
`NoRoute` is nothing, no question, no line, not counted among the
registrations.

**Constants** (BUG-028). A package-level `const` of a string type is a
compile-time constant: Go gives it one value, and the syntax gives that
value as surely as a literal's. The reader reads it as the literal it
names, in the same package through any file of it and in another package
of the module through its import, as a handler in another package is
found today, and so a constant written as literals and constants joined
by `+`. A package-level `var` is not fixed, so it is read as a literal
list only when the syntax shows it never changes: its value is a
composite literal of string literals, string constants or `net/http`
method constants, and no statement of the package assigns to the name,
appends to it or takes its address, by the same test the reader applies
to a name a call bound once. Any other name stays the must question it
is, and the question now says what stopped the reader: the second
assignment and its line, or the value that is not a list. Go has no
constant lists, so a method list named by a package-level name is always
a `var`, which is why the second rule exists. The rule holds for every
place a literal path or method list is read: `Match`, gin's and echo's
method calls, gorilla/mux's `Methods`, chi's `Method`, a `ServeMux`
pattern and a group's prefix.

**The fallback handler and the catch-all** (BUG-029). A registration the
table knows on a surface the reader reads is written or asked about,
never dropped (ADR-075). gin's `NoRoute` and `NoMethod`, chi's `NotFound`
and `MethodNotAllowed`, gorilla/mux's `NotFoundHandler` and
`MethodNotAllowedHandler`, echo's `HTTPErrorHandler` and a `ServeMux`
pattern of `/` alone with no method are where a gateway lives in each
library, and where a plain 404 lives too; the reader cannot tell which,
so each is a must question citing the call, or the assignment where the
library's fallback is a field, asking what the handler answers or
forwards and by which claim, counted as a registration so that it shows
in the coverage. The question blocks the stage `design` until the
element exists, because the validator refuses a `blocks` entry that is
no section of the schema (`question_block`, checked with the 0.7.0
build), and blocks `gateways` once the element's item has added the
section, which is when a table can answer it. A route whose path ends
in a catch-all (`/*path`, `{path...}`, chi's `/*`), which prints a line
today, is the other way a gateway is written, under a prefix with `Any`
or `Match`; it is the same must question, naming the prefix and the
methods, in place of the line. Once the question blocks `gateways`, the
gateway table answers it in the merge (ADR-142), and an answer that the
handler answers a problem document is a decision that closes it with
nothing written.

These are reader changes and no meta-model change: they are built before
the element, since a specification with a question at its fallback
handler is already better than one that does not know it has one.

## Where each keyword sits

| Keyword | Where | Why |
|---|---|---|
| `gateways`, with `token`, `claim`, `forwards` (`value`, `dependency`), `calls`, `middlewares`, `refusals` | design | a client must know which token to present, which audience to ask for and what a wrong one is answered with |
| the issuer, as `iss` with `const` on the token; the algorithm, as the token's `signature` | design | the verifier refuses any other, and the client's issuer must be the one (ADR-097, RFC 8725) |
| the identity provider and each forwarded service | design, `dependencies`, with ADR-134's `kind` and `protocol` | what the gateway reaches is the same on every stack |
| the keys' URL and each service's address in an environment | implementation file: a server with `dependency` per deployment (ADR-136), and the value of the setting the code reads | a deployment's facts |
| the package that forwards, the server it runs on | implementation file: `implements: ["#/gateways/<name>"]`, `runs` | one stack's structure and placement |
| the function that forwards and the one that verifies | the code; a binding that names them waits for the code reader of the forwarding | one stack's choice, as `permissionChecks` is |

The origin table of `docs/conventions.md` gains, when built, `gateways`
as SpecArch's after RFC 9110's gateway, `claim` and `value` as SpecArch's
after RFC 7519, `forwards` as SpecArch's, and `token`, `calls`,
`middlewares`, `refusals` and `dependency` on a gateway with the meaning
they have elsewhere.

## The validator

Both builds, each with a case in `spec/tests/` and its design test:

| Rule | Reports |
|---|---|
| `gateway` | a `token` not declared; a `claim` the token does not declare; an empty `forwards`; a value twice; a `dependency` not declared or of kind `database`, `messageBroker` or `fileStore`; a forward's dependency under `calls`; with `claim: aud`, the token marked `default` or named by an operation |
| `token_audience` (extended, ADR-109) | `const` or `enum` on the `aud` of a token a gateway forwards by `aud`; the forwards fix it |
| `gateway`, umbrella run | a forward whose value is none of the `aud` values the module its dependency's `specification` names accepts, when that module is given; unresolved, not an error, when it is not. The umbrella's one-audience refusal leaves out a token a gateway forwards by `aud` |
| `middleware`, `middleware_missing`, `dependency`, `problem` | reach a gateway as they reach an operation |
| `link_missing` (extended, ADR-137) | a server running a package that implements a gateway with no link to a server that is one of its forwarded or called dependencies |

## The documents

| Output | What a gateway adds |
|---|---|
| techspec, chapter 3 | the context diagram's edges to each forwarded dependency, labelled with the gateway and its claim; the gateway among the identity provider's callers |
| techspec, chapter 6 | a section per gateway: token, claim, forwards, middlewares, refusals, derived cases, and the sequence of one forwarded request |
| test plan | the gateway's derived cases, with the operations' |
| `generate openapi` | nothing, and a line naming the gateway |
| `generate tests` | a test per written case |
| deployment guide, handover | nothing new: the servers and settings carry it |
| `extract gateway`, `check` | the tree of section 2; the three differences |

## Left out

- A `prefix` or a method list on a gateway: `paths` carves the space,
  and `nobody` closes a path; no system asked for a partial gateway.
- A `permission` on a gateway: it decides nothing, and a permission it
  checked would be a second check of the forwarded service's.
- An `issuer` key: `iss` with `const` says it, and the identity provider
  is the dependency the gateway `calls`.
- A `jwks` or `keys` key: where the keys are is a deployment's fact
  (RFC 8414's `jwks_uri` under the issuer), in the implementation file.
- The `aud` list written on the token beside the forwards: one list
  twice, and a disagreement to report instead of a fact.
- Routing by a header or a path prefix to several services: the
  Kubernetes Gateway API's shape, and a reverse proxy's job, not this
  system's; the element's shape admits a `header` beside `claim` when a
  system asks.
- Rewriting the path or the host on the way (RFC 9110's translation):
  the forwarded request is as received; a gateway that rewrites is a
  question until a system has one.
- A code reader of the forwarding (`extract go` writing a gateway from a
  `NoRoute` handler that verifies and looks up): it needs the http
  binding to name the verifier and the lookup, as `permissionChecks`
  names the check; the table is the code side until then, and the Go
  reader's question marks the place.
- Reading the project's own table file: ADR-118's reason.
- A gateway as a hop of the delegation chain (`via`): it presents nothing
  of its own, so the service's `via` names the client.

## Building

Each item changes `spec/` first (the design, the schemas where a key is
one, `spec/design/commands/` for a command), both validator builds where
a rule is added, with its conformance cases and design tests, the
documents' designs, the origin table rows and the convention's section,
sets its record accepted, and writes a history entry. These are not
queued: the dispatcher queues them once the owner has approved this
file. The order puts the reader fixes first, since they need no decision
of the meta-model and remove two defects, and the element after the
designs it stacks on: 0.3's step 4 (`tokens`, `token` on an operation,
as `docs/caller-context.md` amends it) and the diagram keywords' items 2
and 4 (ADR-134's `kind` and `protocol` on a dependency, ADR-136's
`dependency` on a server), none built. Until the element is built, the
table stays in `extract outline` and under `gaps` as a surface not
read, which is the right record.

1. **The Go reader's constants and fallbacks** (ADR-143, BUG-028,
   BUG-029). The two rules of section 3 in `internal/extract`, the walk
   paragraph of `spec/design/commands/extract.yaml`, the Go table of
   `docs/reading-code.md` and step 15 of `docs/extraction.md`, a test
   with the fixture of section 3 and the cases around it. Done when the
   fixture gives five operations and no question on their paths or
   methods, a `var` assigned in a second place gives the question naming
   that line, `NoRoute` and a catch-all each give the must question,
   blocking `design`, and `Any` and the plain registrations read as
   today.
2. **The element** (ADR-141, SA-75), after 0.3's step 4 and the diagram
   keywords' items 2 and 4. `gateways` in the design schema, its fragment
   schema and the Swift embed; `gateways` in the implementation schema's
   `designRef`; the rule `gateway`, the extension of `token_audience`,
   and `link_missing`'s gateway leg in both builds with cases; the
   derived cases in `derive` and the rank table of
   `docs/test-generation.md`; the fallback question of item 1 moved to
   block `gateways`; the context diagram's edges and the
   chapter 6 section in `document techspec`, the test plan's rows,
   `generate openapi`'s line; the convention's section and origin rows;
   and the library estate's gateway, a third module of `examples/estate`
   that forwards to the lending desk and the catalogue service by
   audience, with its implementation file placing it, so the estate's
   module map, context and deployment diagrams show it. Done when the
   estate's gateway validates with its token and two forwards, each
   refusal of the rule has a red case with the same output in both
   builds, removing the gateway server's link to the catalogue's server
   warns at the gateway server, `const` on its `aud` is the error, and a
   specification with no gateway renders byte for byte as before.
3. **The gateway table** (ADR-142, SA-76). `tools/gateways/dump-gateway.sh`,
   `extract gateway` with `--implementation-out`, the reader's rules of
   section 2 in `spec/design/commands/extract.yaml`, `merge` leaving the
   fallback question out when a table answers it, `check` with the three
   differences, a dump for the estate's gateway under its sources and a
   CI step that extracts it twice and checks it. Done when the estate's
   dump gives the gateway, two dependencies, two settings and the
   questions of section 2 with byte-identical output twice, merging it
   with the Go reader's tree leaves the fallback question out, a dump
   with a repeated value is refused, and `check` reports a forward
   removed from the specification at the table's row.

## The owner's decision points

1. **A section of its own, `gateways`, with no permission**, after RFC
   9110's word. The alternative, a catch-all operation under `paths`,
   lost: OpenAPI has no catch-all, a path parameter holds one segment, and
   an operation must name a permission the gateway does not check.
2. **The forward table in the design**, value to dependency, with the
   addresses in the implementation file. The alternative, the table in
   the implementation file, lost: a client must know the audiences (RFC
   8707), and what the gateway fronts is the same on every stack.
3. **The forwards fix the token's `aud`** (ADR-109 extended), and the
   `enum` is not written beside them. The alternative, both written and
   compared, says one list twice.
4. **`calls` for the identity provider and `iss` for the issuer**, no
   `issuer` or `keys` key: the keys' URL is a deployment's fact.
5. **No `prefix` and no method list**: `paths` and `nobody` carve the
   space; the shape admits them if a system asks.
6. **The reader rules of section 3**: a package-level constant is a
   literal, a package-level list assigned once is a literal list, and a
   fallback handler or a catch-all is a must question in place of
   silence or a line, built first, as BUG-028 and BUG-029.
7. **A printed gateway table** in SpecArch's format, never the project's
   file, as the route, permission and workflow tables are.
