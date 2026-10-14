# Meta-model 0.3

Meta-model 0.3 adds the caller: who is making a call, as a record an
expression can name. Three things an identity service and a lending
library both need rest on it and are designed together here: the rows a
role reaches (a member sees only their own loans), machine clients and the
gates on a route that are not a permission, and the formats of the tokens
a service issues and accepts. `docs/roadmap.md`, section 5, keeps the list
of candidates; this file is the plan for the ones taken, each step with
what done looks like. ADR-094 to ADR-097 record the decisions.

## What makes it 0.3

0.2 moved the version with the one rule that made a valid 0.1 file
invalid, and put that rule last (ADR-052). 0.3 has one such change too,
and it comes first, because everything else rests on it: `caller` is a
reserved name wherever an expression's names come from. An entity field,
a value object's property, a page field or an algorithm input named
`caller` is refused in a file that says `specarch: "0.3"`, because an
expression naming it could mean the field or the caller, and an ambiguity
is an error, never a guess. Nothing in this repository has such a name
and a real one is unlikely, but the rule breaks what it breaks, so the
version moves with it (ADR-094). Every other keyword in the steps below
is an addition.

Step 1 moves the version and the trees in this repository together: the
root files, the examples, the trees `specarch extract` and `merge` write,
and the hints they carry. A file that stays on `specarch: "0.2"` keeps
validating as it does today, with one `meta_model_version` warning, until
1.0, as a 0.1 file does (ADR-055). A test fixture's `caller` keeps its
string form, a role or `public`; an object form with the caller's
properties is an addition beside it.

## The caller

Every operation, command, page and job names the one permission its
caller must hold, and roles say who holds it, fail-closed. What the
meta-model cannot say is anything about the caller beyond the roles:
which member they are, which organisation they belong to, through which
client they come, with which claims. Each of the three concepts below
needs that, and designing it three times would give three ways to name
the caller, so it is one concept (ADR-095):

```yaml
caller:
  description: Who is making the call, as the service knows it once the session or token has been checked.
  properties:
    memberId: { type: [string, "null"], format: uuid, description: The member signed in; null for staff and for a client. }
    organisationId: { type: string, format: uuid, description: The organisation every caller belongs to. }
```

`caller` is an object of the design stage with the properties of a value
object (JSON Schema keywords, as `schemas` has them). In an expression,
`caller.<property>` names one of them, and that is the only place a `.`
may appear: the validator checks the name against the declared properties
and the type as it checks a field. Lists, macros and `.` elsewhere stay
refused (`docs/conventions.md`, Expressions). CEL has field access because
its values are messages; SpecArch left it out so that every name in an
expression is a column of the one record a rule is about, and a generator
renders it without a join. The caller is one more record with declared
properties, so `caller.<property>` keeps both: still no join, still every
name declared, in CEL's own form rather than a second naming idiom such as
a prefix on flat names.

A property the caller may lack (a librarian has no member identifier)
allows null, written `type: [string, "null"]` as a field does, and a
comparison with it is typed as a bool and is false when it is null, as a
comparison with a field that allows null is today. A rule written
`memberId == caller.memberId` therefore reaches no row for a caller with
no member identifier, which is the fail-closed answer, without a null test
in every rule.

Where the values come from is partly design and partly implementation. A
token's claims fill the properties of their names (Tokens below), and a
session fills them at sign-in; how a running service carries them from the
request to the query (a context value, a connection setting such as
PostgreSQL's `current_setting`) is the implementation file's.

The roles a caller holds and the client they come through are not
properties. No rule needs them in an expression, because the rows rule is
written per role and a gate names its clients, and keeping them out keeps
sets and lists out of the expression language.

## Taken in 0.3

### The rows a role reaches

A role may limit the rows it reaches in an entity:

```yaml
roles:
  member:
    description: A card holder.
    permissions: [catalogue.read, loans.read]
    rows:
      Loan: memberId == caller.memberId
```

`rows` maps an entity to one expression over that entity's fields and
`caller`, giving a bool, in the subset of `docs/conventions.md`. A holder
of the role reads and changes only the records the expression holds for;
an entity with no rule under a role is reached whole, as today. A caller
holding several roles reaches the union of their rows, after PostgreSQL's
permissive row security policies (`CREATE POLICY ... TO role USING
(...)`), which are the model: one rule per role and table, combined with
OR. The rule holds for every operation, view, list, page and job that
reads or writes the entity, and the validator checks that the entity
exists, that the expression names only its fields and the caller's
properties, and that a rows rule grants nothing: the permission a holder
needs is still a role's.

What a caller sees outside their rows is settled once, for every entity.
A list shows only the rows. A read of one record by its key answers "not
found", never "forbidden", so that an identifier does not reveal whether a
record exists (the OWASP Authorization Cheat Sheet, and the object-level
authorisation failure of the OWASP API Security Top 10). A change that
would leave the rows, such as a member lending a copy to another member,
is refused, as PostgreSQL's `WITH CHECK` refuses it. A job acting as a
role with a rule reads and writes those rows only. These are derived
cases: for every role with a rule and every operation over the entity,
`shows only the caller's rows`, `a record outside the rows is not found`
and, for a write, `a change outside the rows is refused`.

`generate sql` writes the rule as a row security policy on PostgreSQL,
with the caller's properties read from the connection as the
implementation file maps them; on an engine with no row security it
writes nothing and marks the entry, so that the service's own filter is
seen to be the only one, and it never drops the rule. `generate openapi`
says nothing, since the wire is unchanged. The techspec's permissions
matrix gains the rows each role reaches, and the library example's
`member` role, whose `why` says the rule in words today, gets it as a
rule.

A tenant scope, every record of an organisation reached only by its
members, is the same rule written on each entity that carries the
organisation's key. A shorthand that names every such entity at once is
left out until a specification is long enough to want it.

### Machine clients and the gates that are not a permission

A caller that is a program, not a person, is a client (ADR-096):

```yaml
clients:
  nightly-reconciliation:
    description: The finance system's batch, which reads the day's returns.
    roles: [auditor]
    proof:
      kind: signedRequest
      algorithm: hmac-sha256
      covers: ["@method", "@path", content-digest, date]
```

A client holds roles, as a person does, so what it may do is said once,
through permissions, and the fail-closed rule stays one rule: a permission
is granted by a role or is public, and a role is held by people, by
clients, or by both. `proof` says how the client proves its identity on
each request: `signedRequest`, a signature over named parts of the request
after RFC 9421, HTTP Message Signatures, whose covered components
(`@method`, `@path`, a header by its name) are the names used here, with
the algorithm the two sides agree on from the ones RFC 9421 registers
(`hmac-sha256`, `ed25519`, `ecdsa-p256-sha256` and the rest); `mutualTls`,
the client certificate of RFC 8705; or `token`, a format declared under
`tokens`. The algorithm is design and not stack because the client cannot
sign without knowing it, which is the interface boundary's test
(`docs/conventions.md`). The key itself is a secret and is never in a
specification.

An operation says who may call it:

```yaml
    post:
      operationId: importReturns
      permission: returns.import
      callers: clients
```

`callers: clients` means only a declared client holding a role that grants
the permission: a person's session is refused whatever roles they hold,
which is the default-deny caller list of a machine route. Left out,
`callers` means people, signed in through the session or a token, as every
operation means today, and a client is refused there. A route open to both
is not written: no system asked for one, and a route that serves both is
two operations with one handler in the implementation file. The validator
checks that a `callers: clients` operation's permission is granted by a
role some client holds, otherwise nothing can call it and that is an error
at the operation rather than a silent dead route, and derives `refused for
a signed-in person` for it and `refused with a bad signature` for every
operation a `signedRequest` client may call.

A signature a service's own front end adds to a person's request is that
front end declared as a client with its proof, calling on the person's
behalf with the person's token (a delegated token, under Tokens). The
step's item settles how the two are written together on the first real
system that has both.

### A route closed on purpose

A route that exists and is open to nobody is written with the reserved
permission `nobody`, beside `public` (ADR-096):

```yaml
      permission: nobody
```

The validator does not ask that a role grant it, as it does not for
`public`; every other permission no role grants stays the fail-closed
error it is today, so a route closed by intent and a permission forgotten
in the roles are told apart by the word. The derived case is `refused for
every caller`. The route stays in the OpenAPI document and the techspec,
marked closed, because a client that still calls it gets the problem
document the specification declares, not a 404 that looks like a wrong
path. An operation that is `nobody` and `enabledBy` a setting is an error:
the setting would switch on nothing.

### Tokens

A service that issues tokens declares each format (ADR-097):

```yaml
tokens:
  access:
    description: What a signed-in person presents on every call.
    format: jwt
    lifetime: PT15M
    signature: ES256
    claims:
      sub: { description: The person's identifier. }
      memberId: { type: [string, "null"], format: uuid, description: The member signed in; null for staff. }
    default: true
  delegated:
    description: An access token a client holds on behalf of a person.
    format: jwt
    lifetime: PT5M
    signature: ES256
    claims:
      sub: { description: The person acted for. }
      actor: { type: string, description: The client acting. }
      memberId: { type: [string, "null"], format: uuid, description: The member acted for. }
    checks:
      actor-is-not-subject: { expression: actor != sub, message: A client may not act for itself. }
```

`format` is `jwt` (RFC 7519, with the access-token profile of RFC 9068) or
`opaque`, a reference the service looks up. `lifetime` is how long one is
valid. `signature` names an algorithm of RFC 7518 (`ES256`, `RS256`,
`EdDSA`, `HS256`); `none` is refused, as RFC 8725 says a verifier must
refuse it, and the algorithm is design because the verifier is often
another service. `claims` are JSON Schema properties, camelCase like every
property, and go on the wire as `info.wireNames` says, as every other
property does: one rule for the whole contract, not a name per claim. The
registered claims of RFC 7519 (`iss`, `sub`, `aud`, `exp`, `nbf`, `iat`,
`jti`) are single lower-case words, so they are written as the RFC names
them and carry its types without being retyped. `checks` are expressions
over the claims, checked wherever the token is accepted, and each is a
derived red case there. A claim fills the caller property of the same
name, so `caller.memberId` is the `memberId` claim of whichever token the
call carried, null when that token has no such claim; a claim and a
caller property of one name and different types are an error at the
claim. The one token marked `default` is what every operation accepts
unless it names another with `token`; a specification with a session and
no tokens, such as the library, writes none of this, and the session
fills the caller's properties at sign-in. An operation accepting a token
gets `denied with expired <token>`, as a session gives `denied with
expired session`, and an operation naming a token no `tokens` entry
declares is an error.

Refresh tokens (RFC 6749, section 1.5), step-up tokens (RFC 9470) and
machine tokens are formats like any other; what makes each is its
lifetime, its claims and which operations accept it. A token exchange that
issues the delegated token (RFC 8693) is an operation whose response
schema is the token, and the actor rule is checked where the delegated
token is accepted, which is where a requirement such as "the actor claim
never equals the subject" is held against the binding.

`docs/caller-context.md` designs what a service that is called on a
person's behalf through other services needs beyond this, decided on
2026-10-14 (ADR-106 to ADR-111): clients keyed by their `software_id`
with a `kind`, a version and a statement, in an identity module the
other modules reference; the application the person signed in through
as a condition on a grant, `through`, checked at every hop; `callers` as
an explicit subset with `people` reserved; one Transaction Token across
the trust domain in place of the `actor` string above, with `via` on
each operation naming its immediate callers and a workload-identity
proof on every service in a chain; `aud` fixed per token kind; and a
security event for every refusal. Its build items 1 to 3 are steps 3 and
4 as amended, and are built with them.

## Left out of 0.3

| Candidate | Why it waits |
|---|---|
| seed data per profile | The permissions reader is built and reads one seed set into roles and permissions. Two profiles whose permission codes differ are two role models, not one with settings: each is extracted on its own, and `specarch merge` asks about every difference, which is where the two are compared. A value that differs by environment (a session limit of three in production) is a setting's value per deployment in the implementation file; a check that a requirement's number agrees with a deployment's value is a small item of its own, not a meta-model keyword. |
| data held in a cache with an expiry, and tables kept but unused | Where data is held is the implementation file's; a record that expires is `validity`; an unused table is a mapping marked `ownedBy` or a question. |
| more than one deliverable per specification | An implementation file is one stack's, and a second deliverable is a second implementation file of the same specification, naming what it builds in its mappings and targets. A design keyword waits for a second front end to be specified. |
| the roles or the client in an expression | No rule needs them: rows are written per role, and a gate names its clients. Taking them would need sets in the expression language. `docs/caller-context.md` keeps the row: the application, the calling service, the chain and the audience are each a condition declared by name. |
| a route both people and clients call | Written as `callers: [people, <client>]`, the explicit subset of `docs/caller-context.md` (ADR-110), which an estate whose reporting service reads the list people read asked for; `callers: all` is declined as fail-open. |
| a shorthand for a tenant scope on every entity | Written per entity until a specification is long enough to want it. |
| rows rules on a view's or a page's own fields | A rule is over the entity, and a view or page shows the entity's rows; a rule over a view's derived fields would be a second place to say it. |

## Building 0.3

Each step is one item, built spec first in both validator builds where
the validator changes, with conformance cases, the techspec regenerated,
and a history entry.

1. The caller and the rows a role reaches. `caller` and `roles.<role>.rows`
   in the design schema; `caller.<property>` in the expression parser of
   both builds, with `caller` reserved; the rules `caller_property` (an
   undeclared property, or `.` anywhere else), `rows` (an unknown entity,
   an expression that is not a bool, a name that is neither the entity's
   nor the caller's) and `reserved_name`; the derived cases; the version
   moved to 0.3 with the trees and hints of this repository, 0.2 read
   with the warning. The library example declares its caller and the
   member's rule. Done when each refusal has a red case with the same
   output in both builds, the library validates with its rule (golden),
   `specarch derive` writes the cases, the techspec's permissions matrix
   shows the rows, and a field named `caller` is refused in a file that
   says 0.3 and read with the version warning in one that says 0.2.
2. Row security in `generate sql`. A policy per rule on PostgreSQL, the
   caller's properties through the implementation file's mapping, nothing
   and a mark on an engine without row security. Done when the library's
   migration carries the member policy and `generate sql --check` passes.
3. Clients, `callers` and `nobody`. `clients` with `roles` and `proof`,
   `callers` on operations and commands, `nobody` as a reserved
   permission, the rules in both builds, the derived cases, and the
   techspec's matrix with clients beside roles. The library gains the fee
   ledger's reconciliation client. Done when a `callers: clients`
   operation no client can reach, a proof with an unregistered algorithm,
   a covered component that is not a request part, a `nobody` operation
   with `enabledBy`, and a client naming an undeclared role are each
   refused with the same output in both builds, and the example
   validates.
4. Tokens. `tokens` with `format`, `lifetime`, `signature`, `claims`,
   `checks` and `default`, `token` on operations, the rules, the derived
   cases, and `generate openapi` writing the bearer scheme the default
   token gives. The example stays on its session; the conformance suite
   carries a token case. Done when `none`, an unregistered algorithm, a
   claim whose type differs from the caller property of its name, a check
   over an undeclared claim, two defaults and an operation naming an
   undeclared token are each refused in both builds, and a valid tree with
   two tokens derives `denied with expired access` on each operation.
5. The readers. `extract router` and `extract go` write `callers: clients`
   where a middleware the implementation file's bindings name as a client
   check guards a route, and ask for the client list; a signature
   middleware becomes a question naming the proof; a table of clients the
   seeds reader finds becomes `clients`; the claims found where a token is
   minted become a question listing them. Done when the lending desk's
   sources extract the same twice with the new questions counted in CI.

## How 0.3 meets the extract readers

Until step 5 lands, a gate that is not a permission is what it is today: a
must question at the operation naming the middleware, and the operation is
written with the permission the question asks for. A client record in a
seed is read as a row of a table. The readers never write a `rows` rule or
a token on their own: both are design, and the code shows only that a
filter or a verifier exists.
