# What SpecArch takes from dxlib

dxlib (github.com/donnyhardyanto/dxlib) and dxlib_module are the owner's Go
libraries, built before code generation was cheap, so that a short declaration
could produce a working Go service: declare the tables and the endpoints, and
the runtime does validation, paging, audit fields, soft delete, masking,
encryption, logging, configuration and the SQL of four database engines. Both
were read for this document, together with two of the owner's services built
on them, to find what SpecArch should take and what it should leave. Nothing
in either library was changed.

dxlib proves two things SpecArch has only argued: that one type definition can be rendered to the API validator, to
JSON, to Go and to four SQL dialects without the design knowing about any of
them, and that a dozen implementation concerns (how a list endpoint pages,
how a row is audited and soft-deleted, how a secret reaches a process, how a
PII value is masked in a log) are the same in every service and belong in a
library, not in each project. SpecArch should adopt the first as a
per-stack type rendering table that the design never sees and a project may
override, and the second as the idiom library designed in `docs/idioms.md`.
It should not adopt dxlib's encoding of either: the type names that fold
width, nullability, range and sensitivity into one word, the positional
constructors, the untyped maps, and the status codes and error bodies that
grew by use rather than by standard. Those were the cost of a declarative
layer written in plain Go with no generator behind it; SpecArch has the
generator.

## dxlib in SpecArch's terms

A dxlib service is a Go program whose `main` registers declarations at start
and then hands control to the runtime. The declarations are the language:

| What is declared | How, in dxlib | Where |
|---|---|---|
| a data type | one `DataType` value with its rendering per target | `types/types.go` |
| a table and its columns | `models.NewModelDBTable(schema, name, order, fields, tde)` with one `ModelDBField` per column: type, key, not null, unique, default per engine, reference as `"schema.table.field"`, encryption and hash companion, generated expression per engine | `databases/models/database_model_table.go` |
| views, indexes, triggers, functions, materialized views, extensions, transparent encryption | one model struct each, rendered per engine | `databases/models/` |
| a table as the API sees it | `tables.NewDXTableSimple(database, table, resultName, listView, idField, uidField, nameIdField, envelope, keys, uniqueGroups, searchFields, orderFields, filterFields)`: the whitelists a list endpoint may search, sort and filter by, and the audit and soft-delete behaviour of `DXTable` over `DXRawTable` | `tables/tables.go`, `tables/tables_table.go` |
| an endpoint | `api.NewEndPoint(title, description, uri, method, kind, contentType, parameters, handler, wsLoop, responses, middlewares, privileges, maxContentLength, rateLimitGroup)`; a parameter is a name, a type name, a description, must-exist, nullable, children and an enum list | `api/api.go`, `api/api_endpoint.go` |
| the same endpoint as a document | an OpenAPI 3.1 document in dxlib's own dialect, emitted from the registrations or read from disk and bound to handlers by `operationId`; a document and its handlers that disagree stop the process | `api/openapi_emit.go`, `api/openapi_read.go`, `api/openapi_bind.go`, `api/OPENAPI.md` |
| a background worker | `task.Manager.NewTask(name, "once" or "always", delaySeconds, handler)` | `task/task.go` |
| configuration | JSON or YAML files with dot-path access, a list of sensitive keys, secure memory for secrets, an optional Vault | `configuration/`, `secure_memory/`, `vault/` |
| the application | `app.App` with lifecycle hooks: define, define configuration, set variables, define endpoints, after start, execute, stopping | `app/app.go` |
| a ready module | a struct holding its `DXTable`s and handler methods, such as user management (users, roles, privileges, organisations, menu items), audit log, announcements and templates, sessions | `dxlib_module/module/` |

The services read for this document use the layers unevenly, which is itself
a finding. One declares its endpoints with `NewEndPoint` calls and parses its
request bodies by hand, and creates its tables from a hand-written SQL file.
The other has put two YAML languages of its own on top of dxlib: one file
that is the serialised form of its `NewEndPoint` calls (name, path, method,
handler or workflow, auth flag, privileges, limits), and one that is its data
model (entities with fields, each a dxlib type name, required, primary key,
unique, reference; indexes), from which it builds the dxlib model and renders
the DDL. Both files are within sight of SpecArch's `paths` and `entities`.
The owner has written a SpecArch-shaped declaration twice already, because
the Go form was not a thing an analyst could read or review. That is the
strongest argument in this document for the generator in section 3.

## The unified type definition

### The model

`types/types.go` defines one struct:

    DataType {
      Description
      APIParameterType             the name, and the key the request validator switches on
      JSONType                     string, number, boolean, object or array
      GoType                       the Go type a value has in memory
      TypeByDatabaseType           the column type on PostgreSQL, SQL Server, Oracle and MariaDB
      DefaultValueByDatabaseType   a default expression per engine, used by the public-id type
    }

and about fifty-five values of it. The names carry more than the base type.
Width is a name (`string1`, `string5`, ... `string255`, `string512`,
`string1024`, `string8096`, `string32768`); nullability is a name
(`nullable-string`, `nullable-int64`); a range is a suffix (`int64p` is
greater than zero, `int64zp` is zero or greater, the same for `int32`,
`float32` and `float64`); a content format is a name (`email`,
`phonenumber`, a country-specific tax number, `iso8601`, `date`, `time`);
sensitivity is a prefix (`protected-string` is masked by role on display,
`protected-sql-string` is also checked for SQL injection,
`protected-non-empty-string` combines two); and a storage role is a name
(`serial`, `bigserial`, `id`, the public-id type, `money`). Two registries
list the values: `Types`, the names a request parameter may use, and
`DataTypes`, a shorter list. The column-only types (the serials, the fixed
widths, money, the geometry point, the public id) are in neither, and `money`
has a parameter-type constant but no registry entry, so it cannot be declared
on a request. A registry keyed by names has to be kept complete by hand, and
this one is not.

Enums are not in the type model. A parameter carries an `Enum` list of
values compared case-insensitively as strings; a column has no enum type, and
a status is a set of Go string constants beside the table. "Nullable" is
written twice, as a type name and as a flag (`IsNullable` on a parameter,
`IsNotNull` on a column), and on a request parameter it means that the
parameter may be left out, not that it carries a JSON null: the request
layer records whether a value was given, so `nullable-int32` maps to a
plain `int32` by design. `nullable-string` maps to `*string`, so the two
names differ in their Go type, but both describe absence. SpecArch keeps
the two ideas apart: `required: false` on a request field for absence, a
type list with `null` for a null value; the type-rendering idiom says how
each renders. Check constraints do not exist in the model at all:
a rule such as "the due date is after the loan date" is code in a handler.
Unique groups exist twice, as a column flag rendered into the DDL and as the
table layer's `ValidationUniqueFieldNameGroups`, checked by a query before an
insert.

### The four renderings

1. **The API validator** (`api/api_endpoint_request_parameter_value.go`)
   switches on the name. It accepts the shapes a value arrives in (every JSON
   number is a float64, so an `int64` takes a float with no fraction and a
   string of digits, since query strings carry only strings), enforces the
   suffix bounds, checks the formats, runs the SQL-injection check on the
   protected types, walks the children of a `json` or
   `array-json-template` parameter, checks enum membership, and refuses a
   missing mandatory value. Every refusal is one upper-case code with the
   parameter's path, such as `MANDATORY_PARAMETER_NOT_EXIST:customer.email`,
   answered with 422.
2. **JSON.** The `JSONType` field is read by nothing outside `types.go`. The
   JSON rendering the library uses is the table in its OpenAPI emitter
   (`api/OPENAPI.md`, section 2.4): each name becomes a faithful JSON Schema
   (`int64p` is `integer`, `format: int64`, `minimum: 1`;
   `non-empty-string` is `string`, `minLength: 1`; `email` is `string`,
   `format: email`), and because several names share one JSON schema, the
   exact name travels beside it as `x-dxlib-type`. Money, the protected
   strings, the phone and tax numbers and `id` have no JSON spelling of
   their own and exist only through that extension.
3. **Go.** The `GoType` field is read in one place, to decide whether a
   column default is quoted. Handlers do not see typed values: they receive
   `map[string]any` and cast, `aepr.ParameterValues["name"].Value.(string)`.
   The Go rendering was declared and never generated.
4. **SQL.** `TypeByDatabaseType` is read by `fieldToDDL` in
   `databases/models/database_model_table.go`, which writes the column type,
   the default (the field's per-engine default, else its generic default
   with the engine-aware sentinels of `database_model_quote.go`, else the
   type's per-engine default for an auto-generated key), `PRIMARY KEY`,
   `NOT NULL`, `UNIQUE` and `REFERENCES`, in the order Oracle requires. The
   engine differences are handled one by one and each carries its reason in
   a comment: `DEFAULT` before `NOT NULL` because Oracle refuses the other
   order; a nullable `UNIQUE` column on SQL Server becomes a filtered unique
   index, because SQL Server treats two NULLs as equal and the other three
   engines do not; generated columns are `STORED` only on PostgreSQL,
   `VIRTUAL` only on Oracle, either on MariaDB, and typeless on SQL Server;
   a MariaDB "schema" is a dot inside one quoted table name, because MariaDB
   has no schema layer; identity columns take the four spellings.

The SQL table, by base type, is the part to keep. Where it holds a choice
worth copying, the reason dxlib wrote next to it is kept too.

| Base type | PostgreSQL | SQL Server | Oracle | MariaDB |
|---|---|---|---|---|
| 32-bit integer | `INT` | `INT` | `NUMBER(10)` | `INT` |
| 64-bit integer | `BIGINT` | `BIGINT` | `NUMBER(19)` | `BIGINT` |
| 64-bit float | `DOUBLE PRECISION` | `FLOAT` | `BINARY_DOUBLE` | `DOUBLE` |
| 32-bit float | `REAL` | `REAL` | `BINARY_FLOAT` | `FLOAT` |
| decimal P,S | `NUMERIC(P,S)` | `DECIMAL(P,S)` | `NUMBER(P,S)` | `DECIMAL(P,S)` |
| text up to N, N at most 8000 | `VARCHAR(N)` | `VARCHAR(N)` | `VARCHAR2(N)` | `VARCHAR(N)` |
| text beyond 8000 | `VARCHAR(N)` | `VARCHAR(MAX)` | `CLOB` | `TEXT` |

That table is dxlib's. Two of its rows change in SpecArch's type-rendering
idiom. Oracle text is `VARCHAR2(N CHAR)` only up to 1000 characters and
`CLOB` beyond: Oracle's limit is 4000 bytes under `MAX_STRING_SIZE =
STANDARD`, the default, and a length in characters is a limit rather than a
capacity, so 1000 characters is what the column always holds (8191 under
`EXTENDED`, which a project sets in an override). dxlib mixes `VARCHAR` and
`VARCHAR2` on Oracle; the idiom always takes `VARCHAR2`. SQL Server text is
`NVARCHAR`, since `VARCHAR` holds the column's code page unless its
collation is a UTF-8 one.
| boolean | `BOOLEAN` | `BIT` | `NUMBER(1)` | `BOOLEAN` |
| date | `DATE` | `DATE` | `DATE` | `DATE` |
| time of day | `TIME` | `TIME` | `DATE` | `TIME` |
| instant | `TIMESTAMP WITH TIME ZONE` | `DATETIMEOFFSET` | `TIMESTAMP WITH TIME ZONE` | `VARCHAR(35)` |
| bytes | `BYTEA` | `VARBINARY(MAX)` | `BLOB` | `LONGBLOB` |
| JSON object | `JSONB` | `NVARCHAR(MAX)` | `CLOB` | `JSON` |
| list of text, of 64-bit integers | `TEXT[]`, `BIGINT[]` | `NVARCHAR(MAX)` | `CLOB` | `JSON` |
| generated 32-bit key | `SERIAL` | `INT IDENTITY(1,1)` | `NUMBER GENERATED BY DEFAULT AS IDENTITY` | `INT AUTO_INCREMENT` |
| generated 64-bit key | `BIGSERIAL` | `BIGINT IDENTITY(1,1)` | `NUMBER(19) GENERATED BY DEFAULT AS IDENTITY` | `BIGINT AUTO_INCREMENT` |
| current time as a default | `CURRENT_TIMESTAMP` | `SYSDATETIMEOFFSET()` | `SYSTIMESTAMP` | `CURRENT_TIMESTAMP` |
| a true default | `true` | `1` | `1` | `true` |

The reasons that travel with the rows:

- An instant on MariaDB is fixed-width UTC text, `VARCHAR(35)`, holding
  `2006-01-02T15:04:05.000000000Z` with nine fractional digits always. The
  engine has no column type that keeps an offset: `DATETIME` drops it on
  write and `TIMESTAMP` renders through the reader's session zone, so the
  same row reads differently on two connections. Fixed-width UTC text is the
  one representation whose byte order is chronological order, which
  `ORDER BY`, `BETWEEN`, `MIN` and `MAX` all depend on. The write path
  normalises to that layout. The comment on `DataTypeISO8601` is the full
  argument and is the best example in the library of a reason written next
  to a choice, which is what principle 11 asks for.
- A key or unique text column is at most 255 characters. SQL Server indexes
  at most 900 bytes and MariaDB with utf8mb4 at most 3072; 255 characters is
  1020 bytes at four bytes each, under both. 512 also fits; 1024 does not on
  SQL Server.
- Money is `NUMERIC(23,4)`: 19 integer digits and 4 decimals fit the 38-digit
  ceiling of SQL Server and Oracle and the 65 of MariaDB, and the value
  travels as a JSON string because JavaScript numbers lose digits.
- Text wider than 8000 is `VARCHAR(MAX)` on SQL Server because that is its
  `VARCHAR` ceiling, and `CLOB` or `TEXT` on the engines that have no wide
  `VARCHAR`. On Oracle the ceiling is lower than 8000, as the paragraph
  under the table says.
- A public identifier that must not be guessable is generated as the
  hexadecimal microsecond time followed by a UUID, in the application
  (`GenerateUID`) or by the engine (`DefaultValueByDatabaseType` carries the
  four expressions), so it sorts roughly by creation and cannot be
  enumerated.

### Where it is used

The model is adopted unevenly. The ERP backend builds its dxlib model from
its own data-model YAML, renders the DDL for its audit tables through
`CreateDDL`, and keeps a snapshot of the model beside the generated SQL. The
notification service bypasses the model and ships one hand-written SQL file
applied at boot. The `tables` layer, which every service uses, needs no model
at all: a `DXRawTable` can name its table directly and declare a per-column
type only for the array columns. So the type definition is single-source for
validation and for the OpenAPI document, and single-source for the DDL only
where a project chose to go through the model.

### Compared with SpecArch

SpecArch's design side already says what dxlib's names say, as separate
keywords, because the keywords are JSON Schema's and each one carries one
fact (`docs/conventions.md`, Types):

| dxlib name | SpecArch field |
|---|---|
| `int32`, `int64` | `type: integer, format: int32`; `format: int64` with bounds inside 2^53, or `type: string, format: int64` |
| `int64p`, `int64zp` | the same with `minimum: 1`, `minimum: 0` |
| `float64`, `float64p` | `type: number, format: double`; with `exclusiveMinimum: 0` |
| `money` | `type: string, format: decimal, precision: 23, scale: 4`, or the precision and scale the field needs |
| `string255`, `string1024` | `type: string, maxLength: 255`; `maxLength: 1024` |
| `non-empty-string` | `minLength: 1` |
| `nullable-string`, `nullable-int32` on a request parameter | `required: false`: the field may be left out; a type list with `null` only for a value that may itself be null |
| `email`, `date`, `time`, `iso8601` | `format: email`, `format: date`, `format: time`, `format: date-time` |
| `bool` | `type: boolean` |
| `blob` | `type: string, format: byte` |
| `json`, `array-string`, `array-int64` | `type: object` with `properties`; `type: array, items: { ... }` |
| `map-string-string` | no spelling yet: `additionalProperties` is not in the field schema |
| an `Enum` list on a parameter | `$ref: "#/enums/Name"`, with `valueDescriptions` |
| `protected-string` | no spelling yet: the gap section 2 fills |
| `id`, the public id, `serial`, `bigserial` | `primaryKey`, `readOnly`, and the identifier idiom of `docs/idioms.md` |
| the tax-number and phone formats | `pattern`, and a project's own format when a stack validates it |

What SpecArch lacks is the other half: there is no table that says what each
of those fields becomes on a stack. The implementation file's `mappings`
name a table for an entity and a function for an algorithm, and the Go
example says in prose that money columns are `numeric(10,2)`. `docs/
generators.md` already assigns the job to the implementation file ("the type
a decimal maps to") and the implementation schema's `target` already has a
`dialect` key with one value, `postgresql`. The slot exists; the table does
not.

### The verdict

Adopt "one type, many targets": one design type, rendered per target by a
table the design never sees. Keep SpecArch's encoding of the type, not
dxlib's. The design type stays the structural description (type, format,
width, bounds, precision and scale, nullability as a type list, enum by
reference), because that is what JSON Schema, OpenAPI and CEL already read
and because an orthogonal description cannot have the registry gaps a list
of fifty-five names has. The rendering table is keyed by that structure: a
row matches a type, a format, a width class and, for decimals, the precision
and scale.

The table is the first shipped idiom, `type-rendering`, in the form
`docs/idioms.md` designs: SpecArch carries the default rows for Go and the
four SQL dialects, Swift and Dart once a project on each exists, a project may override a row, and the override is
recorded and reported. The `sql` target's `dialect` grows from one value to
four, and `specarch-gen-sql` renders through the table. The OpenAPI target
needs no table, since the design is already JSON Schema. Go, Swift and Dart
renderings are the type columns of the same idiom. Evidence that this is the
right home: in dxlib the two renderings that are read, validation and the
OpenAPI document, live where the reader is, and the two that were only
declared, `JSONType` and `GoType`, were never used.

Copy as is: the SQL table above, with its Oracle and SQL Server text rows
as corrected under it, the four engine divergences and their
reasons, the default sentinels, the identity forms, the index-safe width
rule, the public-id expressions, the fixed-width instant text on MariaDB.

Change: precision and scale stay per field, not fixed at 23,4; a 32-bit
float is not in SpecArch and is not added, since nothing in the owner's
designs needs an approximate narrow number; absence and null are two
keywords, `required` and a type list with `null`, each meaning one thing,
rather than one word on a type and a flag; an enum column is rendered as text of the longest value's
width plus a check constraint on all four engines, because the native enum
types of PostgreSQL and MariaDB make renaming or removing a value a type
change; a 64-bit integer is carried as a JSON string or bounded, which
SpecArch's `unsafe_integer` rule already enforces and dxlib does not.

The owner's decisions on the open points:

- Lists on the three engines without an array type render as JSON text, and
  the row says so in its `why`, as dxlib does: it is a documented
  representation, not a silent loss, and the alternative is a child table
  the design did not ask for.
- A UUID column is `UUID` on PostgreSQL, `UNIQUEIDENTIFIER` on SQL Server,
  `VARCHAR2(36 CHAR)` on Oracle and `CHAR(36)` on MariaDB; the public-id
  text form from dxlib is a row a project may choose in an override.
- A duration column is `INTERVAL` on PostgreSQL and `INTERVAL DAY TO
  SECOND` on Oracle, and ISO 8601 text of width 32 on SQL Server and
  MariaDB.
- `format: time`, a time of day, is in the SpecArch type list.
- A money amount is a decimal of the precision and scale the field needs,
  23 and 4 for a currency with many zeros; in Go it is `decimal.Decimal`
  from `github.com/shopspring/decimal` (v1.4.0, MIT), the library dxlib
  uses, and never a float or a plain integer.

## 1. Map: dxlib concept to SpecArch element

Exists means the meta-model has it; partly means it has the design half and
not the runtime half, or the reverse; missing means neither.

| dxlib concept | Where | SpecArch element | Status |
|---|---|---|---|
| the type definition with four renderings | `types/types.go` | field keywords (design); no rendering table (implementation) | partly |
| a table with columns, keys, not null, unique, defaults, foreign keys | `databases/models/database_model_table.go` | `entities` with `properties`, `required`, `primaryKey`, `relations` with `onDelete`, `constraints` of kind unique | exists |
| check constraints | none | `constraints` of kind check, in the CEL subset | SpecArch only |
| generated columns, triggers, functions, views, materialized views, indexes, extensions, transparent encryption per table | `databases/models/` | none; implementation detail of the `sql` target | missing, implementation side |
| a list view beside a table (`v_user`) | `tables.NewDXTableSimple` fourth argument | none; a read model is a 0.2 candidate beside value objects | missing |
| encrypted columns with a searchable hash companion | `databases/database_encryption.go`, `ModelDBField` | none | missing, section 2 |
| enum values | `Enum []any` on a parameter; constants beside a table | `enums`, `$ref`, `valueDescriptions` | exists |
| an endpoint: URI, method, content type, parameters, responses | `api/api_endpoint.go` | `paths` with operations, parameters, request body, responses | exists |
| parameter validation with named failure codes | `api/api_endpoint_request_parameter_value.go` | derived red test cases of `docs/test-generation.md`; no error code or body shape | partly, section 2 |
| privileges on an endpoint; roles granting privileges; organisation-scoped roles | `Privileges []string`; `dxlib_module/module/user_management` | `permission` on every operation, command and page; `roles`; fail-closed | exists; row-level and tenant scope are 0.2 candidates |
| the `EVERYTHING` privilege | `dxlib_module/module/self` | none; a role lists every permission | not taken, section 4 |
| menu items bound to privileges, with parent, level and order | `user_management.menu_item` | `pages` with `route` and `permission`; no navigation tree | partly, section 2 |
| a list endpoint: search text over named fields, filters over whitelisted fields with operators, sort over whitelisted fields, page size and index, include-deleted, download as a sheet | `tables/tables_raw_table.go`, `tables/query_builder/` | a list page has `columns` and `filters`; an operation spells its own parameters | partly, section 2 |
| audit fields on every row (created and last-modified time and user) | `tables/tables_table.go` | none | missing, section 2 |
| soft delete | `DXTable.SoftDelete`, `is_deleted` | none | missing, section 2 |
| internal integer key, public opaque id, human name id, version tag | `FieldNameForRowId`, `...Uid`, `...NameId`, `...Utag` | `primaryKey` only | partly; the identifier idiom |
| WebSocket, upload and download streams, encrypted-envelope endpoints | `DXAPIEndPointType` | HTTP, messaging and command line only | missing; interfaces beyond HTTP are a 0.2 item |
| request size limit, rate-limit group per endpoint | `RequestMaxContentLength`, `RateLimitGroupNameId` | none | missing, section 2 |
| the error body `{status, status_code, reason, reason_message}` | `api/api_endpoint_request.go` | responses carry a description and a schema; no shared shape | missing, section 2 |
| a per-request audit log entry and an error log table with a correlation id | `DXAPIAuditLogEntry`, `dxlib_module/module/audit_log` | `monitors` watch the live system; no audit concept | missing; an idiom, and a 0.2 candidate |
| the OpenAPI document emitted, read and bound by operationId | `api/openapi_*.go` | the planned `specarch-gen-openapi`; `docs/sync-gates.md` route gate | exists as design, section 3 |
| a background task, once or repeating with a delay | `task/task.go` | none; "background jobs and schedules" is a 0.2 candidate | missing, confirmed needed |
| a queue drained by a worker with retries | `dxlib_module/module/push_notification` | `channels` and `messages` (the event); no consumer or retry | partly; the dependency and idempotency concepts of `docs/test-generation.md` |
| configuration files with sensitive keys, secure memory, Vault | `configuration/`, `secure_memory/`, `vault/` | `configuration` settings with `secret`; `deployments` values | exists for the design; the secret path is an idiom |
| masking rules for PII in logs: partial, email, initials, location; credential headers always whole | `utils/utils.go` | none | missing, section 2 |
| a finite state machine with action history | `state_diagram/` | `stateField`, `transitions` | exists |
| multi-language messages | `language/` | none | missing; an idiom |
| health endpoint, printed Markdown spec, Postman collection | `dxlib_module/module/oam`, `api.PrintSpec` | `checks` and `monitors`; the document targets | partly; the documents replace the printers |
| the application lifecycle hooks | `app/app.go` | none; the Go service skeleton is an idiom | implementation side |
| the ERP's endpoint YAML and data-model YAML | that project's `apidef` and `model` packages | `paths`, `entities`; indexes are missing | partly |

## 2. What the design side should adopt

Each item names what dxlib does, which part of it is design (a client of the
interface or a stakeholder needs to know it) and which is an idiom (only the
builders need to know it), and the keyword proposed. Every keyword below is a
0.2 item and follows the rule for new keywords in `docs/conventions.md`: a
standard's word where one exists, SpecArch's own where none does, and only
with a validator rule or a generator that reads it.

1. **Sensitivity of a field.** dxlib marks PII by type name
   (`protected-string`), masks it by role on display, masks it in every log
   with a per-field rule (partial, email, initials, rounded location), and
   never writes a credential at all. The classification is design: a privacy
   requirement with `harm: [privacy]` has to name the fields it protects,
   and a tester needs to know which responses are masked for whom. The rule
   is an idiom. Built: `sensitivity` on a field, one of `public`,
   `internal`, `personal`, `credential`; the validator refuses a `credential`
   field a response can carry unless it is `writeOnly`, and warns for a
   `personal` field in the response of an operation whose permission is
   `public` (`sensitivity_exposed`); the techspec lists the fields that are
   not public in chapter 8. The masking rule per
   sensitivity, and the role that sees the clear value, is the `pii-in-logs`
   idiom.
2. **Encryption at rest.** dxlib encrypts a column in the engine with a
   session key from secure memory and keeps a salted hash beside it so the
   value can still be looked up. Which fields are encrypted is a compliance
   fact, so it is design; the engine function, the key source and the hash
   are an idiom. Built: `atRest: encrypted` on a field, with `lookup:
   hash` when it must stay a key, unique or searchable by equality, which
   the validator requires (`at_rest`). The `sql` target will render through
   the `encrypted-column` idiom or fail for a dialect the idiom does not
   cover.
3. **Audit fields and soft delete.** Every dxlib table of the audited kind
   carries `is_deleted`, `created_at`, `created_by_user_id`,
   `created_by_user_nameid`, `last_modified_at` and the two last-modified
   fields, set by the library and never by the caller, so a client cannot
   forge a timestamp or undelete a row by passing a field. A client needs
   to know that a delete is reversible and that rows carry an author; the
   column names are an idiom. Built: `audited: true` and `deletion:
   soft` on an entity. The entity does not declare `createdAt`,
   `createdBy`, `lastModifiedAt`, `lastModifiedBy` or `deleted`, since the
   keyword says them, so no request body can write them (`audited`); the
   test derivation adds `deleted <Entity> not listed` and `deleted <Entity>
   read`; the `audit-fields` and `soft-delete` idioms will give the
   columns.
4. **List operations.** dxlib's list endpoint is one shape everywhere:
   `search_text` over the table's search fields, `filter_key_values` over
   its filterable fields with the operators equal, in, greater, less,
   null and not null, `order_by` over its sortable fields, `row_per_page`
   and `page_index`, `is_include_deleted`, and an answer of rows, total rows
   and total pages; a sibling endpoint downloads the same selection as a
   sheet. The whitelists are design: a client must know which fields it may
   filter and sort by, and a tester needs the boundary cases of the page
   size. The parameter names and the answer envelope are an idiom. Built:
   `listOf` on an operation, naming the entity, with `searchable`,
   `filterable` and `sortable` field lists and `pageSize` with `maximum`;
   the validator checks the fields exist (`list_of`) and derives `page
   beyond last`, `page size above maximum`, and a sort and a filter outside
   the lists. The OpenAPI target will expand the operation through the
   `paginated-list` idiom.
5. **Background jobs.** dxlib runs a task once or forever with a delay, and
   the notification service drains a queue with retries and a dead-letter
   state. Already a 0.2 candidate in `docs/roadmap.md`; dxlib confirms the
   shape. Built: `jobs`, each with a `trigger` (a schedule in cron form,
   an interval, or a channel it consumes), the `role` it acts as, what it
   `reads` and `writes`, the dependencies it `calls` and what it `emits`,
   `retries` with a limit and a dead-letter or discard outcome, and
   `satisfies`. A job is a subject of tests, with the cases `runs twice`,
   `dependency fails` and `dependency times out`, and `an item fails every
   try`.
6. **The error body.** dxlib answers every refusal with one JSON shape and
   one upper-case code; its status codes are its own (422 for every
   validation failure, 409 for a bad credential). The shape is design: a
   client parses it. Built: RFC 9457, Problem Details for HTTP APIs, is
   the shape of every 4xx and 5xx response, and `errors` in the design is a
   named catalogue of problem types, each with its status, its title and
   the condition, that a response names under `problem`. The reason to take
   the standard rather than dxlib's shape is that a problem document
   carries a `type` URI and an `instance`, so a client can branch on the
   type without parsing a message, and every HTTP client library already
   reads it. The validator refuses a 4xx or 5xx response without a problem
   type once the catalogue exists (`problem`), and the techspec names the
   types each operation refuses with.
7. **Limits on an operation.** A request size ceiling and a rate-limit group
   are facts a client must know. Built: `limits` on an operation with
   `maxRequestBytes` and `rate` (`requests`, `per`, `burst`), with the cases
   `request larger than <n> bytes` and `rate exceeded`; the OpenAPI document
   will carry it as an extension, and the runtime through the `rate-limit`
   idiom.
8. **Navigation.** A menu item in dxlib is a page's place in a tree, with
   the privilege that shows it. Built: `menus`, a tree whose leaves name
   pages; the validator checks every leaf is a page (`menu`), and a leaf is
   shown to who may open its page; the techspec lists the tree under the
   pages.
9. **Identifiers.** dxlib keeps an internal integer key, a public opaque id
   that cannot be enumerated, a human-readable name id and a version tag on
   most rows. Which of these a client sees is design; their generation is an
   idiom. Nothing new is needed in the meta-model: `primaryKey`, a unique
   constraint and `readOnly` say it, and the `identifiers` idiom gives the
   generation. Documented here so the choice is deliberate.
10. **A read model.** A list view that joins names and counts onto a row is
    what every list page shows. Already close to the value-object candidate
    of the roadmap; dxlib adds that it is read-only, derived and
    per-engine. Proposed for 0.2 after the items above: `views`, an entity
    whose `properties` are derived from named entities and relations, never
    written.

Not adopted into the design, because they are implementation: indexes
(they go into the `sql` target's settings under the entity's mapping),
triggers, functions and materialized views, transparent encryption of a
table, the session key mechanism, the multipart and octet-stream request
shapes, the secure-memory and Vault path, the language dictionaries, the
application lifecycle.

## 3. A Go implementation that targets dxlib

### The two options

Plain Go is the route `docs/roadmap.md` and the readiness report describe:
`specarch-gen-openapi` writes the OpenAPI document, a standard OpenAPI
generator in strict-server mode writes the typed server interface, `sqlc` or
a driver writes the queries, `specarch-gen-sql` writes the migrations, and
handlers are written by hand against the interface. The stack is public,
each tool does one job, and the compiler refuses a handler whose signature
drifts from the document.

go-dxlib makes dxlib and dxlib_module the runtime. The implementation file
says `stack: Go` with dxlib among its `libraries`, and a generator,
`specarch-gen-go-dxlib`, emits what a dxlib service declares by hand today:

- the OpenAPI document in dxlib's dialect, which dxlib binds at start to
  handlers registered by `operationId`; a handler missing from the document
  or a document operation without a handler stops the process, which is the
  route-table gate of `docs/sync-gates.md` for free;
- the data model as `models.NewModelDBTable` declarations, so dxlib renders
  the DDL for all four engines, or the DDL itself through the
  `type-rendering` idiom;
- the `tables.NewDXTableSimple` declarations with the search, sort and filter
  whitelists from `listOf`, the unique groups from the constraints, and the
  audit and soft-delete kind from the entity flags;
- one handler stub per operation that calls the library's standard
  operation where there is one (`RequestSearchPagingList`,
  `RequestCreateWithValidation`, `RequestReadByUid`, `RequestEdit`,
  `RequestSoftDelete`) and a typed accessor for its parameters, leaving the
  body for the handler of an operation with an algorithm;
- the privilege and role seed rows for `dxlib_module`'s user management from
  `permissions` and `roles`, and the menu item rows from `menus`;
- a task per job.

### What dxlib cannot take as it stands

Two mismatches were read, not guessed, and both change the size:

- dxlib's OpenAPI reader refuses `pattern`, `maximum`, `maxLength`,
  `minItems`, `uniqueItems` and `const`, and accepts `minLength` only as 1
  and `minimum` only as 0 or 1, because its validator enforces nothing else
  and the reader refuses to promise what the server does not do. Rule 4 of
  `docs/generators.md` then fails generation for a field with a `maxLength`
  of 200 or a `pattern`, which the example specification has in its first
  operation. Either the go-dxlib target emits the constraint as a check in
  the generated handler stub and strips it from the document it hands to
  dxlib, or dxlib's validator grows to enforce the JSON Schema bounds. The
  second is the honest one and is a dxlib item, outside this repository.
- dxlib routes by URI alone and checks the method inside the endpoint, so a
  path item carries one method. The example has `GET` and `POST` on
  `/members`. The generator can split them (`/members` and `/members/create`
  is dxlib's own convention, every operation a `POST` with a JSON body), but
  then the document dxlib binds is not the document the design wrote, and
  the route-table gate compares the wrong thing. This is also a dxlib item,
  or a stated convention of the go-dxlib target that the design accepts.

Smaller ones: handlers receive untyped maps (the generated accessor closes
that); the error body and status codes are dxlib's (the `error-response`
idiom for Go-dxlib maps the problem catalogue onto them until dxlib answers
with a problem document); 32-bit floats and the country-specific formats
have no design spelling (they are not needed).

### Size and payoff

| Piece | Size | Needed by |
|---|---|---|
| `specarch-gen-openapi`, standard dialect | medium | every Go route |
| a `dialect: dxlib` setting on the openapi target: the extensions, one method per path, body parameters for every method with a body, the type names in `x-dxlib-type` | small | go-dxlib |
| `specarch-gen-sql` through the type-rendering idiom, four dialects, new-file migrations | medium to large | every route |
| `specarch-gen-go-dxlib`: models, tables, stubs, accessors, seeds, tasks | medium to large | go-dxlib |
| the dxlib items above (bounds in the validator, a method in the route key) | medium, in dxlib | go-dxlib without workarounds |

The payoff is large for the owner's own services, because the runtime
already does what a generated service would otherwise have to carry as
library code: validation, paging, audit, soft delete, masking, encryption,
four engines, sessions, privileges, menus, tasks. A dxlib service from a
specification would be declarations plus the bodies of the operations with
an algorithm. For a project that is not on dxlib, plain Go stays the
default, and nothing in go-dxlib is needed.

`docs/generators.md` says a generator is accepted only against a real
project. The notification service is the natural first target: it is the
owner's, it is small, it is on dxlib, and its endpoints and tables are hand
declarations today. The umbrella repository's current focus leaves it
waiting; where its specification would live is the owner's call.

### Job C and the OpenAPI route

Job C's private service is generated as plain Go, not in the dxlib
dialect; that is the owner's decision. The route is plain Go as the
readiness report described, and the only thing this document adds to it
is the type-rendering idiom under `specarch-gen-sql` and the Go idioms of
`docs/idioms.md` as the conventions the agent follows when it writes the
handlers. The order of the implementation items below stands as it is.

## 4. What not to take

Each with the reason, which is in every case the same: it was the only way
to get a declaration out of plain Go before a generator existed, and a
generator does it better.

- **Constraints folded into type names.** `int64p`, `non-empty-string`,
  `protected-non-empty-string`, `string512`: each name is one validator
  case and one registry row, the registry is incomplete, and two names
  disagree about nullability. SpecArch keeps one keyword per fact and lets
  the generator produce the validator case.
- **Untyped values in handlers.** `map[string]any` and a cast per parameter
  saved writing a struct per endpoint. A generated accessor costs nothing
  and the compiler then checks the field names.
- **Positional constructors.** `NewDXTableSimple` takes thirteen arguments
  and `NewEndPoint` fourteen, in an order the reader must know; `nil, nil`
  in the middle of a call says nothing. A declaration in YAML with full
  keys is the SpecArch form, and the ERP's own endpoint YAML shows the owner
  reached the same conclusion.
- **A second and a third declaration language on top of the first.** The
  ERP's endpoint YAML and data-model YAML exist because the Go form was not
  reviewable. They are evidence for SpecArch, not shapes to adopt: both lack
  responses, permissions as a model, tests and traceability.
- **Document printers inside the runtime.** The Markdown spec and the Postman
  collection served at run time read the registrations. SpecArch writes the
  documents from the specification before anything runs, which is where a
  reviewer needs them.
- **Status codes and error bodies by habit.** 422 for every validation
  failure, 409 for a wrong credential, 500 for anything a handler did not
  classify, a body of four fields with two of them the same text. A
  standard exists; section 2 takes it.
- **The wildcard privilege.** `EVERYTHING` grants everything and is expanded
  at login. SpecArch's access model is fail-closed and a role lists what it
  grants; a generator can expand a role that holds every permission.
- **Header-carried parameters** (`X-Var` beside a raw body), the
  include-deleted flag, the search-columns override, the download formats:
  runtime details of one library. They live in the go-dxlib idioms, not in
  the design.
- **Country-specific formats as types.** A tax-number type is a `pattern`
  on a field of one project.
- **The deprecated and legacy rows.** The string-backed decimal and the
  1024-wide public id are kept in dxlib for old data; neither is a starting
  point.
- **References as dotted strings.** `"schema.table.field"` resolved at init
  was reflection without reflection. SpecArch's `relations` name the target
  entity and the field, and the validator resolves them before any code
  exists.

## 5. Implementation items, in order

Each changes the specification of `specarch` first, both validator builds
where it adds a rule, and the conformance cases; the generators are Go only.

1. The type-rendering idiom and the idiom mechanism of `docs/idioms.md`:
   the idiom file format and its schema, the shipped set embedded in the
   binary, the implementation file's `idioms` section, the override file,
   the lookup order, the rules `idiom_unknown`, `idiom_part_unknown`,
   `idiom_override_reason` and `idiom_version_behind`, and the Idioms table
   in the techspec's implementation chapter. The first shipped idiom is
   `type-rendering` with the table of this document for the four dialects,
   Go and Swift, and `format: time` joins the type list.
2. The design keywords of section 2 that the validator can check without a
   generator: `sensitivity`, `audited` and `deletion`, `listOf`, `errors`
   and `problem`, `limits`, `menus`, `jobs`, `atRest`. Each with its rule,
   its derived test cases and its place in the documents.
3. `specarch-gen-openapi`, standard dialect, with the problem catalogue as
   the error responses and `listOf` expanded through the `paginated-list`
   idiom.
4. The first Go idioms written from dxlib (`docs/idioms.md`, section "The
   first set"), stack-neutral contract first, Go rendering second.
5. `specarch-gen-sql` through `type-rendering`, four dialects, new-file
   migrations with the destructive step in its own file, indexes from the
   entity mapping's settings, encrypted columns through their idiom.
6. The `dialect: dxlib` setting on the openapi target, and
   `specarch-gen-go-dxlib`, against the notification service as the real
   project, when the owner decides that service's specification goes ahead.
7. Reported to dxlib's own queue, not done here: enforce the JSON Schema
   bounds in the parameter validator and accept them in the OpenAPI reader;
   route by method and URI; add `money` to the parameter registry; answer
   with a problem document.
8. `views` as a read model, after the first real specification needs one.

## Left out of this document

The names of the former private dependency and its owner that a comment in
dxlib_module still mentions, the ticket and design references in dxlib's
comments, the product names in the operation ids of dxlib's OpenAPI
examples, and the country whose tax number dxlib validates. None of them
changes a conclusion.
