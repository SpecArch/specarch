# Caller context: the application, the calling service, the delegation chain and the audience

An authorisation decision has more inputs than the role, the path and the
method. The person, known by the token's `sub`, and the roles they hold
are what the meta-model says today (`docs/meta-model-0.3.md`). Four more
inputs came from an estate whose services call each other on a person's
behalf: the application the person signed in through, as a condition on a
grant; the service making a machine-to-machine call, with grants of its
own; the chain of services a call passed through on the person's behalf,
with rules over the chain; and the audience of each token, checked at
every hop so that a token minted for one service is not replayed at the
next. This file is the design: a keyword for each, where each sits by the
interface boundary's test, how the readers find each in code, the red
cases each derives, what the validator checks, what the technical
specification shows, and what stays out with the reason. Nothing here is
built; the owner's questions are at the end, the recommendation first in
each.

Three tests are applied, as `docs/estate.md` applies them: the compiler
test (`docs/principles.md`), the acceptance test (`docs/acceptance.md`),
and the readers' test (ADR-044, ADR-050, ADR-075): a reader writes
nothing the code does not say, and the rules of `extract` stay in Go.

Standards first. OAuth 2.0 (RFC 6749) calls every registered program a
client, confidential when it can keep a credential and public when it
cannot (section 2.1), and says a client identifier is not a secret
(section 2.2) and that the authorisation server must not rely on a public
client's authentication to identify it (section 2.3). A machine calling
on its own account uses the client credentials grant, which only a
confidential client may (section 4.4). The JWT access token profile (RFC
9068, section 2.2) requires `aud`, `sub` and `client_id` in every access
token, and its section 4 says the resource server must validate that
`aud` holds a value it identifies itself with, and must reject the token
otherwise; RFC 7519, section 4.1.3, says the same of every JWT. Token
exchange (RFC 8693) is how a service obtains a token to call the next one
on a person's behalf: its `act` claim "provides a means within a JWT to
express that delegation has occurred", "a chain of delegation can be
expressed by nesting one `act` claim within another", "the outermost
`act` claim represents the current actor while nested `act` claims
represent prior actors", and "the least recent actor is the most deeply
nested" (section 4.1); its `client_id` claim "carries the client
identifier of the OAuth 2.0 client that requested the token" (section
4.3); the exchange request names the next service with `audience` or
`resource` (section 2.1), the resource indicator of RFC 8707, which the
issuer puts in `aud` (RFC 9068, section 3). The OAuth 2.0 Security Best
Current Practice (RFC 9700, section 2.3) asks that access tokens be
audience-restricted to one resource server, which must refuse a token
meant for another. OpenID Connect Core's `azp` is a claim of the ID
token, "the party to which the ID Token was issued", and the connect
specification itself discourages its use; the access token's word for the
application is RFC 9068's `client_id`, so that is the claim this design
reads.

## Where each input stands

| Input | Standing |
|---|---|
| 1. the person, `sub` | 0.3, step 4: the `sub` claim fills the caller |
| 2. the person's roles | 0.1: `roles` and `permission`, fail-closed |
| 3. the application the person signed in through, as a condition on a grant | designed below: `clients` of `kind: application`, and `through` on a role; checked against `client_id` where the person's own token is accepted |
| 4. the calling service, with grants of its own | 0.3, step 3 (ADR-096): a client holds roles; this design adds `kind: service`, a client with no roles that only acts for people, and the permission table's `client` on a grant; the skipped check is the `bypassedFor` gate of `docs/estate.md`, item 7 |
| 5. the delegation chain, with rules over it | designed below: the `act` claim as RFC 8693 registers it, in place of 0.3's `actor` string, and `via` on a token and on an operation; two confused-deputy red cases |
| 6. the audience per hop | designed below: `aud` with `const` or `enum` on every token a service accepts, required, and the replay red case |

The left-out row of 0.3 "the roles or the client in an expression" stays
left out, with a revised reason, and the row "a route both people and
clients call" is the estate's question 3, which this design needs
answered (For the owner to decide, 5).

## 3. The application the person signed in through

A role may delete invoices only from the administration application, not
from the phone. The role still grants the permission; the application
narrows where the grant holds. The application is a registered OAuth
client, and so is the service of input 4; RFC 6749 has one word for both,
and so does the meta-model: `clients`, the object ADR-096 declares. What
tells them apart is what each does, said by `kind`:

```yaml
clients:
  admin-web:
    description: The administration application in the browser, where staff sign in.
    kind: application
  mobile:
    description: The application on a member's phone.
    kind: application
  reporting:
    description: The reporting service, which reads invoices on its own account every night.
    kind: service
    roles: [invoice-reader]
    proof:
      kind: token
      token: machine
  bff:
    description: The service behind the front ends, which calls the middleware on the person's behalf.
    kind: service
  middleware:
    description: The service between the front ends' backend and the core, which calls the core on the person's behalf.
    kind: service
```

An `application` is where a person signs in: a person's own token carries
its name in `client_id`. A `service` calls on its own account, with
`roles` and a `proof`, or on a person's behalf as an actor in a chain
(input 5), or both. `roles` and `proof` become optional: a client with
`roles` must have a `proof`, because a client that calls on its own
account must prove itself (RFC 6749, section 4.4, the client credentials
grant is for confidential clients only); a client without `proof` is a
public client, whose name in a token is the issuer's word and no proof of
origin (RFC 6749, section 2.3). A `service` with no roles that no chain
names is a warning, `client_unused`, since nothing can reach it. An
application may hold roles with a proof, a kiosk that calls on its own
account, which is rare and allowed.

The condition is written on the role, keyed by permission, beside `rows`,
which narrows a role the same way, keyed by entity:

```yaml
roles:
  invoice-manager:
    description: Staff who manage invoices.
    permissions: [invoices.read, invoices.delete]
    through:
      invoices.delete: [admin-web]
```

`through` maps a permission the role grants, directly or by `implies`
(`docs/estate.md`, point 5), to the clients the grant holds through. A
permission not listed holds through every client. The check is one, at
the operation, command or page that names the permission: the token's
`client_id` is among the listed clients. For a person it is the
application they signed in through; for a client calling on its own
account it is the client itself, so a service that holds the role and is
not listed is refused too. Listing nobody is not written: to close the
grant, drop the permission. The validator refuses `through` naming a
permission the role does not grant, a client not declared, or an empty
list (`through`), and refuses `through` in a specification whose accepted
tokens declare no `client_id` claim and whose session is the only
sign-in, with the reason that a session carries no client identifier
(`through`); a specification that wants the condition declares the claim.

After a token exchange the condition cannot be checked, and the design
says so rather than pretending. RFC 8693, section 4.3, makes `client_id`
the client that requested the token, so at the second hop it names the
first hop, and the application is in no claim of the delegated token. The
check therefore belongs to the service that accepts the person's own
token: the single service of a system with no hops, or the first hop of a
chain, which is the service whose accepted token declares no `act`.
`through` is checked on a token that carries no `act`; a token that
carries one is checked by `via` instead (input 5), and the first hop's
`through` is trusted through the chain, since the chain names which first
hop checked it. A service that accepts both, a direct token and a
delegated one, checks each token by its own rule and never compares the
application against a hop. A specification whose every accepted token
declares `act` and that writes `through` gets the error `through` with
the reason that no token it accepts carries the application. A project
that carries the application past the first hop does so in a claim of
its own and asks for the keyword then (For the owner to decide, 3).

`through` is design by the interface boundary's test: the application
depends on it, since a person using the phone is refused what the same
person is allowed in the browser, and the phone shows or hides the action
by it. The condition is as strong as the client's registration, which the
techspec says beside the matrix: for a confidential client the issuer
checked a credential, for a public one it checked a redirect and a code
verifier (RFC 7636, RFC 8252), and a public client's name narrows the
surface without proving the origin.

The derived red case, for every role with a `through` entry and every
operation, command or page naming that permission: `refused through a
client not listed for <role>`.

Compiler test: a wrong name in `through` is an error at the role, and
the grant stays read, since the permission is still in `permissions`.
Acceptance test: "not from the phone" is one line on the role, and the
matrix shows at once what each application reaches.

## 4. The calling service

A service that calls on its own account is ADR-096 already: a client with
`roles` and a `proof`, whose grants are its roles' permissions, so "the
reporting service may only read" is the `invoice-reader` role on the
`reporting` client and `callers: clients` on the routes meant for
machines. What the estate's code does instead, a machine token that skips
the role check, is the gate `bypassedFor` in the permission table
(`docs/estate.md`, item 7): a `must` question at every operation the check
guards, which is the record until the owners answer whether every machine
was meant to reach it.

What this design adds is the delta:

- `kind: service` with no `roles`, a client that never calls on its own
  account and only carries a person's call (the `bff` and `middleware`
  above). Without the word, the validator could not tell a service that
  lost its roles from one that never had them, and could not derive the
  confused-deputy cases of input 5, which name every service.
- The permission table gains `client` on a grant: a row whose subject is
  a client and not a role, as a policy library's rows name a service
  directly (`{"client": "reporting", "permission": "invoices.read"}`).
  `extract permissions` writes a role per such client, named after it with
  the `could` question to rename, grants it the permissions, and declares
  the client with `kind: service` holding that role and a `must` question
  for its `proof`; a name that is a row's subject and also appears in a
  grant's `client` condition (input 3's form in the table) is a `must`
  question, since one name is then used as a caller and as a condition.
- The machine-token case of the confused deputy: a service that calls a
  route meant for people with its own token, so the call carries the
  service and not the person, and the person's rows, roles and `through`
  are not applied. 0.3 refuses a client at an operation whose `callers`
  is left out; this design names it as a derived red case at every such
  operation once a `service` is declared: `refused for a service's own
  token`.

A route both people and a service call, the reporting service reading
the same list people read, is 0.3's left-out row and the estate's
question 3. This design needs it answered (For the owner to decide, 5).

Compiler test: a client row with no proof is a question, not a dropped
grant. Acceptance test: the matrix shows the services beside the roles
with what each reaches, which the policy rows never showed.

## 5. The delegation chain

A call from the administration application reaches the core through two
services, each calling the next on the person's behalf: the application
holds the person's own token (`sub` the person, `client_id` `admin-web`,
`aud` the first hop); the first hop exchanges it for a token to call the
middleware (`sub` the person, `client_id` `bff`, `act: { sub: bff }`, `aud`
the middleware); the middleware exchanges that for a token to call the
core (`sub` the person, `client_id` `middleware`, `act: { sub: middleware,
act: { sub: bff } }`, `aud` the core). The core wants to accept invoice
deletes only when the call came that way.

### The `act` claim

0.3 wrote the delegated token with `actor: { type: string }` and a check
`actor != sub`. The standard's claim is `act`, registered by RFC 8693 in
the JSON Web Token Claims registry beside `client_id`, `scope` and
`may_act`, with a nested object as its type. ADR-097 writes the
registered claims of RFC 7519 as the RFC names them, carrying its types
without being retyped, because the registered names are single lower-case
words. The rule widens to the registry: a claim name in the JSON Web
Token Claims registry that SpecArch knows is written as registered, goes
on the wire as registered whatever `info.wireNames` says, since the
registry is the wire, and carries the registering RFC's type. The
validator ships the list it knows: `iss`, `sub`, `aud`, `exp`, `nbf`,
`iat`, `jti` (RFC 7519); `act`, `scope`, `client_id`, `may_act` (RFC
8693); `cnf` (RFC 7800); `auth_time`, `acr`, `amr`, `azp` (OpenID Connect
Core, named by RFC 9068, section 2.2.1). Any other claim is a camelCase
property under the one wire-name rule, as today, and a name with an
underscore outside the list is refused as it is today.

```yaml
tokens:
  delegated:
    description: What a service presents when it calls on a person's behalf.
    format: jwt
    lifetime: PT5M
    signature: ES256
    claims:
      sub: { description: The person acted for. }
      client_id: { description: The service that requested this token, the last actor. }
      act: { description: The chain of services that acted, the last one outermost. }
      aud: { const: "https://core.example.org" }
```

`act` carries the rule the standard gives it, and the validator derives
it without a check written: `refused when the actor is the subject`, for
every token declaring `act`, at every operation accepting it. 0.3's
`actor-is-not-subject` check goes, because an expression over `act.sub`
would need field access on a claim object, which the expression language
does not have and does not need for a rule the keyword already states.
The nested chain is read by the validator and the generated tests from
the standard's own order: the outermost `act` is the last actor, the
innermost the first.

### `via`: the chains a token is accepted through

A token declaring `act` may name the chains it is accepted through, and
an operation may narrow them:

```yaml
tokens:
  delegated:
    via:
      - [bff, middleware]
```

```yaml
    delete:
      operationId: deleteInvoice
      permission: invoices.delete
      via:
        - [bff, middleware]
```

`via` is a list of chains, each the clients a call passed through in
call order, the first hop first, which the validator maps onto the
nesting in reverse: `[bff, middleware]` is `act: { sub: middleware, act:
{ sub: bff } }`. The empty chain `[]` is a direct call with no `act`, so
an operation that takes a direct call and the chain writes `[[], [bff,
middleware]]`, and one that takes the chain only writes the chain alone.
`via` on the token is what every operation accepting it checks; `via` on
an operation replaces it for that operation, the pattern of the default
token and `token` on an operation (ADR-097). Left out on both, every
chain the issuer signed is accepted, which is trusting the issuer's
`may_act` policy, and the techspec says so at the token. A chain names
declared clients, each once; `via` on an operation whose accepted token
declares no `act`, a client not declared, a chain naming an
`application` after its first element, a client twice in one chain, and
the same chain twice are refused (`via`). Every chain is written out: a
wildcard or a chain of any length would admit a hop added later without
anyone writing it, the fail-open that the estate declined pattern
permissions for.

`via` is design: the hops depend on it, since the middleware must
exchange its token for one naming the core, and the first hop must call
the middleware and not the core. Which middleware function verifies the
chain is the implementation file's binding (The readers below).

### The confused deputy

The failure the chain rule guards against has a name, the confused
deputy: a service with authority of its own is made to use it for a
caller who has less. It takes two forms here, and each is a derived red
case at every operation for people once a `service` is declared, whether
or not `via` is written, because neither depends on the chain rule:

- `refused for a service's own token`: a hop calls on with its own
  machine token (`sub` the service, no `act`), and the person is lost
  with their rows, roles and `through`. Input 4 names it.
- `refused for a person's token requested by a service with no actor`:
  a hop obtains a token with `sub` the person and `client_id` itself,
  with no `act`, which RFC 8693 calls impersonation as against delegation
  (section 1.1); the service then looks like the application and no
  chain is recorded. A `client_id` naming a `service` in a token with no
  `act` is the sign, and it is why `kind` is on the client.

Where `via` is written, a third case per operation: `refused with a chain
not listed`, which the test gives as the chain with one hop missing
(`[middleware]` alone, the first hop skipped).

Compiler test: a chain naming an unknown client is an error at the entry,
and the token is still read with its other claims. Acceptance test: the
chain is one line that reads as the call is drawn, first hop first, and
the techspec's call diagram draws it.

## 6. The audience per hop

Each token in the example above is minted for one service: `aud` names
the first hop, then the middleware, then the core, and each service
checks that `aud` names itself, so a token captured at one hop is
refused at the next. RFC 9068, section 4, makes that check a must; RFC
7519, section 4.1.3, says a JWT whose `aud` does not name the processing
principal must be rejected; RFC 8707 gives the value, a URI naming the
service, and its security considerations (section 3) say why: an
audience-restricted token presented to one resource "cannot then be
taken by that resource and presented elsewhere"; RFC 9700, section 2.3,
obliges every resource server to verify, for every request, that the
token was meant for it, and to refuse the request otherwise.

The keyword exists: `aud` is a claim under `claims`, and JSON Schema's
`const` says its one value, `enum` several. No new word is needed:

```yaml
    claims:
      aud: { const: "https://core.example.org" }
```

The value is a StringOrURI (RFC 7519, section 2): a value with a colon
must be a URI, which the validator checks (`token_audience`). What the
design adds is the rule: every token a specification accepts, the
default and each one an operation names, declares `aud` with `const` or
`enum`; a token accepted without it is the error `token_audience` at the
token, with RFC 9068's reason. The rule holds for `opaque` tokens too,
since the introspection response that stands in for the token carries
`aud` (RFC 7662) and the check protects the same replay. A token the
specification issues and no operation accepts, the identity service's
delegated token minted for other services, declares `aud` as a typed
claim with no `const`, since its value differs per exchange, and the rule
does not reach it. The key is not named `audience` because `docs/estate.md`
gives that word to an element's audience, internal or everyone, and one
word must not mean two things.

The derived red case, at every operation accepting a token: `refused
with a token for another audience`, which the test gives as a token for
the previous hop.

`aud` is design: the client that requests the exchange names the next
service with RFC 8693's `audience` or `resource` parameter, so it must
know the value. The umbrella of `docs/estate.md` (item 6) gains a check
once it exists: two modules whose accepted tokens declare the same `aud`
value are an error at the second module, since the value is what keeps
a token at one hop.

Compiler test: a token without `aud` is an error at the token and the
token is still read, so every operation naming it still validates
against its other claims. Acceptance test: one line per accepted token,
and the techspec names the audience beside each.

## The caller's properties

None of these claims fills a property of `caller`. 0.3 keeps the roles
and the client out of the caller's properties because no rule needs
them in an expression, and that stays true: the application is a
condition on a grant (`through`), the service is a client with roles,
the chain is a rule on the token and the operation (`via`), and the
audience is a value the token declares. Each is a keyword the validator
checks and a generator renders, and none needs a set or a list in the
expression language. The left-out row of 0.3, "the roles or the client
in an expression", keeps its place with this as its reason: the four
inputs that seemed to need the client in an expression are conditions
declared by name.

## Where each keyword sits

| Keyword | Where | Why |
|---|---|---|
| `clients.<name>.kind` | design | the issuer and every hop must know whether a name is where a person signs in or a service that acts |
| `roles.<role>.through` | design | the application shows or hides the action by it |
| `act`, `client_id`, `aud` under `claims` | design | the issuer mints them and the next hop requests them |
| `via` on a token and an operation | design | the hops depend on it to call the right next service |
| which middleware verifies `aud`, `act` and `client_id`, and which setting holds the audience value | implementation file, `bindings` | one stack's choice, as `permissionChecks` is (ADR-081) |
| the resource indicator the issuer is configured with per environment | implementation file, `deployments` | a value per deployment, with the `aud` `const` of the design as what it must equal |

The origin table of `docs/conventions.md` gains `kind` (of a client),
`through` and `via` as SpecArch's, and `act`, `client_id`, `may_act` and
`scope` as RFC 8693's, `aud` and the other registered claims as RFC
7519's, under the row for `claims`.

## The readers

The readers write nothing of this on their own, after 0.3's rule for
rows and tokens: the code shows that a verifier exists, and what it
compares against is a question unless it is a literal.

- `extract permissions`. The permission table's `grants` gain two
  optional keys: `client`, when the row's subject is a client (input 4),
  and `through`, when the row holds through one client only, the extra
  field a policy library's rows carry beside role, pattern and method
  (`{"role": "invoice-manager", "permission": "invoices.delete",
  "through": "admin-web"}`). The reader writes `through` on the role
  from the second, declaring each named client with `kind: application`
  and a `must` question for its description; a role granted one
  permission in two rows, one with `through` and one without, is a
  `must` question at the role, since the rows disagree on whether the
  grant is narrowed. With `rules` (pattern rows, `docs/estate.md`, point
  5) the same field on a pattern row narrows the permissions the row
  yields.
- `extract go`, `extract swift`, `extract javascript` and `extract
  dart`. The implementation file's binding names the project's own
  checks, as `permissionChecks` does (ADR-081), with the argument that
  holds the names: `clientChecks` (the function that checks `client_id`,
  and `clientArgument`), `chainChecks` (the function that checks `act`,
  and `chainArgument`) and `audienceChecks` (the function that checks
  `aud`, and `audienceArgument`). A client check on a route whose
  permission no other route names writes `through` on every role
  granting that permission; on a permission other routes name too it is
  a `must` question at the operation, since the condition would then
  narrow a grant elsewhere. A chain check with literal names writes `via`
  on the operation; one reading its names from a setting is a `must`
  question naming the setting. An audience check against a literal writes
  `aud` with `const`; against a setting, which it nearly always is, a
  `must` question at the token naming the setting, which the
  implementation file's `deployments` answer. A verifier the bindings do
  not name is what it is today: a `must` question at the operation naming
  the middleware.
- `extract go` on a token exchange: a handler that mints a token with an
  `act` claim is the exchange operation, and the claims found where the
  token is minted stay the question 0.3's step 5 asks, listing them, now
  with `act` and `client_id` among the names.

## The validator

Each rule is made in both builds with the same output and a case in
`spec/tests/`.

| Rule | Refuses |
|---|---|
| `client` | a client with `roles` and no `proof`; a `kind` other than `application` and `service` |
| `client_unused` (warning) | a `service` with no roles that no `via` chain names |
| `through` | a permission the role does not grant, directly or by `implies`; a client not declared; an empty list; `through` where no accepted token declares `client_id` and the session is the only sign-in; `through` where every accepted token declares `act`, since it is checked only on a token with no `act` |
| `claim` | a registered claim retyped; an underscore name outside the registry list |
| `via` | on an operation whose accepted token declares no `act`; a client not declared; an `application` after a chain's first element; a client twice in one chain; one chain twice |
| `token_audience` | an accepted token without `aud` as `const` or `enum`; a value with a colon that is not a URI; in an estate, two modules accepting one audience value |
| `callers` | as 0.3 has it; a third value follows the owner's answer to question 5 |

## The technical specification

The permissions section keeps its one matrix, roles by permissions, and
the cell says the clients when `through` narrows the grant, otherwise a
mark as today:

| Role | invoices.read | invoices.delete |
|---|---|---|
| invoice-manager | yes | admin-web only |
| invoice-reader (reporting) | yes |   |

A client with roles is a row of the matrix, named with its kind, so the
matrix is role by permission with the application in the cell, and the
callers are the rows: the three dimensions the owner asked for, without a
table per combination. Under it, a table of clients: name, kind, roles,
proof, and for a public client the note that its name is the issuer's
word. Then a table of the tokens accepted: name, audience, the chains
accepted through, and the operations that name a chain of their own.
The call diagram draws each chain as the hops in order with the token's
audience on each arrow. Separation of duties counts a client's roles as
it counts a person's.

## Left out

| Candidate | Why it waits |
|---|---|
| the application or the chain in an expression | no rule needs them: each is a condition declared by name, and sets and lists stay out of the expression language (0.3's row, kept) |
| `may_act` (RFC 8693, section 4.4) | the issuer's statement of who may become an actor, checked at the exchange; it belongs to the identity service's specification, as its exchange operation's rule, and the chain rule at the receiving service says the same from the other end |
| `scope` as a second right beside `permission` | a scope is what the person consented to let the client ask for; the permission is what the service checks, and one right is the fail-closed rule. A token that carries `scope` declares it as a claim; mapping scopes to permissions is no one's yet |
| sender-constrained tokens (`cnf`, RFC 7800; DPoP, RFC 9449; mutual TLS, RFC 8705) | a different attack: a token stolen and replayed at the same hop, where the audience check does nothing. RFC 9700 asks for both. `cnf` is declared as a claim today; a keyword for the binding waits for a system that has one |
| a wildcard or a chain of any length in `via` | admits a hop added later without anyone writing it, the fail-open the estate declined pattern permissions for |
| the application past the first hop | no standard claim carries it after an exchange (RFC 8693, section 4.3); a project claim of its own, if a system wants one, is the owner's question 3 |
| `azp` as the application | an ID token's claim, and one its own specification discourages; the access token's word is `client_id` |
| the person's roles differing by application | the roles are the person's; the application narrows a grant, which is what was asked |

## Building

Each item changes `spec/` first, in both validator builds, with its
conformance cases, its decision record numbered when it lands, the
sources it cites added to the root file (`rfc-6749`, `rfc-8693`,
`rfc-9068`, `rfc-8707`, `rfc-9700`, `openid-connect-core`), the techspec
regenerated and a history entry. Items 1 to 3 amend 0.3's steps 3 and 4
before they are built, so each is one item with its 0.3 step; item 4
follows the estate's item 7, and item 5 its item 6.

1. Clients with `kind`, with 0.3 step 3. `kind` on a client, `roles` and
   `proof` optional with the `client` rule, `client_unused`, the matrix
   rows named with the kind. Done when a client with roles and no proof
   and a kind outside the two are refused in both builds, and the
   library's reconciliation client validates with `kind: service`.
2. `through`, with 0.3 step 3. The key on a role, the `through` rule in
   both builds, the cell in the matrix, the derived case, the permission
   table's `through` and `client` on a grant with the reader's questions.
   Done when a role narrowed to one application derives the refusal at
   every operation naming the permission, the table case with a row per
   client gives the client with its role, and the lending desk's matrix
   is unchanged.
3. The claims, `via` and the audience, with 0.3 step 4. The registry list
   in both builds with the `claim` rule; `act` with its derived case in
   place of the `actor` check; `via` on a token and an operation with the
   `via` rule and the three red cases; `token_audience`; the tokens
   table and the call diagram in the techspec. Done when a tree with the
   three-hop example derives the chain, audience and confused-deputy
   cases, each refusal of the `via` and `token_audience` rules has a red
   case with the same output in both builds, and a 0.2 file is read with
   the version warning.
4. The readers, with 0.3 step 5 and the estate's item 7. `clientChecks`,
   `chainChecks` and `audienceChecks` in the implementation schema, the
   four code readers writing `through`, `via` and `aud` from literals
   and asking otherwise, the permission table's two keys. Done when the
   lending desk's sources extract the same twice and a fixture with a
   literal chain check gives `via`.
5. The estate's audience check, after the estate's item 6: two modules
   accepting one `aud` value refused at the second, and a `via` chain
   whose names resolve across modules as references do.

## For the owner to decide

The recommendation is first in each.

1. **An application is a client with `kind: application`, in the one
   `clients` object**, or a separate `applications` object. Recommended:
   one object, because OAuth has one word for both, a service the person
   signs in through (a confidential front-end backend) would otherwise be
   declared twice, and `kind` is the one fact that tells them apart.
2. **`through` on the role, keyed by permission**, or on the permission
   for every holder, or on the operation. Recommended: the role, because
   the request is per role ("this role may delete only from the
   administration application"), it sits beside `rows`, which narrows a
   role the same way, and the other two forms fold into it; a check on
   a route narrows every role's grant of that permission, which the
   reader writes.
3. **The application is checked where the person's own token is
   accepted, and a later hop narrows by chain**, or a project claim
   carries the application past the first hop, named on the token with a
   key, and every hop checks `through` against it. Recommended: the
   first; it is what the standards give, since `client_id` names the
   last requester, and the chain names which first hop did the check. The
   second is one key when a system asks.
4. **`aud` with `const` or `enum` is required on every accepted token, an
   error**, or a warning. Recommended: an error, because RFC 9068, section
   4, is a must and a token accepted for any audience is the replay the
   whole input exists to refuse.
5. **A route both people and a service call is `callers: all`, reported
   in the techspec as every client with a granting role reaching it**,
   the estate's question 3 answered the second way, or it stays left out
   and such a route is two operations with one handler. Recommended:
   `all`, because the reporting service reads the list people read, and
   an estate of services on one list contract would otherwise write every
   list twice, which the OpenAPI document cannot say on one path.
6. **The two confused-deputy cases are derived at every operation for
   people once a service is declared**, or only where `via` is written.
   Recommended: everywhere, because neither failure depends on the chain
   rule, and a system with one service and no chain has the first case
   already.
7. **`via` on the token as the default and on an operation to narrow**,
   or on the operation only. Recommended: both, the pattern of the
   default token, because a service accepts one chain nearly everywhere
   and the exceptions are what is worth writing.
