# Diagrams from the specification: the keywords the context, component, deployment and network pictures need

Every diagram of a system should be drawn from its specification, never
kept beside it, so that it cannot drift (`docs/conventions.md`, Mermaid
diagrams). An editor that draws every diagram from a SpecArch
specification can draw six today: the business process of a workflow,
the state diagram of an entity, the entity relations, the sequence of an
operation or a command, the screen flow and the permissions matrix. It
cannot draw four: the software architecture of one implementation (the
components and what calls what), the system context (the people and the
systems around this one), the deployment topology (what runs on which
host) and the network (zones, ports, who may reach whom), because the
meta-model has no keyword for what they show. That editor's design named
the gaps A to F, and three toolchain points beside them, G to I. This
file is the design: the standards consulted, the decision for each gap
with its decision record, what stays out and why, and the build items in
order. The decision records, ADR-133 to ADR-139, are accepted.

Three tests are applied, as `docs/estate.md` and `docs/caller-context.md`
apply them. The compiler test (`docs/principles.md`): does it keep an
incomplete or wrong definition readable, with every problem in the one
problems file and marked at the entry, and does a file without the new
keys read and render as before? The acceptance test (`docs/acceptance.md`):
does it make a picture fast to show, and a remark made by someone looking
at the picture cheap to take back into the specification? The readers'
test (ADR-044, ADR-050, ADR-075), where `extract` fills a key: a reader
writes nothing the code does not say, and the rules stay in Go.

## Where each gap stands

| Gap | What the picture needs | Decision |
|---|---|---|
| A. which package calls which | an edge per call between the packages of one implementation | `dependsOn` on a layout entry, CycloneDX's word, filled by `extract go` from the imports (ADR-133) |
| B. a cross-system view | the modules of an estate and the dependencies between them | built: `document techspec` given an estate writes `estate.md` with the module map as a flowchart (ADR-113); nothing is added, and the editor reads it through G |
| C. what an external system is, how it is reached, which way a message goes | a shape per kind, a protocol on the edge, the peer of a channel | `kind` and `protocol` on a dependency, `carriedBy` on a channel, and a generated context diagram (ADR-134) |
| D. which callers are people and which are systems | a person and a system drawn apart | no keyword: a role is held by people, a calling system is a client of kind service (ADR-106, ADR-110); the context diagram draws both (ADR-135) |
| E. what a server is and what runs on it | a box per server with the components in it | `name`, `kind`, `runs` and `dependency` on a server, and a generated deployment diagram (ADR-136) |
| F. zones, ports and allowed links | a zone per group of servers, a port per edge, a check that every call has a link | `zone` and `listens` on a server, `links` per deployment, the network diagram and the link check (ADR-137); the owner may keep the picture hand-drawn instead |
| G. one loader for an editor | the merged document with every node's position, the references, the problems | `specarch model`, one JSON document in canonical form, in both builds (ADR-138) |
| H. a lane per approver and a clickable SVG | a lane per role, an id per element | `generate bpmn` writes a lane set and gives every shape the BPMN element's id (ADR-139) |
| I. the handler flowchart's nodes | stable ids and a source range per node | a note for the item that designs the representation, below; no record here |

## Standards first

The rule for new keywords (`docs/conventions.md`) is to take a standard's
word with its exact meaning where one exists, and to say why where none
does. The standards of these pictures, and what each gives:

- **The C4 model** draws four levels: the system context (people, the
  software system, the external software systems), the containers of a
  system (a deployable unit: a service, a database, a browser
  application), the components of a container, and the code. A
  relationship carries a description and a technology ("JSON over
  HTTPS"). Its deployment diagram places container instances on
  deployment nodes (a host, a virtual machine, a container runtime) and
  draws infrastructure nodes (a load balancer, a DNS). The model has no
  file format; the Structurizr DSL spells its words `person`,
  `softwareSystem`, `container`, `component`, `deploymentNode`,
  `infrastructureNode`, `containerInstance` and `technology`, and writes
  a relationship as an arrow with a description and a technology. C4 has
  no word for a zone, a port or an allowed link: its deployment diagram
  shows placement, not the network.
- **ArchiMate 3.2** names the same things in its layers: a business actor
  and a business role; an application component and its interface; and in
  the technology layer a node, a device, system software, an artifact, a
  communication network and a path. Its relationships are serving, flow,
  triggering, access, assignment (an artifact to the node that runs it)
  and realization. ArchiMate is a modelling language with its own
  exchange format; SpecArch takes its words as names, not its format.
- **AsyncAPI 3.0** has what a channel's picture needs: a server object
  with `host`, `protocol`, `protocolVersion` and `pathname`, a channel's
  `servers`, the servers the channel is available on, and an operation's
  `action`, `send` or `receive`, from the application's own point of
  view. SpecArch already takes `channels`, `messages` and `payload` from
  it, one level deep.
- **CycloneDX 1.6**, the software bill of materials format, writes the
  dependency graph of a system's components as a list in which each
  component names under `dependsOn` the components it depends on. That is
  gap A's edge, with the exact meaning.
- **IEC 62443-3-2** divides a system under consideration into zones,
  groupings of assets that share security requirements, joined by
  conduits, groupings of the communication channels between zones. The
  zone is gap F's grouping; a conduit is coarser than the host-to-host
  link a firewall check needs.
- **NIST SP 800-41 Rev. 1**, the firewall policy guideline, states a
  packet-filtering rule as a source, a destination, a service (the
  protocol and the port) and an action. That is the form of an allowed
  link.
- **RFC 8785**, the JSON Canonicalization Scheme, fixes one byte form for
  a JSON document: keys sorted, no whitespace, numbers and strings in one
  spelling. It is what lets two builds print one JSON document byte for
  byte, which is how the conformance suite checks them.

Taken: `dependsOn` (CycloneDX), `protocol` (AsyncAPI's server object),
`zone` (IEC 62443) and the form of a link (NIST SP 800-41), the words
person and system for the context diagram (C4), and RFC 8785 for the
model's bytes. Not taken, with the reason at the gap: C4's `uses` and
`technology`, Structurizr's `containerInstance`, ArchiMate's `assignment`
and `node`, AsyncAPI's `action` and its channel's `servers`, IEC 62443's
conduit.

## A. Which package calls which: `dependsOn`

The implementation file's `layout` lists the packages, modules or folders
of one implementation, each with the design objects it implements. The
component diagram of C4's third level has one box per package and, today,
no arrows, because nothing says which package calls which.

A layout entry gains `dependsOn`, a list of the layout's own keys: the
packages this one calls, imports or links against, in the implementation
file, since what a package depends on is one stack's structure
(`docs/conventions.md`, Design and implementation):

```yaml
layout:
  cmd/api:
    description: The HTTP server.
    implements: ["#/paths/~1loans/post"]
    dependsOn: [internal/loans, internal/store]
  internal/loans:
    description: The loan rules.
    implements: ["#/algorithms/lateFee"]
    dependsOn: [internal/store]
  internal/store:
    description: The database access.
```

The word is CycloneDX's, with its meaning: the component depends on the
components named. C4 writes a relationship's verb in prose (`uses`) and
Structurizr's DSL has no keyword for it, so there was no word to take
there; `imports` is Go's and Python's word and not Java's or Swift's;
`uses` names a relationship in UML with another meaning (a usage
dependency between classifiers), close but not the same. The validator
reports, as the rule `depends_on` at the entry, a name that is not a key
of `layout` and a package that names itself; a cycle is not an error,
since Go forbids one and Java, Swift and TypeScript allow one, and the
file describes the code as it is. The rule is its own and not `layout`,
which already means the file tree (a file or a section not where the
tree puts it), so that one rule name keeps one meaning. A package that
lists no `dependsOn` says nothing: it may depend on anything or nothing,
and the diagram draws no arrow from it. The schema does not require the
key, so every implementation file reads as before.

`extract go --implementation` fills it from the imports: for each package
of the layout, the packages of the same module its files import, by their
layout key, and nothing outside the module (a library is under
`libraries`). The readers' test holds: an import is what the code says,
and the rules stay in Go. The other readers fill it when their
code-facts dumps carry imports; until then they write none.

The technical specification's chapter 5 gains, per implementation file,
the component diagram between the markers `components <stack>`: a
`flowchart` with one node per layout entry, labelled with its key and the
count of design objects it implements, an edge per `dependsOn`, and the
libraries drawn apart. Acceptance test: the arrows are the first thing a
reviewer of an architecture asks for, and a wrong arrow is one entry's
list to change. Compiler test: a name not in the layout is an error at
the entry, and the diagram still draws every other edge.

## B. A cross-system view: built

`document techspec` given an estate writes `estate.md` beside the estate
file: the modules, the version each is held at, the estate's problems,
and each dependency that names a specification as an edge of a flowchart
and a row of a table, a specification the estate does not list drawn
apart (ADR-113, `docs/conventions.md`, Estates). That is the cross-system
view the gap asks for, and the data it is drawn from is
`dependencies[].specification`, as the gap proposes. Nothing is added.
An editor draws the same map from the model of G, which carries the
estate with its modules. With C built, a module's dependencies of other
kinds (a database, a broker) are the module's own and stay in its context
diagram; the estate map shows modules.

## C. What an external system is, how it is reached, and which way a message goes

A dependency is an external system the operations and jobs call,
declared once with the time limit of one call (`docs/conventions.md`,
Dependencies). It has a description, a timeout, and when it is specified
on its own, the name and version of that specification. The context
diagram would draw every dependency the same box, and no edge could say
what travels over it.

Three keys, all in the design, since what kind of thing a system calls,
how it talks to it and where its messages go are the same on every
stack:

```yaml
dependencies:
  memberRegistry:
    description: The registry of members, whose numbers the loans keep.
    timeout: PT2S
    kind: system
    protocol: https
    specification: member-registry
    version: 2.4.0
  ledger:
    description: The finance system's ledger, where a late fee is posted.
    timeout: PT5S
    protocol: https
  eventBus:
    description: The library's message broker.
    timeout: PT1S
    kind: messageBroker
    protocol: amqp
  statementDrop:
    description: The bank's SFTP drop for the monthly statements.
    timeout: PT30S
    kind: fileStore
    protocol: sftp

channels:
  loans:
    description: Loan events, for the notification service and the audit log.
    carriedBy: [eventBus]
    messages:
      LoanOverdue: { payload: { $ref: "#/schemas/LoanEvent" } }
```

**`kind`** says what the dependency is, a SpecArch keyword with five
values: `system`, a software system with an interface this one calls
(C4's software system; a payment gateway, a registry, another module of
the estate); `database`, a data store another party keeps that this
system reads or writes directly; `messageBroker`, a broker that carries
channels; `fileStore`, a file exchange or object store (an SFTP drop, a
bucket); `identityProvider`, the issuer of the tokens this system
accepts, which the caller-context design names at every hop
(`docs/caller-context.md`). Left out, it is `system`, the one meaning
every dependency had. The set is what changes the picture or a check:
a database is drawn as a cylinder and may hold no `specification`, a
broker is where channels live, a file store has no operations, an
identity provider is where the tokens come from. A payment gateway and
a tax authority are both systems, told apart by their descriptions; a
longer list of business kinds would age with every project. The
system's own database and own broker are not dependencies: a dependency
is a system someone else runs; this system's store is a server of its
deployment (E).

**`protocol`** says how the dependency is reached, AsyncAPI's word with
AsyncAPI's meaning, the protocol of a server, written as AsyncAPI writes
it: a lowercase scheme name, `https`, `grpc`, `amqp`, `kafka`, `mqtt`,
`postgresql`, `sftp`, `smtp`. It is a string in the form of a URI
scheme (RFC 3986, 3.1: a letter, then letters, digits, `+`, `-` or `.`),
not an enumeration, because AsyncAPI keeps it open and a new protocol
should not need a new release of SpecArch. C4's `technology` lost: it is
a free sentence ("JSON over HTTPS") that nothing can check, and the
link check of F needs a word it can compare. Left out, the edge is drawn
without a protocol and the link check of F compares nothing.

**`carriedBy`** on a channel names, by their keys under `dependencies`,
the dependencies that carry the channel: the broker it lives on, or the
system that receives its messages directly, as a webhook does. The word
is SpecArch's own. AsyncAPI's channel object has `servers` for the same
place, and it was not taken, for two reasons. An AsyncAPI server is a
host with a protocol, and hosts are what a design file excludes: the
design schema sends servers, hosts, ports and deployments to the
implementation file, where `deployments[].servers` already means hosts,
so the one word would mean hosts on one side of the boundary and
dependencies on the other. And an AsyncAPI channel with no `servers` is
available on every server, where a SpecArch channel named by no
dependency is this system's own; the borrowed word would have been taken
with another meaning, which the rule for new keywords forbids. Which way
a message goes is already said: an operation or a job that lists the
channel's message under `emits` sends it, a job whose trigger `consumes`
the message receives it; AsyncAPI's `action` was not taken because it
would say the same thing a second time at the channel. A channel with no
`carriedBy` is this system's own, drawn at its boundary with no peer.
The validator reports, as the rule `channel` at the entry, a `carriedBy`
entry that is not a declared dependency, and one whose kind is
`database`, `fileStore` or `identityProvider`, since none carries
messages. The rule `dependency` gains a `specification` on a dependency
whose kind is `database`, `messageBroker` or `fileStore`: a
specification is a system's, and an identity provider specified on its
own is one, the identity module of `docs/caller-context.md`, which every
other module declares with `specification` and `version` so that
`identity#/clients/bff` and `identity#/roles/invoice-manager` resolve.

**The context diagram.** The technical specification's chapter 3 gains
it between the markers `context`, a `flowchart LR` in C4's context style
(ADR-135 says who the people are): a person per role on the left and,
when any operation or page is public, one for anyone; the system in the
middle, named by `info.title`; the dependencies on the right, grouped by
kind with a shape per kind (a cylinder for a database, a double box for
a broker, a slanted box for a file store, a hexagon for an identity
provider, a box for a system) and labelled with the protocol; an edge
from the system to each dependency labelled with the operations, jobs
and gateways that call it, a gateway's forwards as `<gateway> by
<claim>` (ADR-141), three names and then a count; an edge from the system to
each dependency a channel is carried by for each channel the system
emits on, labelled with the channel's name, and one the other way for
each channel a job consumes from it. The hand-drawn context picture of
`specarch.md` is replaced by the marker; the convention's table of
diagrams moves the context picture to the generated rows when the item
is built.

Acceptance test: the context diagram is the first picture a stakeholder
sees, and a remark on it ("the ledger is reached over a queue, not
HTTP") changes one key of one dependency. Compiler test: a dependency
of no kind, a channel carried by nothing and a specification of no
dependency at all read and render as before; a wrong `carriedBy` entry
is an error at the entry, and the diagram still draws the channel at
the boundary.

## D. People and systems: no `role.kind`

The gap proposes `kind: person | system` on a role, so that a context
diagram draws people and calling systems apart. That fact is already
said elsewhere. A role is what its holders may do, a set of grants; who
holds it is not the role's business. A calling service is a client of
kind `service` holding roles of its own, and an operation that serves
one lists it under `callers` (ADR-106, ADR-110, `docs/caller-context.md`);
a person is anyone signed in, `people` in that list. A `kind` on the
role would say the same fact a second time, and would break on the
common case of a role held by both: the `invoice-reader` role that the
reporting service and an auditor both hold would need two roles.

Decision: no keyword (ADR-135). The context diagram draws one person per
role, and, once `clients` exists, one system per client of kind
`service`, labelled with the roles it holds, with the roles only clients
hold left out of the people; an application client (the desk's screens)
is the way a person reaches the system and is drawn on the person's edge
as the technology, not as a node. A role a job acts as (the job's
`role`, the scheduler of a nightly job) is left out of the people too,
whatever else holds it: a job's role is the grants the system gives its
own work, and a person who needs the same grants holds a role of their
own. The job is the system's own work and gets no node of its own; it
appears on the edges, where the operations and jobs that call a
dependency, emit on a channel or consume from one label them. Until the
caller-context's first two build items land, every role no job acts as
is a person, which is what every specification means today. The build
item for D is therefore those two items, already designed and not
repeated here; the context diagram of C reads `clients` when the section
exists.

## E. What a server is and what runs on it

An implementation file's `deployments` say where each environment really
runs: a list of servers, each a URL and a sentence. A deployment diagram
drawn from it shows hosts and nothing on them: nothing says which
package or target runs on which server, that a server is the database,
or that a server is where a dependency is reached in this environment.

A server gains four keys, all in the implementation file, since where
one stack's build runs is that implementation's choice (ADR-129 put
`build`, `method` and `infrastructure` there for the same reason):

```yaml
deployments:
  production:
    environment: production
    servers:
      - name: api
        url: https://library.example
        kind: service
        runs: { packages: [cmd/api, internal/loans, internal/store] }
        description: The API behind the desk screens.
      - name: db
        url: postgresql://db.library.internal:5432
        kind: database
        runs: { targets: [sql] }
        description: The loans database.
      - name: bus
        url: amqps://bus.library.internal:5671
        dependency: eventBus
        description: The library's broker, in this environment.
```

**`name`**, a kebab-case name unique within the deployment, is what the
other keys and F's links refer to. It is optional so that every file
reads as before; a server without a name can be referred to by nothing,
which the validator says when a link names a server that does not
exist. Deriving the name from the URL's host lost: two servers can
share a host, and a host name is not a name a person chooses.

**`kind`** says what the server is, with five values: `service`, a host
that runs this implementation's packages; `database`, the store its
migrations run on; `messageBroker`, the broker this system runs for its
channels; `proxy`, a reverse proxy or load balancer in front of a
service (C4's infrastructure node); `fileStore`, the store its files
land in. Left out, it is `service`, the one meaning every server had.
The words are the dependency's (C) where the thing is the same, so that
a reader meets one word for a database whether this system or another
runs it; `service` rather than `system` because a server of this kind
runs part of this system, never another. ArchiMate's node, device and
system software lost: they tell physical from logical, which a
deployment that names a URL has no need of; C4's deployment node and
infrastructure node lost as keywords because C4 tells them apart by
whether something of ours runs there, which `runs` says.

**`runs`** says what of this implementation runs on the server: under
`packages`, keys of `layout`; under `targets`, keys of `targets` (the
`sql` target's migrations run on the database; a `ui` target's pages
are served by a service or a proxy). Two lists under one key because the
two names come from two sections of the file, so a list that mixed them
would need a prefix to tell `sql` the target from `sql` the package.
C4's container instance lost as a word: it names the thing placed, not
the placing. The validator reports, as the rule `server` at the entry,
a name used twice in one deployment, a package or target the file does
not have, and `packages` on a server whose kind is not `service`; a
target may run anywhere, since a proxy serves a `ui` target's pages as
well as a service does.

**`dependency`** names, by its key, a dependency of the design this
server is in this environment: the broker's host, the registry's staging
instance. Such a server's kind is the dependency's own, one of C's five,
`system` and `identityProvider` among them, and it is drawn as that
dependency, in the dependency's shape; it runs nothing of ours. The
server's own `kind` keeps its five values, since `kind` or `runs`
written on such a server is the error `server`. It is how
a link of F reaches a dependency, and how the deployment diagram shows
the real host behind a dependency in each environment. The setting that
hands the code the dependency's address (a channel URL under
`configuration`) stays what it is: the value the code reads; the server
is the host, and the validator does not compare the two.

**The deployment diagram.** The deployment guide and the technical
specification's chapter 7 gain, per deployment, a `flowchart` between
the markers `deployment <deployment>`: a node per server labelled with
its name, kind, host and what it runs, drawn with the kind's shape as in
C; a server that is a dependency drawn as that dependency; the
environment named on the subgraph. With F built, the servers sit in
their zones and the edges are the links; without it, the diagram has no
edges, and says so under it. One diagram per deployment rather than one
for all, because the environments table already shows the promotion
order and three deployments in one picture overrun a page. The
hand-drawn deployment picture of `specarch.md` is replaced by the
marker; the convention's table moves the deployment picture to the
generated rows when the item is built.

Acceptance test: a deployment diagram is what an operator and a
customer's engineer ask for first, and a remark on it changes one
server's `runs`. Compiler test: a deployment with no named server reads
and renders as before, a wrong name is an error at the entry, and the
diagram still draws every other server.

## F. Zones, ports and allowed links

A network diagram shows zones, the ports each host listens on, and which
host may reach which; a firewall check needs the same three facts.
SpecArch has none of them: `infrastructure` holds them as sentences
(`network`, `ports`, `services`, ADR-129), which a reader can follow and
nothing can check. The convention keeps the picture hand-drawn.

The smallest form that draws the picture and runs the check, in the
implementation file, beside E's keys:

```yaml
    servers:
      - name: proxy
        url: https://library.example
        kind: proxy
        zone: edge
        listens: [{ port: 443, protocol: https }]
      - name: api
        url: http://api.library.internal:8080
        zone: application
        runs: { packages: [cmd/api, internal/loans, internal/store] }
        listens: [{ port: 8080, protocol: http }]
      - name: db
        url: postgresql://db.library.internal:5432
        kind: database
        zone: data
        runs: { targets: [sql] }
        listens: [{ port: 5432, protocol: postgresql }]
      - name: bus
        url: amqps://bus.library.internal:5671
        dependency: eventBus
        zone: data
        listens: [{ port: 5671, protocol: amqp }]
    links:
      - { from: proxy, to: api, port: 8080 }
      - { from: api, to: db, port: 5432 }
      - { from: api, to: bus, port: 5671, description: Loan events. }
```

**`zone`** on a server is the name of the network zone it is in, a
kebab-case name the implementation file chooses, IEC 62443-3-2's word
with its meaning: a grouping of assets that share security requirements.
Servers of one zone are drawn in one subgraph. A server with no zone is
drawn in none.

**`listens`** on a server lists what it accepts: a `port`, 1 to 65535,
and a `protocol` in C's form. The URL's port does not count by itself,
so that a link is checked against what the file says and not against
what a URL implies; a server with no `listens` accepts nothing a link
can name.

**`links`** on a deployment lists the allowed connections, each in the
form of a firewall rule (NIST SP 800-41): `from`, a server of the
deployment; `to`, a server of the deployment, a dependency's host among
them; `port`, one the target listens on; and a `description` when the
reason is not plain. IEC 62443's conduit lost: a conduit joins zones,
and the check needs the host and the port. A link in the other
direction is another link, since a firewall rule is directed.

A job is placed the way an operation is: the package that runs it says
so under `implements` (`implements: ["#/jobs/markOverdue"]`), and
the server that runs the package runs the job. The implementation
schema's `designRef`, the pointer `implements` and `mappings` take,
admits entities, enums, permissions, roles, paths, commands, channels,
pages, menus, algorithms and decisions today and no jobs, so it gains
`jobs`; without it nothing in the file puts a job on a server, and a
worker that only runs jobs would be a host the check cannot see.

The validator reports, as the rule `link` at the entry, a `from` or `to`
that is no named server of the deployment, a link from a server to
itself, and a port the target does not listen on. It reports, as the
warning `link_missing` at the server, a server that runs a package
implementing an operation or a job that `calls` a dependency, or that
emits on or consumes from a channel carried by a dependency, while no
link leads from that server to a server that is that dependency in this
deployment; and one that runs a package implementing an entity
(`implements: ["#/entities/Loan"]`, the data access) or a job that
`reads` or `writes` an entity, while no link leads from it to the
deployment's server of kind `database` that runs the `sql` target, when
the deployment has one. That is the firewall check: every call and every
data access the design declares has a path, and nothing else has to: a
gateway package, one that only emits, or a worker that reaches only a
dependency needs no link to the database and gets no warning for it. It
is a warning, not an error, because a deployment is written in order and
the links are its to-do list, as the handover's checks and monitors are
(ADR-129). A must question that blocks the environment the deployment
installs (`#/environments/production`), the `environments` section or the
`deployment` stage holds it (ADR-172); the `link` errors stay.

**The network diagram.** The deployment guide, the handover document and
the technical specification's chapter 7 gain, per deployment that has a
link or a zone, the network diagram between the markers `network
<deployment>`: a subgraph per zone, a node per server as in E, an edge
per link labelled with the port and the target's protocol, and the
deployment's dependencies without a server in it drawn apart as
unreached. The handover document derives its network and ports
paragraphs from the links and listens when a deployment has them, and
prints the `infrastructure` sentences after them for what the keys do
not say (bandwidth, DNS, the backup store); the sentences are not
refused, since the customer's engineers read them, but a deployment with
links may leave `network` and `ports` out.

**The owner's option.** The owner may decide the network picture stays
hand-drawn. Then nothing of F is added: E's deployment diagram shows
placement with no edges, `infrastructure` stays the only place for the
network, and the convention's table gains no network row. The
recommendation is to take F in the form above, last of the keyword items,
because the three keys are small, the firewall check is the one check in
this file that a reviewer cannot do by reading, and a hand-drawn network
picture is the first to drift when a host moves. Taking F later, after
E has been used on a real deployment, costs nothing: E's keys do not
change.

Acceptance test: an operator sees the zones and ports the day the
deployment is written, and a link the firewall team refuses is one line
to remove. Compiler test: a deployment with no zone, listen or link
reads and renders as before; a wrong link is an error at the entry, a
missing one a warning at the server, and the diagram still draws every
other link.

## G. One loader for an editor: `specarch model`

An editor that shows a specification as SpecArch reads it needs the
merged document, where every node came from, what refers to what, and
the layout problems. Today it would load the files itself, which means
a second loader that drifts from the first, in a language the Go build
and the Swift build already share nothing with. The Go build keeps the
file of every node (`internal/spec`) and the line and column of every
value (`internal/source`); the Swift build keeps the same for the
validator; the references are resolved in the validator's rules and
kept by nothing.

`specarch model <folder>...` prints one JSON document on standard output,
in both builds (requirement SA-74, which the case `model-writes-json`
verifies when the item is built), with:

- `specarchModel`, the version of this document's shape, `1.0.0`, and
  `program`, the program version;
- `specifications`, one per root file found: its `root` file, `name`
  and `version`, `metaModel`, the merged `document` as plain values (what
  the schema validates), `positions`, a map from every node's JSON
  pointer (RFC 6901, the pointers the problems file uses) to its `file`,
  `line` and `column`, the ones the problems file reports for that entry,
  `references`, a list of every resolved reference as `from` (the pointer
  of the entry that refers), `to` (the pointer of the element referred
  to) and `through` (the keyword that made it: `$ref`, `target`,
  `permission`, `emits`, `calls`, `satisfies`, `verifies`, `decidedIn`,
  `implements` and the others), `problems`, the layout problems found
  while reading (a file not where the tree puts it, a section in the
  wrong stage, a file that did not parse) in the problems file's form,
  and `implementations`, each implementation file with its `file`,
  `stack`, `document` and `positions`;
- `estates`, each estate file with its modules, their locations and
  versions, and which specification of the document each resolved to.

One list of references serves both directions: a usages lookup is the
list filtered by `to`, which an editor does in one pass; two lists would
say one fact twice. The validator's errors and warnings are not
repeated: the problems file and its SARIF form are the one list of them
(ADR-065), and an editor already reads it. A reference that does not
resolve is one of those problems, so it is absent from the list.

The bytes are RFC 8785's canonical form, so that the Go and the Swift
build print the same document for the same folder and the conformance
suite compares them as it compares every other output. The scheme was
chosen over an indented form because the document is for a program, and
over SpecArch's own ordering because a standard's rule needs no
explaining. `--pretty` is not offered; a reader who wants to look at the
document has `jq`. An unreadable folder, or one with no specification,
is exit status 2 as validate's; a specification with layout problems is
printed with them, status 0, since the document is the editor's way to
show them.

The model is read by the editor's Swift build through the Swift
`specarch`, which is why both builds print it. The model carries no
rule's result at all: a held entry is the `questions` rule's finding,
and it is in the problems file with the rest. Compiler test: a
specification that does not validate is still printed, with every
layout problem in it. Acceptance test: an editor shows the specification
the day the model exists, and what it shows is what validate checks.

## H. A lane per approver role, and a clickable SVG

`generate bpmn` lays a workflow's steps on one row and gives the SVG's
shapes no identity, so a viewer cannot map a click to a step, and a
reader cannot see which role approves which step without reading the
labels (ADR-085).

The process gains a lane set: a first lane named after the system
(`info.title`), holding the start event, the service tasks, the
gateways and the end events, and one lane per role that approves, in the
order of the roles' first approval, each holding the approvals that role
makes. BPMN puts a flow node in exactly one lane, so an approval with
several approvers sits in the lane of its first-listed role and keeps
every role as a potential owner, as today; a lane per set of roles lost
because a BPMN lane is one participant's. The lane set is written for
every workflow, one lane or many, so that a workflow is always drawn the
same way and a viewer meets one layout. The layout stays deterministic:
a row per lane, the steps left to right as today, the same workflow the
same bytes.

The BPMN XML already gives every element a stable id derived from the
step's name: `start`, `step-<step>`, `deadline-<step>`,
`decision-<step>`, `end-done`, `end-refused` and `flow-<n>`, beside the
interface's `operation-<operationId>` and `message-<name>`; no id is
renamed. Only the SVG lacks them: it wraps each shape in a group whose
`id` is that element's id, with `data-step` naming the step as the
specification spells it and `data-role` on a lane. The lanes are the one
addition, with a prefix no other element uses: the lane set is `lanes`,
the system's lane is `lane`, and a role's lane is `lane-<role>`; a
role's lane always carries the suffix and the system's never does, so no
role name can collide with it. A viewer that gets a click on a group has
the step; an editor that gets the step has the entry. `extract
workflows` reads the lane set as it reads the rest of the diagram, from
the potential owners, so the round trip of ADR-085 holds.

## I. A note for the handler flowchart

The item that designs how an operation's handler is recorded as a
flowchart (steps, branches, calls, reads and writes, with the source
cited) should give every node of that representation a stable id, so
that the Mermaid the documents draw and the SVG a viewer shows carry it,
and a source range per node (file, first and last line, at the cited
commit), so that a click on a node opens its lines. Both are cheap when
the representation is designed and costly after. This is a note to that
item, not a record here.

## The kinds, one table

| Kind | On a dependency (design) | On a server (implementation) | C4 | ArchiMate | Drawn as |
|---|---|---|---|---|---|
| `system` | another software system this one calls | only through `dependency`: a server that is a dependency of this kind in that environment | software system | application component | box |
| `service` | not used: a dependency is never ours | a host that runs this implementation's packages | container, on a deployment node | node with an artifact assigned | box |
| `database` | a store another party keeps | the store the migrations run on | container tagged database | system software | cylinder |
| `messageBroker` | a broker that carries channels | the broker this system runs | container tagged queue | system software | double box |
| `fileStore` | a file exchange or object store | the store its files land in | container | artifact on a node | slanted box |
| `identityProvider` | the issuer of the tokens accepted | only through `dependency`: a server that is a dependency of this kind in that environment | software system | application component | hexagon |
| `proxy` | not used: a proxy is not called, it is passed through | a reverse proxy or load balancer | infrastructure node | node | box with a bar |

A person (C4's person, ArchiMate's business actor) is a role's holder
(D); a zone (IEC 62443) is a subgraph.

## Where each keyword sits

| Keyword | Where | Why |
|---|---|---|
| `dependsOn` on a layout entry | implementation file | which package calls which is one stack's structure |
| `kind`, `protocol` on a dependency; `carriedBy` on a channel | design | what a system calls, how, and where its messages go are the same on every stack |
| `name`, `kind`, `runs`, `dependency`, `zone`, `listens` on a server; `links` on a deployment | implementation file, `deployments` | where one stack's build runs, and what its hosts may reach, are that deployment's facts, beside `build`, `method` and `infrastructure` (ADR-129) |

## The validator

Both builds, each with a case in `spec/tests/` and its design test:

| Rule | Reports |
|---|---|
| `depends_on` | a `dependsOn` entry that is no key of `layout`, or the entry's own key |
| `dependency` (extended) | a `specification` on a dependency whose kind is `database`, `messageBroker` or `fileStore` |
| `channel` | a channel's `carriedBy` entry that is no declared dependency, or one of kind `database`, `fileStore` or `identityProvider` |
| `server` | a server's name used twice in one deployment; a `runs` package or target the file does not have; `packages` on a server not of kind `service`; `kind` or `runs` on a server that names a `dependency`; a `dependency` that is not declared |
| `link` | a `from` or `to` that is no named server of the deployment, a link from a server to itself, or a port the target does not listen on |
| `link_missing` (warning) | a server whose packages call a dependency or use a channel it carries with no link to a server that is that dependency, or whose packages implement an entity or a job that reads or writes one with no link to the deployment's database server that runs the `sql` target; held by a must question that blocks the environment the deployment installs, the `environments` section or the `deployment` stage |

## The documents

| Diagram | Marker | Source | Document |
|---|---|---|---|
| context, C4 style | `context` | `roles`, `clients` once built, `dependencies`, `channels`, `calls`, `emits`, `consumes` | techspec chapter 3; replaces the convention's hand-drawn context row |
| components of one implementation | `components <stack>` | `layout`, `dependsOn`, `libraries` | techspec chapter 5 |
| deployment | `deployment <deployment>` | `servers` with `name`, `kind`, `runs`, `dependency` | techspec chapter 7, deployment guide; replaces the convention's hand-drawn deployment row |
| network | `network <deployment>` | `zone`, `listens`, `links` | techspec chapter 7, deployment guide, handover document; a new row of the convention's table |
| module map | none: `estate.md` | the estate (ADR-113) | built |

## Left out

- A `kind` on a role (D): said by `clients` and `callers`.
- C4's `technology` as a free sentence: nothing can check it; `protocol`
  is the word it would hold.
- A port on a dependency in the design: a port is a deployment's fact,
  and the design's `protocol` is what the link check compares.
- Deriving a link from a server's URL, or a listen from its port: the
  check compares what the file says, not what a URL implies.
- A check that a `configuration` value's host is the server named with
  `dependency`: the value is what the code reads and the server is the
  host, and they differ legitimately (a path, a port behind a proxy).
- A cycle in `dependsOn` as an error: stacks differ on it.
- An estate-wide deployment or network picture: each module's
  implementation file has its own deployments, and an estate holds
  nothing of a module (ADR-113).
- A second list of references in the model, by target: one list serves
  both directions.
- A pretty form of the model: it is for a program.
- The validator's problems in the model: the problems file is their one
  list.

## Building

Each item changes `spec/` first (the design, the implementation file's
schema where a key is one, `spec/design/commands/` for a command), both
validator builds where a rule is added, with its conformance cases and
design tests, adds the origin table rows and the convention's diagram
rows, updates the library-lending example and its documents, and
writes a history entry. The order is the order an editor needs them in: the model first, since every
panel reads it, then the pictures by what each gives the reader.

1. **`specarch model`** (ADR-138, SA-74). The command in both builds, the
   command's design in `spec/design/commands/model.yaml`, RFC 8785 in
   both builds, the positions from the readers both already keep, the
   references collected where the validator resolves them, the case
   `model-writes-json` with its design test verifying SA-74, and the two
   builds compared
   on `spec/` and `examples/`. Done when both builds print the same
   bytes for every example and an estate, a folder with a layout problem
   is printed with it, and a reference's `to` is a pointer the problems
   file would use.
2. **The context diagram** (ADR-134, ADR-135). `kind`, `protocol` and
   `carriedBy` in the design schema and its fragments, the Swift embed,
   the rules `channel` and the extended `dependency` in both builds, the
   `context` marker in `document techspec`, the library-lending
   example's dependencies and channels keyed, its hand-drawn picture
   replaced, and the convention's rows moved. Done when the example's
   techspec draws its registry, ledger, broker and people, a channel with
   a wrong `carriedBy` entry is an error at the entry, and a specification without
   the keys renders byte for byte as before.
3. **`dependsOn`** (ADR-133). The key in the implementation schema, the
   rule `depends_on` in both builds, `extract go --implementation`
   filling it from the imports with a case, the `components` marker in
   chapter 5, SpecArch's own Go implementation file filled by its own
   reader. Done when the Go implementation file's component diagram shows
   the arrows the imports give, and a name not in the layout is an error
   at the entry.
4. **The deployment diagram** (ADR-136). `name`, `kind`, `runs` and
   `dependency` on a server, the rule `server` in both builds, the
   `deployment` marker in chapter 7 and the deployment guide, the
   library-lending example's three deployments named and placed. Done
   when the example's production shows the API, the database and the
   broker with what each runs, and a `runs` naming a target the file
   lacks is an error at the entry.
5. **The network diagram and the link check** (ADR-137), unless the
   owner keeps it hand-drawn. `zone`, `listens` and `links`, `jobs` in
   the implementation schema's `designRef` and in the resolver of
   `implements` in both builds, the rules `link` and `link_missing` in
   both builds, the `network` marker in chapter 7, the deployment guide
   and the handover document, the handover's paragraphs derived, the
   example's production with zones and links, and a worker package in
   its layout, which the item adds, implementing `markOverdue` (the
   layout has `internal/api` and `internal/lending` today and no package
   runs the job). Done when removing the API's link to the database
   warns at the API server, removing the worker's link to the broker
   `markOverdue` emits on warns at the worker, a link on a port the database does
   not listen on is an error at the link, and the handover document
   prints the links before the infrastructure sentences.
6. **BPMN lanes and ids** (ADR-139). The lane set and the ids in
   `specarch-gen-bpmn`, the SVG's groups, `extract workflows` reading the
   lanes past, the goldens of `generate-bpmn` and
   `extract-workflows-reads-generated-bpmn` regenerated. Done when a
   two-role workflow draws three lanes, a click target exists per step,
   and the round trip reads the same workflow back.

The client side of D (a system per client of kind `service`) lands with
the caller-context's build items 1 and 2 (`docs/caller-context.md`,
Building), which are not repeated here; item 2 above draws every role as
a person until then.

## The owner's decision points

1. **The kinds** (C and E): five on a dependency, five on a server, one
   word for one thing across both. The alternative is a free string,
   which draws every dependency the same.
2. **No `role.kind`** (D): the departure from the gap as proposed, since
   `clients` and `callers` already say it. The alternative, the key as
   proposed, says one fact twice and breaks on a role both hold.
3. **A server's identity** (E): an optional `name`, rather than a name
   derived from the URL's host.
4. **The network keys** (F): take them in the form above, last, or keep
   the picture hand-drawn. The recommendation is to take them.
5. **The model's bytes** (G): RFC 8785, with no pretty form.
6. **Lanes always** (H): a lane set on every workflow, rather than only
   when a workflow has several approver roles.
