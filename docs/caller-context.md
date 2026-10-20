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
cases each derives, the security event each refusal writes, what the
validator checks, what the technical specification shows, and what stays
out with the reason. All five items of Building are built, the readers in
`extract go` and `extract permissions`; the owner's decisions are at the
end, each with its decision record (ADR-106 to ADR-111, and ADR-159 for
the readers), which is where the reason for each choice and the
alternative that lost are written, so that every document generated from
the specification carries them.

Three tests are applied, as `docs/estate.md` applies them: the compiler
test (`docs/principles.md`), the acceptance test (`docs/acceptance.md`),
and the readers' test (ADR-044, ADR-050, ADR-075): a reader writes
nothing the code does not say, and the rules of `extract` stay in Go.

Standards first. OAuth 2.0 (RFC 6749) calls every registered program a
client, confidential when it can keep a credential and public when it
cannot (section 2.1), and says a client identifier is not a secret
(section 2.2) and that the authorisation server must not rely on a public
client's authentication to identify it (section 2.3). Dynamic client
registration (RFC 7591) gives a client four names: `client_id`, issued by
the authorisation server per registration (section 3.1); `software_id`,
the publisher's name for the software, the same across its versions;
`software_version`, which changes with each release; and a
`software_statement`, a signed bundle of the metadata whose claims the
server may trust where everything else "MUST treat all client metadata as
self-asserted" (sections 2, 2.3 and 5). The JWT access token profile (RFC
9068, section 2.2) requires `aud`, `sub` and `client_id` in every access
token, and its section 4 says the resource server must validate that
`aud` holds a value it identifies itself with, and must reject the token
otherwise; RFC 7519, section 4.1.3, says the same of every JWT. Token
exchange (RFC 8693) is how a service obtains a token on a person's
behalf: its `act` claim expresses delegation, nested with the current
actor outermost (section 4.1), and its `client_id` claim "carries the
client identifier of the OAuth 2.0 client that requested the token"
(section 4.3). The OAuth 2.0 Security Best Current Practice (RFC 9700,
section 2.3) asks that access tokens be audience-restricted to one
resource server, which must refuse a token meant for another; RFC 8707
gives the audience value, a URI naming the service.

Inside a trust domain the standard the design takes is the OAuth working
group's Transaction Tokens draft (draft-ietf-oauth-transaction-tokens,
version 11 of 30 July 2026, on the standards track, cited as a draft): one
short-lived token, the Txn-Token, minted once by a transaction token
service at the edge and passed unmodified through every workload of the
call (section 12.2), carrying the person (`sub`), the transaction's
identifier (`txn`, RFC 8417's claim), the purpose (`scope`, required),
the workload that requested it (`req_wl`), the trust domain as its
audience (`aud`), the issuer's request context (`rctx`) and the
authorisation context that stays immutable through the chain (`tctx`)
(section 9.2). It travels in the `Txn-Token` header, never in
`Authorization` (section 12), its token type is
`urn:ietf:params:oauth:token-type:txn_token` (section 11.1), and a
workload must not use it to authenticate itself (section 13.12). A call
that leaves the trust domain follows the working group's identity
chaining draft (draft-ietf-oauth-identity-chaining, version 17 of 19 July
2026): a JWT authorisation grant from the caller's own issuer, presented
to the other domain's issuer for an access token of that domain (section
14 of the Txn-Token draft defers to it). Both are drafts; their claim
names are taken at these versions and revisited when they are published.
Security events take RFC 8417's Security Event Token, a JWT whose
`events` member names are URIs and whose `txn` joins related tokens, and
what each must hold follows OWASP ASVS 5.0, chapter V16. A service's
identity to the next service is a workload identity: mutual TLS (RFC
8705), a SPIFFE SVID, or the WIMSE working group's Workload Identity
Token (draft-ietf-wimse-workload-creds-02, 2 July 2026).

## Where each input stands

| Input | Standing |
|---|---|
| 1. the person, `sub` | 0.3, step 4: the `sub` claim fills the caller |
| 2. the person's roles | 0.1: `roles` and `permission`, fail-closed; the roles live in the identity module (ADR-107) |
| 3. the application the person signed in through, as a condition on a grant | designed below: `clients` keyed by `software_id` with `kind: application` (ADR-106), `through` on a role, checked at every hop against `client_id` on an access token and `tctx.application` on a Txn-Token (ADR-108) |
| 4. the calling service, with grants of its own | 0.3, step 3 (ADR-096): a client holds roles; this design adds `kind: service`, `callers` as an explicit subset (ADR-110), and the permission table's `client` on a grant; the skipped check is the `bypassedFor` gate of `docs/estate.md`, item 7 |
| 5. the delegation chain, with rules over it | designed below: one Txn-Token across the domain, `via` on each operation naming its immediate callers, the chain derived by induction, a workload-identity proof on every service in it (ADR-111); the confused-deputy red cases in their Txn-Token forms (ADR-108) |
| 6. the audience per hop | designed below: `aud` with `const` or `enum` on every access token a service accepts, the trust domain for a Txn-Token, and the replay red case; every refusal a security event (ADR-109) |

The left-out row of 0.3 "the roles or the client in an expression" stays
left out, with a revised reason. The row "a route both people and
clients call" is answered: `callers` lists the subset (ADR-110).

## The identity module

Clients, roles and the trust domain are facts of the estate, not of one
service, and the umbrella of `docs/estate.md` lists and pins its modules
without holding any of their design. So they live in one module, the
identity module: a specification like any other, with the exchange
operation that issues the estate's tokens (0.3, step 4), and three
things the other modules reference (ADR-107):

```yaml
trustDomain: https://estate.example.org

clients:
  admin-web:
    description: The administration application in the browser, where staff sign in.
    kind: application
    softwareVersion: ">=1.2.0 <2.0.0"
    softwareStatement: true
  mobile:
    description: The application on a member's phone.
    kind: application
    softwareVersion: ">=3.0.0 <4.0.0"
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
    proof:
      kind: spiffe
  middleware:
    description: The service between the front ends' backend and the core, which calls the core on the person's behalf.
    kind: service
    proof:
      kind: spiffe

roles:
  invoice-manager:
    description: Staff who manage invoices.
    permissions: [core#/permissions/invoices.read, core#/permissions/invoices.delete, middleware#/permissions/invoices.delete, bff#/permissions/invoices.delete]
    through:
      core#/permissions/invoices.delete: [admin-web]
      middleware#/permissions/invoices.delete: [admin-web]
      bff#/permissions/invoices.delete: [admin-web]

securityEvents:
  store: securityLog
  alarm: pager
```

`trustDomain` is the URI that names the trust domain: the `aud` of every
Txn-Token, and the base of every security event's identifier. Every
other module declares the identity module as a dependency with
`specification` and `version`, as the estate's decision 2 has it, and
names its clients and roles by reference: `identity#/clients/bff` in
`via` and `callers`, `identity#/roles/invoice-manager` where a role is
named. A role's `permissions` and `through` name each permission in the
module whose service enforces it, `core#/permissions/invoices.delete`;
permissions stay where they are checked. Three reference targets join
the estate's four: `#/permissions/<name>`, `#/clients/<name>` and
`#/roles/<name>`. A module validated alone counts a permission granted
only by a referenced role as an unresolved reference in its summary
line, beside the open questions, and `specarch gaps` lists it; the
umbrella run resolves every reference and refuses a permission no role
grants, and a `through`, `via` or `callers` naming what the identity
module does not hold (`reference`), at the entry.

Every hop's route has a permission of its own. The front ends' backend,
the middleware and the core each declare `invoices.delete`, and the
role above holds all three: a route with no permission is the schema
error it is today, at every hop, which is the fail-closed rule (SA-5)
applied to a hop that forwards as it is to the one that answers.

Compiler test: a reference the validator cannot resolve alone is a
counted gap and the module is still read. Acceptance test: the identity
module's techspec shows the estate's one client table and one matrix,
and each module's shows which of its permissions each referenced role
reaches.

## 3. The application the person signed in through

A role may delete invoices only from the administration application, not
from the phone. The role still grants the permission; the application
narrows where the grant holds. The application is a registered OAuth
client, and so is the service of input 4; RFC 6749 has one word for both,
and so does the meta-model: `clients`, the object ADR-096 declares. What
tells them apart is what each does, said by `kind` (ADR-106).

### The client's names

`clients` is keyed by the client's `software_id` (RFC 7591, section 2):
the name the publisher gives the software, the same across its versions,
in kebab-case like every key. `through`, `via` and `callers` name it.
The `client_id` is the authorisation server's issue per registration
(section 3.1) and differs per environment, so it is a deployment value of
the implementation file and never a design key:

```yaml
deployments:
  production:
    clients:
      admin-web:
        clientId: 7f3a9c1e-admin
```

An `application` is where a person signs in: a person's own access token
carries its name in `client_id`. A `service` calls on its own account,
with `roles` and a `proof`, or on a person's behalf as a hop in a chain
(input 5), or both. `roles` and `proof` are optional on an application:
a client with `roles` must have a `proof`, because a client that calls on
its own account must prove itself (RFC 6749, section 4.4, the client
credentials grant is for confidential clients only); an application
without `proof` is a public client, whose name in a token is the issuer's
word and no proof of origin (RFC 6749, section 2.3). A `service` always
has a `proof` (The delegation chain, below). A `service` with no roles
that no `via` names is a warning, `client_unused`, since nothing can
reach it. An application may hold roles with a proof, a kiosk that calls
on its own account, which is rare and allowed.

### The version

A client may say `softwareVersion`, the versions it is admitted at: a
lower bound included and an upper bound excluded, each a Semantic
Versioning 2.0.0 version, written `>=1.2.0 <2.0.0`. The version is
checked once, at the edge: the hop that accepts the client's own request,
a person's access token or the client's own proof. It is read from the
claim `softwareVersion` of that token, which the issuer fills from the
registration; a specification with `softwareVersion` on any client whose
edge token declares no such claim is refused (`software_version`). No
later hop checks it: no claim of a Txn-Token carries a version, and the
chain binds every later hop to the edge that did. With
`softwareStatement: true` the version is the publisher's, asserted in
the signed statement the issuer verified at registration (RFC 7591,
section 2.3), and the techspec marks the check as security; without it
the version is self-asserted (section 5) and the check is marked
compatibility, which is what a browser or a phone can offer. A native
application whose installs run different versions registers each
instance, the per-instance registration RFC 8252, section 8.4, names RFC
7591 for; a browser application has the version it was deployed at, and
its registration is updated when that changes. A refusal is 403 with the
problem type the design declares for it, never 426, which RFC 9110
reserves for a server that refuses the current protocol.

The derived red case at the edge, for every client with
`softwareVersion`: `refused with a version outside the admitted range`.

### `through`

The condition is written on the role, keyed by permission, beside
`rows`, which narrows a role the same way, keyed by entity:

```yaml
roles:
  invoice-manager:
    permissions: [core#/permissions/invoices.read, core#/permissions/invoices.delete]
    through:
      core#/permissions/invoices.delete: [admin-web]
```

`through` maps a permission the role grants, directly or by `implies`
(`docs/estate.md`, point 5), to the clients the grant holds through. A
permission not listed holds through every client. The check is at every
hop whose operation, command or page names the permission, against the
application the token carries: `client_id` on an access token, which is
the edge, and `tctx.application` on a Txn-Token, which is every later
hop (ADR-108). For a person it is the application they signed in
through; for a client calling on its own account it is the client
itself, so a service that holds the role and is not listed is refused
too. Listing nobody is not written: to close the grant, drop the
permission. The validator refuses `through` naming a permission the role
does not grant, a client not declared, or an empty list (`through`); and
refuses `through` in a specification whose accepted tokens declare
neither `client_id` nor `tctx` and whose session is the only sign-in,
with the reason that a session carries no client identifier (`through`).
A Txn-Token accepted where any `through` reaches must declare
`tctx.application` (`token`).

`through` is design by the interface boundary's test: the application
depends on it, since a person using the phone is refused what the same
person is allowed in the browser, and the phone shows or hides the action
by it. The condition is as strong as the client's registration, which the
techspec says beside the matrix: for a confidential client the issuer
checked a credential, for a public one it checked a redirect and a code
verifier (RFC 7636, RFC 8252), and a public client's name narrows the
surface without proving the origin.

The derived red case, for every role with a `through` entry and every
operation, command or page naming that permission, at every hop:
`refused through a client not listed for <role>`.

Compiler test: a wrong name in `through` is an error at the role, and
the grant stays read, since the permission is still in `permissions`.
Acceptance test: "not from the phone" is one line on the role, holds at
every hop, and the matrix shows at once what each application reaches.

## 4. The calling service

A service that calls on its own account is ADR-096 already: a client with
`roles` and a `proof`, whose grants are its roles' permissions, so "the
reporting service may only read" is the `invoice-reader` role on the
`reporting` client and `callers` naming it on the routes meant for it.
What the estate's code does instead, a machine token that skips the role
check, is the gate `bypassedFor` in the permission table
(`docs/estate.md`, item 7): a `must` question at every operation the check
guards, which is the record until the owners answer whether every machine
was meant to reach it.

### `callers`

`callers` is a list, the explicit subset of who may call (ADR-110):
`people`, the reserved word for anyone signed in, and clients by name.
Left out, it is `[people]`, as today. Which people is the roles'
business, never `callers'`:

```yaml
    get:
      operationId: listInvoices
      permission: invoices.read
      callers: [people, identity#/clients/reporting]
```

There is no `all`. The validator refuses, as `callers` at the entry, an
empty list, a name that is neither `people` nor a declared client, a
client twice, a `service` with no roles (it only carries a person's call
and never calls on its own account), and a client none of whose roles
grants the operation's permission, directly or by `implies`. ADR-096's
`callers: clients` is written as the list of clients. The derived cases
of ADR-096 follow: `refused for a signed-in person` where `people` is
absent, `refused for every caller not listed`.

### What this design adds

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
  route meant for people with its own proof or token, so the call carries
  the service and not the person, and the person's rows, roles and
  `through` are not applied. This design names it as a derived red case
  at every operation for people once a `service` is declared: `refused
  for a service's own token`.

Compiler test: a client row with no proof is a question, not a dropped
grant. Acceptance test: the matrix shows the services beside the roles
with what each reaches, which the policy rows never showed.

## 5. The delegation chain

A call from the administration application reaches the core through two
services, each calling the next on the person's behalf. The application
holds the person's own access token (`sub` the person, `client_id`
`admin-web`, `aud` the front ends' backend). That backend is the edge: it
checks the person, the version and `through`, requests one Txn-Token from
the identity module's exchange operation, and calls the middleware with
it in the `Txn-Token` header; the middleware checks it and calls the core
with the same token, unmodified. The core wants to accept invoice deletes
only when the call came that way.

### The token's kind

A token declares `tokenType` (ADR-111): `access_token`, the default when
left out, or `txn_token`, the suffix of the OAuth token type identifier.
A `txn_token` has `format: jwt`, is carried in the `Txn-Token` header
and never in `Authorization` (the draft, section 12), which `generate
openapi` writes as an `apiKey` scheme in that header for every operation
accepting it, and declares the draft's claims:

```yaml
tokens:
  transaction:
    description: What a hop presents when it calls the next on a person's behalf, inside the trust domain.
    format: jwt
    tokenType: txn_token
    lifetime: PT5M
    signature: ES256
    claims:
      txn: { description: The transaction's identifier, one for the whole call. }
      sub: { description: The person acted for. }
      scope: { description: The permission the edge operation checked. }
      req_wl: { description: The edge workload that requested the token. }
      tctx:
        properties:
          application: { description: The software_id of the application the person signed in through. }
```

ADR-097's rule for registered claims widens to the JSON Web Token Claims
registry: a claim name in the registry that SpecArch knows is written as
registered, goes on the wire as registered whatever `info.wireNames`
says, since the registry is the wire, and carries the registering
document's type. The validator ships the list it knows: `iss`, `sub`,
`aud`, `exp`, `nbf`, `iat`, `jti` (RFC 7519); `act`, `scope`,
`client_id`, `may_act` (RFC 8693); `cnf` (RFC 7800); `txn`, `events`,
`toe` (RFC 8417); `auth_time`, `acr`, `amr`, `azp` (OpenID Connect Core,
named by RFC 9068, section 2.2.1); and, while the Txn-Token draft is a
draft, its `req_wl`, `rctx` and `tctx`, under the same rule, with the
note that the registration is pending. Any other claim is a camelCase
property under the one wire-name rule, as today, and a name with an
underscore outside the list is refused as it is today. `aud` is not
written on a `txn_token`: its audience is `trustDomain` (input 6).

What the Txn-Token carries, and what each hop checks, is ADR-108: `sub`
is the person; `scope`, which the draft requires, holds the permission
the edge operation checked, in that module's wire form, and each later
hop checks that it is the permission of an edge operation whose chain
reaches it; `req_wl` is the edge workload; `txn` is the transaction's
identifier, the same in every security event of the call; and
`tctx.application` is the application, immutable from the edge on, which
every hop checks `through` against. The identity module's exchange
operation issues it with the token type the draft registers, and its
response schema is the token (0.3, step 4).

A call that leaves the trust domain follows identity chaining: the hop
at the boundary obtains a JWT authorisation grant from its own issuer and
presents it to the other domain's issuer for an access token of that
domain, which may carry `act` as RFC 8693 registers it (the outermost the
current actor, the least recent the most deeply nested, section 4.1). A
Txn-Token is never presented outside its domain, and `act` nested inside
a domain is not used. `act` keeps the rule the standard gives it, derived
without a check written: `refused when the actor is the subject`, for
every token declaring `act`, at every operation accepting it. 0.3's
`actor-is-not-subject` check goes, because an expression over `act.sub`
would need field access on a claim object, which the expression language
does not have and does not need for a rule the keyword already states.

### `via`: the immediate callers of an operation

`via` is written on an operation only: the list of the clients that may
be its immediate caller on a person's behalf, each a `service`. It is
required on every operation whose accepted token is a `txn_token` or
declares `act`, and missing there is the error `via`; on an operation
accepting neither it is refused:

```yaml
# the middleware's specification
    delete:
      operationId: deleteInvoice
      permission: invoices.delete
      token: transaction
      via: [identity#/clients/bff]
```

```yaml
# the core's specification
    delete:
      operationId: deleteInvoice
      permission: invoices.delete
      token: transaction
      via: [identity#/clients/middleware]
```

There is no default on the token and no "left out means every chain the
issuer signed". The validator derives the chain by induction across the
estate: for each client in an operation's `via`, the operation of that
client's module that calls this one (the `calls` of its dependency) must
either accept a `txn_token` with a `via` of its own or be the edge,
accepting an access token; a hop whose `via` does not name the caller
that calls it, or a caller with no `via` where one is required, is a gap,
the error `via` at that hop. The middleware's delete lists the front
ends' backend, the core's lists the middleware, and the chain `bff,
middleware, core` is what the induction finds and the techspec draws.
The module of a client named in `via` is the module whose `info.name` is
the client's name: the client `bff` is the module `bff`, so the
induction needs no key beyond the two names the estate already has.
The validator also refuses an empty list (a route nobody may call is
`nobody`), a client not declared, an `application`, a client twice, and
a client without a workload-identity proof (`via`). Every caller is
written out: a wildcard would admit a hop added later without anyone
writing it, the fail-open the estate declined pattern permissions for.

Every client a `via` names declares a `proof` of a workload-identity
kind: `mutualTls` (RFC 8705), `spiffe` (an X.509 or JWT SVID carrying
the SPIFFE ID) or `wimse` (the Workload Identity Token of the WIMSE
drafts, bound to a key the workload proves it holds); a `token` proof
there is refused, and a proof of kind `token` naming a `txn_token` is
refused anywhere (`proof`), since a workload must not authenticate
itself with a Txn-Token (the draft, section 13.12). `via` binds to that
proof: the hop checks that the immediate caller's workload identity is
the one the implementation file's `deployments` map for the named client
(its SPIFFE ID, its certificate, or its Workload Identity Token's
subject, per environment), and that the Txn-Token's `req_wl` is the edge,
not the caller.

`via` is design: the hops depend on it, since the middleware must call
the core and not be called by the application directly, and each service
must present the identity the next one expects. Which middleware function
verifies the caller and the token is the implementation file's binding
(The readers below).

### The confused deputy

The failure the chain rule guards against has a name, the confused
deputy: a service with authority of its own is made to use it for a
caller who has less. Each case is a derived red case at every operation
for people once a `service` is declared, whether or not `via` is written,
because neither depends on the chain rule, and each is a security event
(ADR-108, ADR-109):

- `refused for a service's own token`: a hop calls on with its own proof
  or machine token and no Txn-Token, and the person is lost with their
  rows, roles and `through`. Input 4 names it.
- `refused for a person's token requested by a service with no actor`:
  on an access token, `client_id` naming a `service` in a token with no
  `act`, which RFC 8693 calls impersonation as against delegation
  (section 1.1); it is why `kind` is on the client. On a Txn-Token it
  takes two forms, each its own test: `req_wl` naming a workload that is
  not an edge (a later hop requested a transaction of its own), and `txn`
  differing from the one the hop received (a hop minted a new
  transaction instead of passing the token on unmodified, the draft,
  section 12.2).

Where `via` is written, a third case per operation: `refused with a
caller not listed in via`, which the test gives as the edge calling the
last hop directly.

Compiler test: a `via` naming an unknown client is an error at the
operation, and the operation is still read with its permission.
Acceptance test: `via` is one line per operation that reads as the call
is drawn, and the techspec's call diagram draws the chain the induction
found with the token on each arrow.

## 6. The audience per hop

An access token is minted for one service: `aud` names the front ends'
backend, which checks that `aud` names itself, so a token captured there
is refused anywhere else. RFC 9068, section 4, makes that check a must;
RFC 7519, section 4.1.3, says a JWT whose `aud` does not name the
processing principal must be rejected; RFC 8707 gives the value, a URI
naming the service, and its security considerations (section 3) say why:
an audience-restricted token presented to one resource "cannot then be
taken by that resource and presented elsewhere"; RFC 9700, section 2.3,
obliges every resource server to verify, for every request, that the
token was meant for it.

The keyword exists: `aud` is a claim under `claims`, and JSON Schema's
`const` says its one value, `enum` several. No new word is needed. The
rule is per token kind (ADR-109): a token accepted without a fixed
audience is the error `token_audience` at the token, and what fixed means
depends on `tokenType`:

- an `access_token` declares `aud` with `const` or `enum` naming the
  service that accepts it. The rule holds for `opaque` tokens too, since
  the introspection response that stands in for the token carries `aud`
  (RFC 7662). A token the specification issues and no operation accepts
  declares `aud` as a typed claim with no `const`, since its value
  differs per exchange, and the rule does not reach it;
- a `txn_token` declares no `aud`: its audience is the identity module's
  `trustDomain`, one value for every hop in it, by the draft's rule that
  `aud` "MUST identify the Trust Domain"; `aud` written on one is refused
  as a second way to say it. A Txn-Token captured at one hop and replayed
  at another is refused not by its audience but by the workload identity
  of the immediate caller, which `via` binds to the caller's proof.

The value is a StringOrURI (RFC 7519, section 2): a value with a colon
must be a URI, which the validator checks (`token_audience`). The key is
not named `audience` because `docs/estate.md` gives that word to an
element's audience, internal or everyone, and one word must not mean two
things. The umbrella's check that two modules do not accept one `aud`
value runs over access tokens only, since every module of a domain
accepts the one Txn-Token.

The derived red case, at every operation accepting an access token:
`refused with a token for another audience`, which the test gives as a
token for another service.

`aud` is design: the client that requests the exchange names the next
service with RFC 8693's `audience` or `resource` parameter, so it must
know the value.

Compiler test: a token without `aud` is an error at the token and the
token is still read, so every operation naming it still validates
against its other claims. Acceptance test: one line per accepted token,
and the techspec names the audience beside each.

## Security events

Every refusal of a caller derives a security event with the case
(ADR-109): nobody writes one per case, and a refusal with no event cannot
exist. The event has a fixed name per rule, the identifier URI
`<trustDomain>/events/<rule>`, and a fixed severity:

| Refusal | Rule | Severity |
|---|---|---|
| a token for another audience | `token_audience` | high |
| an immediate caller not listed in `via` | `via` | high |
| the confused deputy, each form | `deputy` | high |
| a caller not in `callers` | `callers` | medium |
| an application not in `through` | `through` | medium |
| a version outside the admitted range, with a software statement | `software_version` | medium |
| a version outside the admitted range, self-asserted | `software_version` | low |
| an expired token (ADR-097's case) | `token` | low |

The identity module declares `securityEvents`: `store`, the dependency
that is the separate append-only store every module writes to, and
`alarm`, the dependency that is the alarm channel; an estate with a
client and no `securityEvents` is refused (`security_events`). Both are
dependencies like any, mapped per environment in the implementation
file. An event is a Security Event Token (RFC 8417): `events` keyed by
the identifier URI, `jti`, `iat`, `toe`, `txn` always, and in the event
the rule, the severity, the operation, the client, `sub`, `req_wl`, the
`jti` of the refused token and the SHA-256 of it; never the token (the
draft, section 15.2; ASVS 16.2.5). An event of severity high, or a
repeat of any event for one client within a window, goes to the alarm
channel too; the count and the window are deployment values of the
implementation file. The validator derives the event beside the red
case, so the generated tests check that the refusal wrote it, and the
techspec lists the events with their severities.

Compiler test: a refusal whose event the project does not write is a
derived red case that fails, never a silent gap, and a missing
`securityEvents` is one error at the identity module with every token
still read. Acceptance test: the techspec's event table is the list a
security reviewer asks for, one line per rule.

## The caller's properties

None of these claims fills a property of `caller`. 0.3 keeps the roles
and the client out of the caller's properties because no rule needs
them in an expression, and that stays true: the application is a
condition on a grant (`through`), the service is a client with roles,
the chain is a rule on the operation (`via`), and the audience is a
value the token declares or the domain gives. Each is a keyword the
validator checks and a generator renders, and none needs a set or a list
in the expression language. The left-out row of 0.3, "the roles or the
client in an expression", keeps its place with this as its reason: the
four inputs that seemed to need the client in an expression are
conditions declared by name.

## Where each keyword sits

| Keyword | Where | Why |
|---|---|---|
| `trustDomain`, `clients`, `roles`, `securityEvents` in the identity module | design | every module and every issuer depends on the same names, domain, store and channel |
| `clients.<software_id>.kind`, `softwareVersion`, `softwareStatement` | design | the issuer and every hop must know whether a name is where a person signs in or a service that acts, and the edge refuses by the range |
| `clients.<name>.clientId` per environment | implementation file, `deployments` | issued per registration, a value per deployment |
| `roles.<role>.through` | design | the application shows or hides the action by it |
| `tokenType`, `txn`, `scope`, `req_wl`, `tctx.application`, `client_id`, `aud` under `claims` | design | the issuer mints them, the edge requests them and every hop checks them |
| `via` on an operation | design | the hops depend on it to call the right next service with the right identity |
| `proof` of kind `mutualTls`, `spiffe`, `wimse` on a service | design | the next service cannot verify the caller without knowing what it presents |
| the workload identity per client (SPIFFE ID, certificate, token subject), the resource indicator, the alarm's repeat count and window | implementation file, `deployments` | a value per deployment, with the design's names as what each must equal |
| which middleware verifies the caller, the token and the application, and which writes the security event | implementation file, `bindings` | one stack's choice, as `permissionChecks` is (ADR-081) |

The origin table of `docs/conventions.md` gains `kind`, `softwareVersion`
and `softwareStatement` (of a client), `trustDomain`, `through`, `via`,
`tokenType` and `securityEvents` as SpecArch's, `act`, `client_id`,
`may_act` and `scope` as RFC 8693's, `txn`, `events` and `toe` as RFC
8417's, `req_wl`, `rctx` and `tctx` as the Txn-Token draft's, `aud` and
the other registered claims as RFC 7519's, under the row for `claims`.

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
  grant is narrowed. A row whose subject is a client gives a role named
  after the client, held by the client declared as a service; where a
  role of the table has that name, the client's role takes the name
  with `-client` added, so that neither holds the other's grants, and
  a `must` question asks whether the client was meant to hold the
  table's role. With `rules` (pattern rows, `docs/estate.md`, point
  5) the same field on a pattern row narrows the permissions the row
  yields.
- `extract go`. The implementation file's binding names the project's
  own checks, as `permissionChecks` does (ADR-081), with the argument
  that holds the names (ADR-159): `clientChecks` with `clientArgument`
  (the function that admits only the clients named, ADR-154's check),
  `applicationChecks` with `applicationArgument` (the function that
  checks the application the person signed in through), `callerChecks`
  with `callerArgument` (the function that checks the immediate caller's
  workload identity), `audienceChecks` with `audienceArgument` (the
  function that checks `aud`) and `eventWriters` (the function that
  writes a security event). The application check has a key of its own
  because a literal name in the code cannot say whether it is an
  application or a service; the client's `kind` says it. A client check
  or a caller check with literal names writes them as the operation's
  `callers` or `via`, with a `must` question for their declarations;
  one reading its names from a setting is a `must` question naming the
  setting. The names are the one argument the binding names, a string
  or a list; strings the call passes after it are a `must` question,
  since they may be more names or a reason, and are written nowhere.
  Two or more literal checks of one kind on a route run in turn, so
  the operation gets the names every one lists, with a `must` question
  where the lists differ. An application check is a `must` question for the `through`
  of every role granting the operation's permission, naming the routes
  that check that permission without it, since the condition would
  narrow a grant there too; an audience check is a `must` question at
  the tokens with its literal or its setting, which the implementation
  file's `deployments` answer; the reader writes no role and no token.
  An event writer is named and nothing is written. A verifier the
  bindings do not name is what it is today: a `must` question at the
  operation naming the middleware. A header read named `Txn-Token` is a
  `must` question for the operation's transaction token and its `via`.
  `extract swift`, `extract javascript` and `extract dart` read
  `permissionChecks` only, as they do for ADR-154's gates.
- `extract go` on a token exchange: a handler that mints a token with a
  `txn` or an `act` claim is the exchange operation, and the claims
  found where the token is minted stay the question 0.3's step 5 asks,
  listing them, now with `txn`, `req_wl`, `tctx`, `act` and `client_id`
  among the names.

## The validator

Each rule is made in both builds with the same output and a case in
`spec/tests/`.

| Rule | Refuses |
|---|---|
| `client` | a client with `roles` and no `proof`; a `kind` other than `application` and `service`; a `service` without `proof`; a version range that does not parse |
| `client_unused` (warning) | a `service` with no roles that no `via` names |
| `software_version` | `softwareVersion` on a client where the edge token declares no `softwareVersion` claim |
| `through` | a permission the role does not grant, directly or by `implies`; a client not declared; an empty list; `through` where no accepted token declares `client_id` or `tctx` and the session is the only sign-in |
| `claim` | a registered claim retyped; an underscore name outside the registry list |
| `token` | a `txn_token` not `jwt`, or missing a claim the draft requires; a `txn_token` accepted where a `through` reaches and declaring no `tctx.application`; `aud` written on a `txn_token` |
| `proof` | a `token` proof naming a `txn_token`; a service in a `via` with a proof that is not `mutualTls`, `spiffe` or `wimse` |
| `via` | missing on an operation accepting a `txn_token` or an `act` token; present on one accepting neither; an empty list; a client not declared; an `application`; a client twice; a client without a workload-identity proof; in the umbrella run, a gap in the chain the induction finds |
| `token_audience` | an accepted `access_token` without `aud` as `const` or `enum`; a value with a colon that is not a URI; in an estate, two modules accepting one audience value on access tokens |
| `callers` | an empty list; a name neither `people` nor a declared client; a client twice; a `service` with no roles; a client none of whose roles grants the permission |
| `security_events` | an estate with a client and no `securityEvents` in the identity module; `store` or `alarm` not a dependency |
| `reference` | in the umbrella run, a `through`, `via`, `callers` or `permissions` entry naming what the referenced module does not hold |

## The technical specification

The identity module's permissions section keeps one matrix, roles by
permissions, and the cell says the clients when `through` narrows the
grant, otherwise a mark as today:

| Role | core invoices.read | core invoices.delete |
|---|---|---|
| invoice-manager | yes | admin-web only |
| invoice-reader (reporting) | yes |   |

A client with roles is a row of the matrix, named with its kind, so the
matrix is role by permission with the application in the cell, and the
callers are the rows: the three dimensions the owner asked for, without a
table per combination. Under it, a table of clients: name, kind,
admitted versions and whether the check is security or compatibility,
roles, proof, and for a public client the note that its name is the
issuer's word. Then a table of the tokens: name, kind, the header it
rides in, its audience or the trust domain, and the operations that
accept it with their `via`. The call diagram draws each chain the
induction found, hop by hop, with the token on each arrow and the proof
each hop presents. A Security events heading lists the events with their
severities, the store and the alarm channel. Separation of duties counts
a client's roles as it counts a person's.

## Left out

| Candidate | Why it waits |
|---|---|
| the application or the chain in an expression | no rule needs them: each is a condition declared by name, and sets and lists stay out of the expression language (0.3's row, kept) |
| `may_act` (RFC 8693, section 4.4) | the issuer's statement of who may become an actor, checked at the exchange; it belongs to the identity module's exchange operation, and `via` at the receiving hop says the same from the other end |
| `scope` as a second right beside `permission` on an access token | a scope is what the person consented to let the client ask for; the permission is what the service checks, and one right is the fail-closed rule. On a Txn-Token `scope` is the edge's permission, which is the same right carried along, not a second one |
| sender-constrained access tokens for people (`cnf`, RFC 7800; DPoP, RFC 9449) | a different attack: a person's token stolen and replayed at the edge, where the audience check does nothing. RFC 9700 asks for both. `cnf` is declared as a claim today; a keyword for the binding waits for a system that has one. Between services the binding is the workload-identity proof |
| a wildcard in `via`, or a chain written whole | a wildcard admits a hop added later without anyone writing it; a whole chain on a token cannot be checked at a hop, since the Txn-Token records no chain, and the induction over each hop's immediate callers checks the same thing from local facts |
| `act` nested inside a trust domain | one Txn-Token per transaction is the draft's design; a chain of exchanges per hop would mint a token per hop with no immutable context |
| `rctx` | the issuer's environmental context, which the draft does not make immutable; nothing of this design reads it |
| `callers: all` | would admit a client added later with a granting role without anyone writing it on the operation (ADR-110) |
| `azp` as the application | an ID token's claim, and one its own specification discourages; the access token's word is `client_id` |
| the person's roles differing by application | the roles are the person's; the application narrows a grant, which is what was asked |
| 426 for an old version | RFC 9110 reserves it for a protocol the server refuses; a version is not the protocol, and 403 with a problem document says why |

## Building

Each item changes `spec/` first, in both validator builds, with its
conformance cases, its decision record cited (ADR-106 to ADR-111 are
written), the techspec regenerated and a history entry. Items 1 to 3
amend 0.3's steps 3 and 4 before they are built, so each is one item
with its 0.3 step; items 4 and 5 need the umbrella (the estate's item 6)
and the permission table's gate (its item 7). The order is the order the
three-hop example needs to validate end to end.

1. Clients by `software_id` with `kind`, version and statement, with 0.3
   step 3 (ADR-106). `kind`, `softwareVersion`, `softwareStatement`,
   `roles` and `proof` optional on an application with the `client` and
   `software_version` rules, `client_unused`, `clientId` per deployment
   in the implementation schema, the matrix rows named with the kind,
   the client table with the admitted versions and the security or
   compatibility mark. Done when a client with roles and no proof, a
   kind outside the two, a range that does not parse and a version with
   no claim to check are refused in both builds, the edge derives the
   version case, and the library's reconciliation client validates with
   `kind: service`.
2. `through` and `callers`, with 0.3 step 3 (ADR-106, ADR-110). The key
   on a role keyed by permission, the `through` rule in both builds, the
   cell in the matrix, the derived case; `callers` as a list with
   `people` reserved and the `callers` rule, ADR-096's `callers: clients`
   rewritten as the list; the permission table's `through` and `client`
   on a grant with the reader's questions. Done when a role narrowed to
   one application derives the refusal at every operation naming the
   permission, a listed service with no granting role is refused, the
   table case with a row per client gives the client with its role, and
   the lending desk's matrix is unchanged.
3. The tokens, `via`, the audience and the events, with 0.3 step 4
   (ADR-108, ADR-109, ADR-111). `tokenType` with the `Txn-Token` header
   in `generate openapi`; the registry list in both builds with the
   `claim` rule, the draft's claims among them; the `token` rule for a
   `txn_token`; `proof` kinds `spiffe` and `wimse` beside `mutualTls`
   with the `proof` rule; `via` on an operation with the `via` rule
   (short of the induction, which needs the umbrella), the chain case and
   the confused-deputy cases in their three forms; `token_audience` per
   token kind; `securityEvents` with the `security_events` rule, the
   derived event beside every refusal with the severity table, and the
   generated tests checking the event; the tokens table, the event table
   and the call diagram in the techspec. Done when a tree with the
   three-hop example in one module derives the chain, deputy, audience
   and version cases each with its event, each refusal of the rules has a
   red case with the same output in both builds, a `txn_token` with
   `aud` written is refused, and a 0.2 file is read with the version
   warning.
4. The identity module and the induction, after the estate's item 6
   (ADR-107, ADR-111). `trustDomain`; the three reference targets;
   `through`, `via` and `callers` by reference; a module alone counting
   referenced grants as gaps; the umbrella run resolving them, refusing
   a permission no role grants, two modules accepting one audience on
   access tokens, and a gap in a chain found by induction through
   `calls`; the `scope` check against the edge's permission. Done when
   the three-hop example as three modules and an identity module
   validates under the umbrella with no errors, removing the
   middleware's `via` gives the gap at the middleware, and the core's
   techspec draws the chain.
5. The readers, with 0.3 step 5 and the estate's item 7 (ADR-159).
   `clientArgument` on `clientChecks`, `applicationChecks`,
   `callerChecks`, `audienceChecks` and `eventWriters` in the
   implementation schema, `extract go` writing `callers` and `via` from
   literals and asking for the rest, the `Txn-Token` header read asking
   for a `txn_token`, the permission table's two keys. The swift,
   javascript and dart readers read `permissionChecks` only. Done when
   the lending desk's sources extract the same twice and a fixture with
   a literal caller check gives `via`.

## The owner's decisions

Decided on 2026-10-14, after a review of the answers for soundness,
consistency and what the standards say. Each record names the
alternative that lost and why.

1. **Standards first: clients keyed by `software_id`, `client_id` a
   deployment value, `softwareVersion` with a per-client range,
   `softwareStatement` when the version must be trusted, `kind`
   `application` or `service`; the version checked at the edge against
   a declared claim; a refusal is 403 with a problem type** (ADR-106).
   The version is checked once, at the edge, because no claim of a
   Txn-Token carries it; 426 lost because RFC 9110 reserves it for a
   protocol the server refuses.
2. **`through` on the role, keyed by permission; every hop's route has
   its own permission, core's referenced as
   `core#/permissions/<name>`; clients, roles and the trust domain in
   an identity module the other modules reference** (ADR-107). The
   umbrella lost because it lists and pins and holds no design.
3. **The application checked at every hop, with Transaction Tokens: it
   rides in `tctx.application`, immutable; `scope` holds the edge's
   permission; a call crossing a trust domain follows identity
   chaining** (ADR-108). Checking at the edge only lost once a
   Txn-Token's `tctx` carries the application on; `act` nesting inside a
   domain lost because one Txn-Token per transaction is the draft's
   design.
4. **A token accepted without a fixed audience is an error, per token
   kind: for a Txn-Token `aud` is the trust domain, and replay between
   hops is refused by the caller's workload identity. Every refusal
   derives a security event with a fixed name and severity; the author
   declares only the append-only store and the alarm channel; RFC 8417
   format, `txn` on every event, ASVS 5.0 V16, never the raw token**
   (ADR-109). One audience rule for every kind lost because it would
   refuse every Txn-Token domain; "the validator errors when a refusal
   has no event declared" lost because derived cases are written by no
   one.
5. **`callers` is an explicit subset; `people` is reserved like
   `nobody`; a service listed with no granting role is an error**
   (ADR-110). `callers: all` lost because it would admit a client added
   later without anyone writing it.
6. **The confused-deputy cases are derived at every operation for
   people once any service exists, with their Txn-Token forms (`req_wl`
   not an edge, `txn` differing from the one received), each a security
   event** (ADR-108).
7. **`via` on each operation, required wherever a Txn-Token or an `act`
   token is accepted, naming the immediate callers; the chain derived
   by induction; a workload-identity proof mandatory on every service
   in a chain; a token says its kind with `tokenType`, a Txn-Token in
   the `Txn-Token` header** (ADR-111). `via` on the token as a default
   lost because a token minted once for the domain cannot say a
   different caller per hop, and "left out means every chain the issuer
   signed" lost as fail-open.

Decided here, as points the owner's decisions did not settle, each with
its reason in the record: where the version rides (a claim the issuer
fills from the registration, RFC 7591's own line between self-asserted
and publisher-signed, rather than a request header any client can write;
a native application with mixed installs registers per instance, RFC
8252, section 8.4); what `scope` holds (the edge's permission); the
member name `tctx.application`; the event identifier under the trust
domain and the severity table; `securityEvents` as two dependencies of
the identity module, with the repeat count and window per deployment;
the design records numbered ADR-106 to ADR-111, since the Go
implementation file holds ADR-101 to ADR-104 and ADR-105 is the
vocabulary record.
