// Written by embed-schemas.sh from ../schema and ../idioms. Do not edit; run the script.

let designSchemaJSON = #"""
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-design-0.1.schema.json",
  "title": "SpecArch specification, meta-model 0.1",
  "description": "A SpecArch specification describes one system through its whole life cycle: the sources it rests on, the requirements (stakeholders, needs, requirements, glossary, assumptions, constraints), the design (enums, entities, permissions, roles, session, endpoints, commands, channels, dependencies, pages, algorithms, decisions), the tests, the deployment (environments, configuration, release, rollback, migrations) and the commissioning (checks, sign-off). On disk it is a folder whose root file is specarch.yaml; each stage listed under 'stages' lives in a folder of that name, and a stage not listed may be written inside specarch.yaml itself. The validator merges the tree into one document that this schema describes. Field keywords come from JSON Schema, endpoint keywords from OpenAPI 3, event keywords from AsyncAPI. Keywords with no standard origin are marked 'SpecArch keyword' in their description. Every object rejects unknown keys; keys starting with 'x-' are allowed everywhere as extensions.",
  "type": "object",
  "properties": {
    "specarch": {
      "description": "Meta-model version this file is written against.",
      "type": "string",
      "const": "0.1"
    },
    "info": {
      "$ref": "#/$defs/info"
    },
    "stages": {
      "description": "SpecArch keyword. The life-cycle stages this specification keeps in folders of their own, in life-cycle order. A listed stage has a folder of that name beside specarch.yaml and its sections live there; a stage that is not listed has no folder, and its sections, if any, are written in specarch.yaml. Only in specarch.yaml.",
      "type": "array",
      "items": {
        "type": "string",
        "enum": [
          "requirements",
          "design",
          "implementation",
          "tests",
          "deployment",
          "commissioning",
          "operation"
        ]
      },
      "uniqueItems": true
    },
    "sources": {
      "description": "SpecArch keyword. The external sources this specification rests on and cites: standards, regulations, documents, interviews, existing systems and external requirement sets, keyed by kebab-case name. A citation anywhere in the specification ('cites') names one of these, so a source's title, edition and URL are written once. Only in specarch.yaml.",
      "type": "object",
      "propertyNames": {
        "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
      },
      "additionalProperties": {
        "$ref": "#/$defs/source"
      }
    },
    "stakeholders": {
      "description": "SpecArch keyword. Who has an interest in the system (ISO/IEC/IEEE 29148:2018, 5.2.2), keyed by kebab-case role name, never a person's name.",
      "type": "object",
      "propertyNames": {
        "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
      },
      "additionalProperties": {
        "$ref": "#/$defs/stakeholder"
      }
    },
    "needs": {
      "description": "SpecArch keyword. Raw stakeholder needs as gathered, before they are turned into requirements (ISO/IEC/IEEE 29148:2018, 5.2.3 and 6.3), keyed by ID such as NEED-1.",
      "type": "object",
      "propertyNames": {
        "pattern": "^[A-Z][A-Z0-9]{1,15}-[A-Za-z0-9._]+$"
      },
      "additionalProperties": {
        "$ref": "#/$defs/need"
      }
    },
    "requirements": {
      "description": "SpecArch keyword. The requirements of the system (ISO/IEC/IEEE 29148:2018, 6.4), keyed by ID such as SA-1. Design elements name the requirements they satisfy under 'satisfies'; tests and commissioning checks name the ones they verify under 'verifies'.",
      "type": "object",
      "propertyNames": {
        "pattern": "^[A-Z][A-Z0-9]{1,15}-[A-Za-z0-9._]+$"
      },
      "additionalProperties": {
        "$ref": "#/$defs/requirement"
      }
    },
    "glossary": {
      "description": "SpecArch keyword. Terms of the domain and their meaning (ISO/IEC/IEEE 29148:2018, 9.2.3), keyed by the term as written.",
      "type": "object",
      "propertyNames": {
        "pattern": "^[A-Za-z][A-Za-z0-9 ./'-]*$"
      },
      "additionalProperties": {
        "$ref": "#/$defs/term"
      }
    },
    "assumptions": {
      "description": "SpecArch keyword. What is taken to be true without proof, and would change the design if it turned out false (ISO/IEC/IEEE 29148:2018, 9.5.19), keyed by ID such as ASSUME-1.",
      "type": "object",
      "propertyNames": {
        "pattern": "^[A-Z][A-Z0-9]{1,15}-[A-Za-z0-9._]+$"
      },
      "additionalProperties": {
        "$ref": "#/$defs/assumption"
      }
    },
    "constraints": {
      "description": "SpecArch keyword. Limits on the solution that are not negotiable: technical, organisational, legal (ISO/IEC/IEEE 29148:2018, 9.6.16; arc42 section 2), keyed by ID such as CON-1. Not to be confused with an entity's data constraints.",
      "type": "object",
      "propertyNames": {
        "pattern": "^[A-Z][A-Z0-9]{1,15}-[A-Za-z0-9._]+$"
      },
      "additionalProperties": {
        "$ref": "#/$defs/projectConstraint"
      }
    },
    "enums": {
      "description": "Named value sets, keyed by PascalCase name.",
      "type": "object",
      "propertyNames": {
        "$ref": "#/$defs/typeName"
      },
      "additionalProperties": {
        "$ref": "#/$defs/enum"
      }
    },
    "entities": {
      "description": "Persistent things with identity, keyed by PascalCase name. Each entity becomes a table (or collection) and a schema component.",
      "type": "object",
      "propertyNames": {
        "$ref": "#/$defs/typeName"
      },
      "additionalProperties": {
        "$ref": "#/$defs/entity"
      }
    },
    "views": {
      "description": "SpecArch keyword, after the SQL view. Read models keyed by PascalCase name: one row per record of an entity, carrying every field of it and the properties the view adds, fields read through its relations and counts of its related records. A view is never written.",
      "type": "object",
      "propertyNames": {
        "$ref": "#/$defs/typeName"
      },
      "additionalProperties": {
        "$ref": "#/$defs/view"
      }
    },
    "schemas": {
      "description": "OpenAPI's components.schemas. Value objects keyed by PascalCase name: data passed around but not stored, with no identity, such as a diagnostic, a summary or a token's claims. A schema has no key and no table; a request body, a response, a message or another schema may refer to it, and an entity may not.",
      "type": "object",
      "propertyNames": {
        "$ref": "#/$defs/typeName"
      },
      "additionalProperties": {
        "$ref": "#/$defs/valueObject"
      }
    },
    "permissions": {
      "description": "SpecArch keyword. Named rights, keyed by dotted lower-case name such as 'loans.create'. The name 'public' is reserved: an operation or page that grants it is open to everyone, including unauthenticated callers.",
      "type": "object",
      "propertyNames": {
        "$ref": "#/$defs/permissionName"
      },
      "additionalProperties": {
        "$ref": "#/$defs/permission"
      }
    },
    "roles": {
      "description": "SpecArch keyword. Roles a caller can hold, each listing the permissions it grants. A role grants nothing it does not list.",
      "type": "object",
      "propertyNames": {
        "$ref": "#/$defs/roleName"
      },
      "additionalProperties": {
        "$ref": "#/$defs/role"
      }
    },
    "separationOfDuties": {
      "description": "SpecArch keyword. Sets of permissions one holder must never have together, after the static separation of duty of ANSI INCITS 359, keyed by kebab-case name. No role may grant cardinality or more of a set's permissions; the techspec lists the combinations of roles that together reach a set, as roles never to be given to one person.",
      "type": "object",
      "propertyNames": {
        "$ref": "#/$defs/separationName"
      },
      "additionalProperties": {
        "$ref": "#/$defs/separationOfDuties"
      }
    },
    "session": {
      "description": "SpecArch keyword. How a caller's signed-in session ends: 'idleTimeout', the time without a request after which it expires, and 'absoluteTimeout', the time after signing in after which it expires whatever the caller does; at least one of the two (OWASP Session Management Cheat Sheet). Once a session is declared, every operation, command and page whose permission is not 'public' gets the derived red case 'denied with expired session'.",
      "$ref": "#/$defs/session"
    },
    "paths": {
      "description": "OpenAPI keyword. HTTP endpoints keyed by path template.",
      "type": "object",
      "propertyNames": {
        "pattern": "^/"
      },
      "additionalProperties": {
        "$ref": "#/$defs/pathItem"
      }
    },
    "commands": {
      "description": "SpecArch keyword. Command-line commands, keyed by the words a user types after the program name, such as 'validate' or 'generate techspec'. A command is an interface like an HTTP operation: it names a permission (fail-closed) and lists its arguments, options, files and exit codes.",
      "type": "object",
      "propertyNames": {
        "$ref": "#/$defs/commandName"
      },
      "additionalProperties": {
        "$ref": "#/$defs/command"
      }
    },
    "channels": {
      "description": "AsyncAPI keyword. Event channels keyed by channel name (dotted lower-case). Each channel carries one or more messages; a message is an event.",
      "type": "object",
      "propertyNames": {
        "pattern": "^[a-z][a-z0-9]*(\\.[a-z][a-z0-9]*)*$"
      },
      "additionalProperties": {
        "$ref": "#/$defs/channel"
      }
    },
    "dependencies": {
      "description": "SpecArch keyword. External systems the operations call, keyed by camelCase name: a payment gateway, a directory, a ledger. Each has a time limit per call. An operation names the ones it calls under 'calls'; the validator derives the cases where one fails and where one does not answer in time.",
      "type": "object",
      "propertyNames": {
        "$ref": "#/$defs/memberName"
      },
      "additionalProperties": {
        "$ref": "#/$defs/dependency"
      }
    },
    "jobs": {
      "description": "SpecArch keyword. The work the system does on its own, keyed by camelCase name.",
      "type": "object",
      "propertyNames": {
        "$ref": "#/$defs/memberName"
      },
      "additionalProperties": {
        "$ref": "#/$defs/job"
      }
    },
    "workflows": {
      "description": "SpecArch keyword, a sequential subset of BPMN 2.0. Requests that finish later, after people approve them, keyed by kebab-case name: the operation that starts each, answering 202, the entity that holds the request while it waits, and its approval and operation steps in order. The person who made a request never approves it. A workflow is a test subject.",
      "type": "object",
      "propertyNames": {
        "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
      },
      "additionalProperties": {
        "$ref": "#/$defs/workflow"
      }
    },
    "errors": {
      "description": "SpecArch keyword. The catalogue of problem types the operations answer with, keyed by kebab-case name. Every 4xx and 5xx response is an RFC 9457 problem document and names its type under problem.",
      "type": "object",
      "propertyNames": {
        "type": "string",
        "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
      },
      "additionalProperties": {
        "$ref": "#/$defs/problemType"
      }
    },
    "pages": {
      "description": "SpecArch keyword. Screens, keyed by kebab-case name.",
      "type": "object",
      "propertyNames": {
        "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
      },
      "additionalProperties": {
        "$ref": "#/$defs/page"
      }
    },
    "menus": {
      "description": "SpecArch keyword. The navigation: a tree of entries whose leaves name pages, keyed by camelCase name. A leaf is shown to who has the page's permission.",
      "type": "object",
      "propertyNames": {
        "$ref": "#/$defs/memberName"
      },
      "additionalProperties": {
        "$ref": "#/$defs/menuItem"
      }
    },
    "flows": {
      "description": "SpecArch keyword, after the navigation flows of OMG's IFML. Tasks a person does across pages, keyed by kebab-case name: who does it and each step, a page and the event on it that leads to the next step's page. A flow is a test subject.",
      "type": "object",
      "propertyNames": {
        "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
      },
      "additionalProperties": {
        "$ref": "#/$defs/flow"
      }
    },
    "accessibility": {
      "description": "SpecArch keyword. The accessibility the user interface conforms to, by the standard and its level. With it, the validator checks what the design decides: a title for every field a page shows and a distinct label for every action of a page.",
      "type": "object",
      "properties": {
        "standard": {
          "description": "WCAG 2.2, the W3C Recommendation of October 2023.",
          "type": "string",
          "enum": [
            "WCAG 2.2"
          ]
        },
        "level": {
          "description": "The conformance level; AA is what most laws ask for.",
          "type": "string",
          "enum": [
            "A",
            "AA",
            "AAA"
          ]
        }
      },
      "required": [
        "standard",
        "level"
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "theme": {
      "description": "SpecArch keyword. The visual design as design tokens in the format of the W3C Design Tokens Community Group, the modes that give a token another value, and the pairs of colours shown as text or controls on a background, whose contrast the validator checks against WCAG 2.2. How a token becomes code is the stack's.",
      "type": "object",
      "properties": {
        "tokens": {
          "$ref": "#/$defs/tokenGroup"
        },
        "modes": {
          "description": "Other modes, such as dark, keyed by camelCase name: each gives tokens, by path, another value. The tokens' own values are the default mode.",
          "type": "object",
          "propertyNames": {
            "$ref": "#/$defs/memberName"
          },
          "additionalProperties": {
            "type": "object",
            "minProperties": 1
          }
        },
        "pairs": {
          "type": "array",
          "items": {
            "$ref": "#/$defs/colorPair"
          }
        }
      },
      "required": [
        "tokens"
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "algorithms": {
      "description": "SpecArch keyword. Computations that must be specified before they are written by hand, keyed by camelCase name.",
      "type": "object",
      "propertyNames": {
        "$ref": "#/$defs/memberName"
      },
      "additionalProperties": {
        "$ref": "#/$defs/algorithm"
      }
    },
    "tests": {
      "description": "SpecArch keyword. Design tests: what must hold whatever the stack, in plain given, when and then sentences. Each test is about one subject (an operation, a command, a page, or an entity's constraint or transition), has a level (system or acceptance) and is marked golden (the path that succeeds) or red (a path that fails). In a folder tree each test is a folder tests/<name>/ holding test.yaml and the scenario's own input and expected-output files. The validator derives the cases each subject needs from the rest of the specification and reports every one no test covers; see 'Tests' in docs/conventions.md.",
      "type": "object",
      "propertyNames": {
        "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
      },
      "additionalProperties": {
        "$ref": "#/$defs/test"
      }
    },
    "decisions": {
      "description": "SpecArch keyword. Architecture decision records, keyed by ID such as 'ADR-001'.",
      "type": "object",
      "propertyNames": {
        "pattern": "^ADR-[0-9]{3,}$"
      },
      "additionalProperties": {
        "$ref": "#/$defs/decision"
      }
    },
    "environments": {
      "description": "SpecArch keyword. Where the system is installed, keyed by kebab-case name such as development, staging, production: what each is for and which one a release goes to next. Hosts, URLs and ports belong to an implementation file's deployments.",
      "type": "object",
      "propertyNames": {
        "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
      },
      "additionalProperties": {
        "$ref": "#/$defs/environment"
      }
    },
    "configuration": {
      "description": "SpecArch keyword. The settings the system reads at run time, keyed by camelCase name: what each is, its type, and whether it is a secret. A secret's value is never written in a specification; the implementation file says where it comes from.",
      "type": "object",
      "propertyNames": {
        "$ref": "#/$defs/memberName"
      },
      "additionalProperties": {
        "$ref": "#/$defs/setting"
      }
    },
    "release": {
      "$ref": "#/$defs/procedure",
      "description": "SpecArch keyword. How a new version is put into an environment, as ordered steps with a check for each (ISO/IEC/IEEE 12207:2017, 6.4.10 Transition process)."
    },
    "rollback": {
      "$ref": "#/$defs/procedure",
      "description": "SpecArch keyword. How the previous version is restored when a release fails, as ordered steps with a check for each."
    },
    "migrations": {
      "description": "SpecArch keyword. Changes to stored data that a release carries, keyed by kebab-case name, each with its steps and how it is reversed. A migration that cannot be reversed says so in its description.",
      "type": "object",
      "propertyNames": {
        "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
      },
      "additionalProperties": {
        "$ref": "#/$defs/migration"
      }
    },
    "checks": {
      "description": "SpecArch keyword. The checks run on the real installed system before it is handed over (the site acceptance test of IEC 62381:2024; ISO/IEC/IEEE 12207:2017, 6.4.11 Validation process), keyed by kebab-case name. Each names its kind, the environment it runs in and its steps. The results of each run are records, kept outside the specification; see 'Commissioning records' in docs/conventions.md.",
      "type": "object",
      "propertyNames": {
        "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
      },
      "additionalProperties": {
        "$ref": "#/$defs/check"
      }
    },
    "signoff": {
      "$ref": "#/$defs/signoff",
      "description": "SpecArch keyword. What must be true for the installed system to be accepted, and which roles sign."
    },
    "monitors": {
      "description": "SpecArch keyword. What is watched on the live system (ISO/IEC/IEEE 12207:2017, 6.4.12 Operation process), keyed by kebab-case name: each says what is measured, the objective it must meet, the environment it runs in and the requirements it verifies. The tool, the query and where an alert goes are implementation; the implementation file's deployments name them per monitor. Lives in the operation stage.",
      "type": "object",
      "propertyNames": {
        "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
      },
      "additionalProperties": {
        "$ref": "#/$defs/monitor"
      }
    },
    "questions": {
      "description": "SpecArch keyword. The open questions: what the sources do not say and a stakeholder must decide or provide, keyed by ID such as Q-12. A question is written in the folder of the stage it is about, so every stage folder may hold questions; the validator merges them into one section and checks each sits in the stage of what it blocks. A question leaves the specification when it is answered; the decision that answered it names it under answers.",
      "type": "object",
      "propertyNames": {
        "pattern": "^[A-Z][A-Z0-9]{0,15}-[A-Za-z0-9._]+$"
      },
      "additionalProperties": {
        "$ref": "#/$defs/question"
      }
    }
  },
  "required": [
    "specarch",
    "info"
  ],
  "propertyNames": {
    "not": {
      "$ref": "#/$defs/stackSpecificKey"
    }
  },
  "patternProperties": {
    "^x-": {}
  },
  "additionalProperties": false,
  "$defs": {
    "typeName": {
      "description": "PascalCase identifier for an entity or enum.",
      "type": "string",
      "pattern": "^[A-Z][A-Za-z0-9]*$"
    },
    "memberName": {
      "description": "camelCase identifier for a field, relation, algorithm or input.",
      "type": "string",
      "pattern": "^[a-z][A-Za-z0-9]*$"
    },
    "permissionName": {
      "type": "string",
      "pattern": "^[a-z][a-z0-9]*(\\.[a-z][a-z0-9]*)*$"
    },
    "roleName": {
      "type": "string",
      "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
    },
    "separationName": {
      "type": "string",
      "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
    },
    "idKey": {
      "description": "SpecArch keyword. The ID of a requirement, need, assumption or constraint: an upper-case prefix of 2 to 16 letters or digits, a dash, and an ID, such as SA-1 or NEED-12.",
      "type": "string",
      "pattern": "^[A-Z][A-Z0-9]{1,15}-[A-Za-z0-9._]+$"
    },
    "duration": {
      "description": "SpecArch keyword. A length of time as ISO 8601 writes it, in days, hours, minutes and seconds only and never zero: PT5S, PT30M, P1DT12H. Weeks, months and years are not allowed, because their length depends on the calendar, and a limit must mean the same every day.",
      "type": "string",
      "pattern": "^P(?:[0-9]+D|(?:[0-9]+D)?T(?:[0-9]+H(?:[0-9]+M)?(?:[0-9]+S)?|[0-9]+M(?:[0-9]+S)?|[0-9]+S))$"
    },
    "questionId": {
      "description": "SpecArch keyword. The ID of an open question: an upper-case prefix of 1 to 16 letters or digits, a dash, and an ID, such as Q-12 or OPEN-3.",
      "type": "string",
      "pattern": "^[A-Z][A-Z0-9]{0,15}-[A-Za-z0-9._]+$"
    },
    "requirementLink": {
      "description": "SpecArch keyword. The ID of a requirement: one defined under 'requirements' in this specification, or one in an external requirement set declared under 'sources' with kind requirement-set, whose prefix is the set's prefix.",
      "type": "string",
      "pattern": "^[A-Z][A-Z0-9]{1,15}-[A-Za-z0-9._]+$"
    },
    "satisfies": {
      "description": "SpecArch keyword. The requirements this element satisfies, by ID (the satisfy relation of ISO/IEC/IEEE 29148:2018, 6.5.1, and SysML). Every named design and deployment element carries it; fields, parameters, responses, actions and worked examples trace through the object that holds them. A requirement no element satisfies is reported by the validator.",
      "type": "array",
      "items": {
        "$ref": "#/$defs/requirementLink"
      },
      "uniqueItems": true
    },
    "verifies": {
      "description": "SpecArch keyword. The requirements this test, commissioning check or monitor verifies, by ID (the verify relation of ISO/IEC/IEEE 29148:2018, 6.5.2). A requirement no test verifies is reported by the validator.",
      "type": "array",
      "items": {
        "$ref": "#/$defs/requirementLink"
      },
      "uniqueItems": true
    },
    "why": {
      "description": "SpecArch keyword. The rationale in plain words: why this element is the way it is, what was concluded. Optional on every element; a document renders it as an Insight next to the element.",
      "type": "string",
      "minLength": 1
    },
    "citation": {
      "description": "SpecArch keyword. One citation: which source, where in it, and what it says here. A document renders it as a Note under the element's Insight.",
      "type": "object",
      "properties": {
        "source": {
          "description": "The key of a source declared under 'sources' in specarch.yaml.",
          "type": "string",
          "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
        },
        "clause": {
          "description": "The clause, section, page or question cited, as the source numbers it, such as '5.2.5' or 'Annex A'.",
          "type": "string",
          "minLength": 1
        },
        "says": {
          "description": "What the source says that applies here, in plain words: what it requires, recommends or reports.",
          "type": "string",
          "minLength": 1
        }
      },
      "required": [
        "source",
        "says"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "citations": {
      "description": "SpecArch keyword. The sources this element rests on, one citation each. Optional on every element.",
      "type": "array",
      "items": {
        "$ref": "#/$defs/citation"
      },
      "minItems": 1
    },
    "origin": {
      "description": "SpecArch keyword. How the element is known. stated: a source says it, and cites names the source and where in it. inferred: it was concluded from evidence (code, data, the source's silence), and why says from what. decided: a stakeholder settled it, answering a question or correcting the source, and decidedIn names the decision. An element without origin was written spec-first. What is not known at all is not an element with an origin but an open question under questions.",
      "type": "string",
      "enum": [
        "stated",
        "inferred",
        "decided"
      ]
    },
    "decidedIn": {
      "description": "SpecArch keyword. With origin decided: the decision record that settled this element, such as ADR-021.",
      "type": "string",
      "pattern": "^ADR-[0-9]{3,}$"
    },
    "markdown": {
      "description": "Prose in Markdown.",
      "type": "string"
    },
    "schemaRef": {
      "description": "A JSON pointer into the specification, such as '#/entities/Member', '#/enums/LoanStatus', '#/views/LoanRow' or '#/schemas/Diagnostic'. The pointer names the object wherever its file is in the tree.",
      "type": "string",
      "pattern": "^#/(entities|enums|views|schemas)/[A-Z][A-Za-z0-9]*$"
    },
    "info": {
      "description": "OpenAPI keyword. What the system is. Only in specarch.yaml.",
      "type": "object",
      "properties": {
        "title": {
          "type": "string",
          "minLength": 1
        },
        "version": {
          "description": "Version of the specification, semantic versioning.",
          "type": "string",
          "pattern": "^[0-9]+\\.[0-9]+\\.[0-9]+(-[0-9A-Za-z.-]+)?$"
        },
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "owners": {
          "description": "SpecArch keyword. Teams or components accountable for the system, not people.",
          "type": "array",
          "items": {
            "type": "string"
          }
        },
        "tracksOrigin": {
          "description": "SpecArch keyword. true when the specification was built from sources and every element says how it is known: the validator then reports each element of a section that carries no origin (origin_missing, a warning).",
          "type": "boolean"
        }
      },
      "required": [
        "title",
        "version"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "source": {
      "description": "SpecArch keyword. One external source: where a requirement, a decision or any other element comes from, or what it cites.",
      "type": "object",
      "properties": {
        "kind": {
          "description": "What the source is: a published standard; a law or regulation; a document (a book, a manual, a report, an internal document); an interview with a stakeholder; an existing system that was observed running; the source code of an existing system, read at one commit (the edition), whose citations name a file and line or a package and function as the clause; or an external tracker that links point into: a requirement set for requirement links, a change set for the change requests and a defect set for the defects that records name.",
          "type": "string",
          "enum": [
            "standard",
            "regulation",
            "document",
            "interview",
            "system",
            "code",
            "requirement-set",
            "change-set",
            "defect-set"
          ]
        },
        "title": {
          "description": "The title as the source itself gives it, such as 'ISO/IEC/IEEE 29148:2018 Systems and software engineering, Life cycle processes, Requirements engineering'.",
          "type": "string",
          "minLength": 1
        },
        "edition": {
          "description": "The edition, version or year, as the source gives it.",
          "type": "string",
          "minLength": 1
        },
        "author": {
          "description": "Who wrote or published it: a body, an organisation or a role, not a private person's name.",
          "type": "string",
          "minLength": 1
        },
        "date": {
          "description": "When an interview was held or a document was issued, as a quoted date.",
          "type": "string",
          "format": "date"
        },
        "url": {
          "description": "Where the source can be read. For a requirement-set, change-set or defect-set, a URL template with {id} for one item.",
          "type": "string",
          "minLength": 1
        },
        "givenOutside": {
          "description": "SpecArch keyword. True when the system's owners gave this source to parties outside, such as a published interface, a manual handed to customers or a contract, so that a promise in it is not the project's alone to change. specarch merge asks a must question about an element only this source or only the code has.",
          "type": "boolean"
        },
        "prefix": {
          "description": "For kind requirement-set, change-set or defect-set: the prefix of the IDs in that set, such as JIRA, so that a link JIRA-12 resolves to it. One tracker that holds several kinds is declared once per kind, and the prefixes may be the same.",
          "type": "string",
          "pattern": "^[A-Z][A-Z0-9]{1,15}$"
        },
        "clauses": {
          "description": "SpecArch keyword. The outline of a document or of code: every section a document numbers or heads, or every package, folder or file of code, each once, so that specarch gaps can show which of them the specification cites and which produced nothing. A citation's clause falls under the longest listed clause it equals or starts with, followed by a dot, a colon, a slash or a space.",
          "type": "array",
          "items": {
            "type": "object",
            "properties": {
              "clause": {
                "description": "The clause as citations name it: a section number or heading, or a path in the code.",
                "type": "string",
                "minLength": 1
              },
              "title": {
                "description": "The heading of the section, or what the code there does.",
                "type": "string",
                "minLength": 1
              }
            },
            "required": [
              "clause",
              "title"
            ],
            "additionalProperties": false
          },
          "minItems": 1
        },
        "description": {
          "$ref": "#/$defs/markdown"
        }
      },
      "required": [
        "kind",
        "title"
      ],
      "if": {
        "properties": {
          "kind": {
            "enum": [
              "requirement-set",
              "change-set",
              "defect-set"
            ]
          }
        }
      },
      "then": {
        "required": [
          "prefix"
        ]
      },
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "stakeholder": {
      "description": "SpecArch keyword. One role with an interest in the system (ISO/IEC/IEEE 29148:2018, 5.2.2).",
      "type": "object",
      "properties": {
        "description": {
          "description": "Who they are and what they do with the system.",
          "type": "string",
          "minLength": 1
        },
        "concerns": {
          "description": "What they need from the system or fear about it, one plain sentence each.",
          "type": "array",
          "items": {
            "type": "string",
            "minLength": 1
          },
          "minItems": 1
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "description"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "need": {
      "description": "SpecArch keyword. One raw stakeholder need (ISO/IEC/IEEE 29148:2018, 6.3). A need is refined into one or more requirements, which name it under 'needs'.",
      "type": "object",
      "properties": {
        "statement": {
          "description": "The need as the stakeholder would say it, in plain words.",
          "type": "string",
          "minLength": 1
        },
        "stakeholders": {
          "description": "The stakeholders who have this need, by key.",
          "type": "array",
          "items": {
            "type": "string",
            "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
          },
          "minItems": 1,
          "uniqueItems": true
        },
        "status": {
          "description": "Where the need stands: proposed until the stakeholders confirm it, accepted when they do, rejected when it will not be met.",
          "type": "string",
          "enum": [
            "proposed",
            "accepted",
            "rejected"
          ]
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "statement",
        "stakeholders"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "requirement": {
      "description": "SpecArch keyword. One requirement with its attributes (ISO/IEC/IEEE 29148:2018, 5.2.8): what it asks, what kind it is, how important, where it stands, how it is accepted and verified, and which needs it comes from.",
      "type": "object",
      "properties": {
        "statement": {
          "description": "The requirement as one verifiable sentence: who or what shall do what, under which condition (ISO/IEC/IEEE 29148:2018, 5.2.4 and 5.2.5).",
          "type": "string",
          "minLength": 1
        },
        "kind": {
          "description": "functional: what the system does; quality: how well (performance, usability, security, reliability); interface: how it connects to people and other systems; constraint: a limit on how it is built.",
          "type": "string",
          "enum": [
            "functional",
            "quality",
            "interface",
            "constraint"
          ]
        },
        "priority": {
          "description": "must: the system is not acceptable without it; should: expected, but a release without it can be accepted; could: wanted when it costs little (MoSCoW, as used in DSDM).",
          "type": "string",
          "enum": [
            "must",
            "should",
            "could"
          ]
        },
        "status": {
          "description": "proposed until the stakeholders agree to it, accepted when they do, rejected when it will not be met, retired when it no longer applies.",
          "type": "string",
          "enum": [
            "proposed",
            "accepted",
            "rejected",
            "retired"
          ]
        },
        "acceptance": {
          "description": "The conditions under which the requirement counts as met, one verifiable sentence each; they are what the tests and commissioning checks show.",
          "type": "array",
          "items": {
            "type": "string",
            "minLength": 1
          },
          "minItems": 1
        },
        "verification": {
          "description": "How the requirement is shown to be met: inspection (looking at the item), analysis (calculation or model), demonstration (operating it), test (measuring it against defined criteria); the four methods of MIL-STD-961E and the INCOSE handbook.",
          "type": "string",
          "enum": [
            "inspection",
            "analysis",
            "demonstration",
            "test"
          ]
        },
        "needs": {
          "description": "The needs this requirement refines, by ID.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/idKey"
          },
          "minItems": 1,
          "uniqueItems": true
        },
        "harm": {
          "description": "SpecArch keyword. What is at stake when the requirement is not met. A requirement with any harm is critical, and so is every derived test case of the elements that satisfy it; priority says whether a release may go without the requirement, not what a failure costs.",
          "type": "array",
          "items": {
            "type": "string",
            "enum": [
              "data-loss",
              "money",
              "security",
              "safety",
              "privacy",
              "availability"
            ]
          },
          "minItems": 1,
          "uniqueItems": true
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "statement",
        "kind",
        "priority",
        "status"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "term": {
      "description": "SpecArch keyword. One glossary term.",
      "type": "object",
      "properties": {
        "definition": {
          "description": "What the term means in this system, in plain words.",
          "type": "string",
          "minLength": 1
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "definition"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "assumption": {
      "description": "SpecArch keyword. One assumption.",
      "type": "object",
      "properties": {
        "statement": {
          "type": "string",
          "minLength": 1
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "statement"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "projectConstraint": {
      "description": "SpecArch keyword. One constraint on the solution.",
      "type": "object",
      "properties": {
        "statement": {
          "type": "string",
          "minLength": 1
        },
        "kind": {
          "description": "technical: a platform, language or interface that is given; organisational: a team, process, budget or deadline; legal: a law, regulation or licence.",
          "type": "string",
          "enum": [
            "technical",
            "organisational",
            "legal"
          ]
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "statement",
        "kind"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "enum": {
      "description": "JSON Schema keywords. A closed set of string values.",
      "type": "object",
      "properties": {
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "type": {
          "type": "string",
          "const": "string"
        },
        "enum": {
          "type": "array",
          "items": {
            "type": "string",
            "pattern": "^[a-z][a-z0-9]*(_[a-z0-9]+)*$"
          },
          "minItems": 1,
          "uniqueItems": true
        },
        "valueDescriptions": {
          "description": "SpecArch keyword. Meaning of each value, keyed by value; JSON Schema has no per-value description.",
          "type": "object",
          "additionalProperties": {
            "type": "string"
          }
        },
        "satisfies": {
          "$ref": "#/$defs/satisfies"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "type",
        "enum"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "field": {
      "description": "JSON Schema keywords, a bounded subset. A field is either a scalar with a type, an array, a nested object, or a reference to an enum or entity.",
      "type": "object",
      "properties": {
        "$ref": {
          "$ref": "#/$defs/schemaRef"
        },
        "type": {
          "oneOf": [
            {
              "type": "string",
              "enum": [
                "string",
                "integer",
                "number",
                "boolean",
                "array",
                "object"
              ]
            },
            {
              "description": "A type that also allows null, written as JSON Schema does: [string, 'null'].",
              "type": "array",
              "items": {
                "type": "string",
                "enum": [
                  "string",
                  "integer",
                  "number",
                  "boolean",
                  "array",
                  "object",
                  "null"
                ]
              },
              "minItems": 2,
              "maxItems": 2,
              "uniqueItems": true,
              "contains": {
                "const": "null"
              }
            }
          ]
        },
        "title": {
          "description": "JSON Schema's title: the label a person reads for the field, on a page and in a document.",
          "type": "string",
          "minLength": 1
        },
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "format": {
          "description": "The concrete type of the value, where the JSON type alone would leave it open. type says how the value is carried in JSON; format says what it is. An integer needs int32, int64 or uint64; a number needs double; a decimal is a string with format decimal, precision and scale; an int64 or uint64 that can exceed 2^53 is carried as a string with format int64 or uint64. See 'Types' in docs/conventions.md.",
          "type": "string",
          "enum": [
            "int32",
            "int64",
            "uint64",
            "double",
            "decimal",
            "date",
            "date-time",
            "time",
            "duration",
            "email",
            "uuid",
            "uri",
            "hostname",
            "ipv4",
            "ipv6",
            "byte",
            "binary",
            "password"
          ]
        },
        "enum": {
          "type": "array",
          "items": {
            "type": [
              "string",
              "integer",
              "number",
              "boolean"
            ]
          },
          "minItems": 1,
          "uniqueItems": true
        },
        "const": {
          "type": [
            "string",
            "integer",
            "number",
            "boolean"
          ]
        },
        "default": {},
        "examples": {
          "type": "array",
          "minItems": 1
        },
        "minimum": {
          "type": "number"
        },
        "maximum": {
          "type": "number"
        },
        "exclusiveMinimum": {
          "type": "number"
        },
        "exclusiveMaximum": {
          "type": "number"
        },
        "multipleOf": {
          "type": "number",
          "exclusiveMinimum": 0
        },
        "minLength": {
          "type": "integer",
          "minimum": 0
        },
        "maxLength": {
          "type": "integer",
          "minimum": 0
        },
        "pattern": {
          "type": "string",
          "format": "regex"
        },
        "items": {
          "$ref": "#/$defs/field"
        },
        "minItems": {
          "type": "integer",
          "minimum": 0
        },
        "maxItems": {
          "type": "integer",
          "minimum": 0
        },
        "uniqueItems": {
          "type": "boolean"
        },
        "properties": {
          "type": "object",
          "propertyNames": {
            "$ref": "#/$defs/memberName"
          },
          "additionalProperties": {
            "$ref": "#/$defs/field"
          }
        },
        "required": {
          "type": "array",
          "items": {
            "$ref": "#/$defs/memberName"
          },
          "uniqueItems": true
        },
        "readOnly": {
          "description": "Set by the system, never by a client (an ID, a timestamp).",
          "type": "boolean"
        },
        "writeOnly": {
          "type": "boolean"
        },
        "deprecated": {
          "type": "boolean"
        },
        "precision": {
          "description": "SpecArch keyword. Total digits of a decimal; required with format: decimal.",
          "type": "integer",
          "minimum": 1
        },
        "scale": {
          "description": "SpecArch keyword. Digits after the decimal point; required with format: decimal.",
          "type": "integer",
          "minimum": 0
        },
        "mistakes": {
          "description": "SpecArch keyword. How often users get this field wrong: frequent or rare. It replaces the default frequency of every derived test case about the field, which decides with the harm of the subject whether the case is written or left out.",
          "type": "string",
          "enum": [
            "frequent",
            "rare"
          ]
        },
        "sensitivity": {
          "description": "SpecArch keyword. How sensitive the value is: public; internal, for staff only; personal, data about a person, masked in logs and shown only to who may see it; credential, a secret such as a password or a token, which no response carries unless the field is writeOnly. The masking per sensitivity is the pii-in-logs idiom.",
          "type": "string",
          "enum": [
            "public",
            "internal",
            "personal",
            "credential"
          ]
        },
        "atRest": {
          "description": "SpecArch keyword. encrypted: the value is stored encrypted, so it cannot be searched, sorted or compared in storage unless lookup gives a way. The engine function and the key are the encrypted-column idiom.",
          "type": "string",
          "enum": [
            "encrypted"
          ]
        },
        "lookup": {
          "description": "SpecArch keyword. With atRest encrypted: hash, a salted hash kept beside the value, so the field can be a key, unique or filtered by equality.",
          "type": "string",
          "enum": [
            "hash"
          ]
        }
      },
      "oneOf": [
        {
          "required": [
            "$ref"
          ]
        },
        {
          "required": [
            "type"
          ]
        }
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false,
      "allOf": [
        {
          "if": {
            "anyOf": [
              {
                "properties": {
                  "type": {
                    "const": "integer"
                  }
                },
                "required": [
                  "type"
                ]
              },
              {
                "properties": {
                  "type": {
                    "type": "array",
                    "contains": {
                      "const": "integer"
                    }
                  }
                },
                "required": [
                  "type"
                ]
              }
            ]
          },
          "then": {
            "required": [
              "format"
            ],
            "properties": {
              "format": {
                "enum": [
                  "int32",
                  "int64",
                  "uint64"
                ]
              }
            }
          }
        },
        {
          "if": {
            "anyOf": [
              {
                "properties": {
                  "type": {
                    "const": "number"
                  }
                },
                "required": [
                  "type"
                ]
              },
              {
                "properties": {
                  "type": {
                    "type": "array",
                    "contains": {
                      "const": "number"
                    }
                  }
                },
                "required": [
                  "type"
                ]
              }
            ]
          },
          "then": {
            "required": [
              "format"
            ],
            "properties": {
              "format": {
                "enum": [
                  "double"
                ]
              }
            }
          }
        },
        {
          "if": {
            "anyOf": [
              {
                "properties": {
                  "type": {
                    "const": "string"
                  }
                },
                "required": [
                  "type"
                ]
              },
              {
                "properties": {
                  "type": {
                    "type": "array",
                    "contains": {
                      "const": "string"
                    }
                  }
                },
                "required": [
                  "type"
                ]
              }
            ]
          },
          "then": {
            "properties": {
              "format": {
                "not": {
                  "enum": [
                    "int32",
                    "double"
                  ]
                }
              }
            }
          }
        },
        {
          "if": {
            "properties": {
              "format": {
                "const": "decimal"
              }
            },
            "required": [
              "format"
            ]
          },
          "then": {
            "required": [
              "precision",
              "scale"
            ]
          }
        }
      ]
    },
    "entity": {
      "type": "object",
      "properties": {
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "type": {
          "type": "string",
          "const": "object"
        },
        "properties": {
          "description": "JSON Schema keyword. The entity's fields.",
          "type": "object",
          "propertyNames": {
            "$ref": "#/$defs/memberName"
          },
          "additionalProperties": {
            "$ref": "#/$defs/field"
          },
          "minProperties": 1
        },
        "required": {
          "type": "array",
          "items": {
            "$ref": "#/$defs/memberName"
          },
          "uniqueItems": true
        },
        "primaryKey": {
          "description": "SpecArch keyword. Field or fields that identify a row.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/memberName"
          },
          "minItems": 1,
          "uniqueItems": true
        },
        "relations": {
          "description": "SpecArch keyword. Links to other entities, keyed by camelCase name.",
          "type": "object",
          "propertyNames": {
            "$ref": "#/$defs/memberName"
          },
          "additionalProperties": {
            "$ref": "#/$defs/relation"
          }
        },
        "constraints": {
          "description": "SpecArch keyword. Rules the data must satisfy beyond a single field's own keywords, keyed by snake_case name (it becomes the database constraint name).",
          "type": "object",
          "propertyNames": {
            "pattern": "^[a-z][a-z0-9]*(_[a-z0-9]+)*$"
          },
          "additionalProperties": {
            "$ref": "#/$defs/constraint"
          }
        },
        "stateField": {
          "description": "SpecArch keyword. The field whose enum values form this entity's state machine; transitions lists the allowed moves. Generators draw the state diagram from it.",
          "$ref": "#/$defs/memberName"
        },
        "transitions": {
          "description": "SpecArch keyword. Allowed state changes of stateField.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/transition"
          },
          "minItems": 1
        },
        "validity": {
          "$ref": "#/$defs/validity"
        },
        "audited": {
          "description": "SpecArch keyword. true: every record carries who created and last changed it and when, set by the system and never by a caller: createdAt, createdBy, lastModifiedAt and lastModifiedBy, which the entity does not declare itself. The columns are the audit-fields idiom.",
          "const": true
        },
        "deletion": {
          "description": "SpecArch keyword. soft: a delete marks the record deleted and keeps it; a deleted record is not listed and reads as not found. The flag, deleted, is not declared by the entity. The column is the soft-delete idiom.",
          "type": "string",
          "enum": [
            "soft"
          ]
        },
        "satisfies": {
          "$ref": "#/$defs/satisfies"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "type",
        "properties",
        "primaryKey"
      ],
      "dependentRequired": {
        "transitions": [
          "stateField"
        ],
        "stateField": [
          "transitions"
        ]
      },
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "relation": {
      "type": "object",
      "properties": {
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "target": {
          "description": "The entity this relation points at.",
          "$ref": "#/$defs/typeName"
        },
        "kind": {
          "type": "string",
          "enum": [
            "one-to-one",
            "one-to-many",
            "many-to-one",
            "many-to-many"
          ]
        },
        "via": {
          "description": "For many-to-one and one-to-one: the field on this entity that holds the foreign key. For one-to-many: the field on the target that points back. For many-to-many: the join entity.",
          "type": "string",
          "pattern": "^[A-Za-z][A-Za-z0-9]*$"
        },
        "onDelete": {
          "description": "What happens to this side when the target row is deleted. Default restrict.",
          "type": "string",
          "enum": [
            "restrict",
            "cascade",
            "set_null"
          ],
          "default": "restrict"
        },
        "satisfies": {
          "$ref": "#/$defs/satisfies"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "target",
        "kind",
        "via"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "constraint": {
      "type": "object",
      "properties": {
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "kind": {
          "type": "string",
          "enum": [
            "unique",
            "check"
          ]
        },
        "fields": {
          "description": "For unique: the fields that together must be unique.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/memberName"
          },
          "minItems": 1,
          "uniqueItems": true
        },
        "expression": {
          "description": "For check: one expression in SpecArch's subset of CEL (docs/conventions.md) over the entity's fields, giving true or false.",
          "type": "string",
          "minLength": 1
        },
        "message": {
          "description": "What a user is told when the constraint fails.",
          "type": "string"
        },
        "satisfies": {
          "$ref": "#/$defs/satisfies"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "kind",
        "message"
      ],
      "allOf": [
        {
          "if": {
            "properties": {
              "kind": {
                "const": "unique"
              }
            }
          },
          "then": {
            "required": [
              "fields"
            ]
          }
        },
        {
          "if": {
            "properties": {
              "kind": {
                "const": "check"
              }
            }
          },
          "then": {
            "required": [
              "expression"
            ]
          }
        }
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "transition": {
      "type": "object",
      "properties": {
        "from": {
          "type": "string"
        },
        "to": {
          "type": "string"
        },
        "trigger": {
          "description": "The operationId, command, channel message ('channel/Message') or algorithm that causes the move.",
          "type": "string"
        },
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "satisfies": {
          "$ref": "#/$defs/satisfies"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "from",
        "to"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "validity": {
      "description": "SpecArch keyword. The fields that bound when a record is current: 'from', the date or instant from which it is valid, which may be left out, and 'until', the date or instant after which it is expired. Both name fields of this entity with the same format, date or date-time. A record used outside its validity is refused; the validator derives that case for every operation that takes one.",
      "type": "object",
      "properties": {
        "from": {
          "description": "The field holding the date or instant from which the record is valid.",
          "$ref": "#/$defs/memberName"
        },
        "until": {
          "description": "The field holding the date or instant after which the record is expired.",
          "$ref": "#/$defs/memberName"
        }
      },
      "required": [
        "until"
      ],
      "additionalProperties": false
    },
    "permission": {
      "type": "object",
      "properties": {
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "satisfies": {
          "$ref": "#/$defs/satisfies"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "description"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "role": {
      "type": "object",
      "properties": {
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "permissions": {
          "description": "Permissions this role grants. Fail-closed: an empty list is not allowed; a role that grants nothing is left out.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/permissionName"
          },
          "minItems": 1,
          "uniqueItems": true
        },
        "satisfies": {
          "$ref": "#/$defs/satisfies"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "description",
        "permissions"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "separationOfDuties": {
      "type": "object",
      "properties": {
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "permissions": {
          "description": "The permissions of the set, each declared and none of them public.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/permissionName"
          },
          "minItems": 2,
          "uniqueItems": true
        },
        "cardinality": {
          "description": "How many of the set's permissions one holder may not reach: at least 2, at most the number the set names, and 2 when left out.",
          "type": "integer",
          "minimum": 2
        },
        "satisfies": {
          "$ref": "#/$defs/satisfies"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "description",
        "permissions"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "session": {
      "description": "SpecArch keyword. The one session object of a specification.",
      "type": "object",
      "properties": {
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "idleTimeout": {
          "description": "The time without a request after which the session expires.",
          "$ref": "#/$defs/duration"
        },
        "absoluteTimeout": {
          "description": "The time after signing in after which the session expires, whatever the caller does.",
          "$ref": "#/$defs/duration"
        },
        "satisfies": {
          "$ref": "#/$defs/satisfies"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "anyOf": [
        {
          "required": [
            "idleTimeout"
          ]
        },
        {
          "required": [
            "absoluteTimeout"
          ]
        }
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "pathItem": {
      "description": "OpenAPI keyword. The operations on one path.",
      "type": "object",
      "properties": {
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "parameters": {
          "$ref": "#/$defs/parameters"
        },
        "get": {
          "$ref": "#/$defs/operation"
        },
        "post": {
          "$ref": "#/$defs/operation"
        },
        "put": {
          "$ref": "#/$defs/operation"
        },
        "patch": {
          "$ref": "#/$defs/operation"
        },
        "delete": {
          "$ref": "#/$defs/operation"
        }
      },
      "anyOf": [
        {
          "required": [
            "get"
          ]
        },
        {
          "required": [
            "post"
          ]
        },
        {
          "required": [
            "put"
          ]
        },
        {
          "required": [
            "patch"
          ]
        },
        {
          "required": [
            "delete"
          ]
        }
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "parameters": {
      "type": "array",
      "items": {
        "$ref": "#/$defs/parameter"
      }
    },
    "parameter": {
      "description": "OpenAPI keyword.",
      "type": "object",
      "properties": {
        "name": {
          "type": "string",
          "minLength": 1
        },
        "in": {
          "type": "string",
          "enum": [
            "path",
            "query",
            "header"
          ]
        },
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "required": {
          "type": "boolean"
        },
        "schema": {
          "$ref": "#/$defs/field"
        }
      },
      "required": [
        "name",
        "in",
        "schema"
      ],
      "if": {
        "properties": {
          "in": {
            "const": "path"
          }
        }
      },
      "then": {
        "properties": {
          "required": {
            "const": true
          }
        },
        "required": [
          "required"
        ]
      },
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "operation": {
      "description": "OpenAPI keywords plus the SpecArch 'permission'. An operation without a permission is invalid: access is fail-closed.",
      "type": "object",
      "properties": {
        "operationId": {
          "$ref": "#/$defs/memberName"
        },
        "summary": {
          "type": "string",
          "minLength": 1
        },
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "permission": {
          "description": "SpecArch keyword. The one permission a caller must hold. Use 'public' for an endpoint open to everyone.",
          "$ref": "#/$defs/permissionName"
        },
        "parameters": {
          "$ref": "#/$defs/parameters"
        },
        "requestBody": {
          "$ref": "#/$defs/requestBody"
        },
        "responses": {
          "type": "object",
          "propertyNames": {
            "pattern": "^([1-5][0-9][0-9]|default)$"
          },
          "additionalProperties": {
            "$ref": "#/$defs/response"
          },
          "minProperties": 1
        },
        "emits": {
          "description": "SpecArch keyword. Channel messages this operation publishes, as 'channel/Message'.",
          "type": "array",
          "items": {
            "type": "string",
            "pattern": "^[a-z][a-z0-9]*(\\.[a-z][a-z0-9]*)*/[A-Z][A-Za-z0-9]*$"
          },
          "uniqueItems": true
        },
        "calls": {
          "description": "SpecArch keyword. The dependencies this operation calls, by their names under 'dependencies'.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/memberName"
          },
          "uniqueItems": true
        },
        "idempotencyKey": {
          "description": "SpecArch keyword. The name of a header parameter of this operation by which a repeated request is told from a new one: a request repeated with the same key is answered as the first was and has no second effect, and a different request with a key already used is refused. Only on a method that is not idempotent by itself (RFC 9110, 9.2.2): post or patch.",
          "type": "string",
          "minLength": 1
        },
        "guard": {
          "$ref": "#/$defs/guard"
        },
        "listOf": {
          "$ref": "#/$defs/listOf"
        },
        "limits": {
          "$ref": "#/$defs/limits"
        },
        "algorithm": {
          "description": "SpecArch keyword. The algorithm this operation runs, if any.",
          "$ref": "#/$defs/memberName"
        },
        "deprecated": {
          "type": "boolean"
        },
        "satisfies": {
          "$ref": "#/$defs/satisfies"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "operationId",
        "summary",
        "permission",
        "responses"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "requestBody": {
      "type": "object",
      "properties": {
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "required": {
          "type": "boolean"
        },
        "content": {
          "$ref": "#/$defs/content"
        }
      },
      "required": [
        "content"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "response": {
      "type": "object",
      "properties": {
        "description": {
          "type": "string",
          "minLength": 1
        },
        "content": {
          "$ref": "#/$defs/content"
        },
        "problem": {
          "type": "string",
          "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$",
          "description": "SpecArch keyword. The problem type this response answers with, by its name under errors. Once the specification declares errors, every 4xx and 5xx response names one."
        }
      },
      "required": [
        "description"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "content": {
      "description": "OpenAPI keyword. Media type to schema.",
      "type": "object",
      "propertyNames": {
        "pattern": "^[a-z]+/[a-z0-9.+-]+$"
      },
      "additionalProperties": {
        "type": "object",
        "properties": {
          "schema": {
            "$ref": "#/$defs/field"
          }
        },
        "required": [
          "schema"
        ],
        "propertyNames": {
          "not": {
            "$ref": "#/$defs/stackSpecificKey"
          }
        },
        "patternProperties": {
          "^x-": {}
        },
        "additionalProperties": false
      },
      "minProperties": 1
    },
    "guard": {
      "description": "SpecArch keyword. What is checked together with a data change: the entity it writes, a precondition over that entity's fields that must hold on the records as they are at the moment of the change, and the exact number of records it changes. The checks run with the change itself, so a writer who read a record before another writer changed it is refused rather than overwriting the other's change, and a change that would touch more records than expected is refused as a whole. The validator derives the cases 'guard precondition fails' and 'concurrent write'.",
      "type": "object",
      "properties": {
        "entity": {
          "description": "The entity whose records the change writes.",
          "$ref": "#/$defs/typeName"
        },
        "precondition": {
          "description": "One expression in SpecArch's subset of CEL (docs/conventions.md) over the entity's fields, giving true or false, that must hold on each record as it is when the change is made.",
          "type": "string",
          "minLength": 1
        },
        "recordsChanged": {
          "description": "The exact number of records of the entity the change writes.",
          "type": "integer",
          "minimum": 0
        }
      },
      "required": [
        "entity",
        "recordsChanged"
      ],
      "additionalProperties": false
    },
    "channel": {
      "description": "AsyncAPI keywords.",
      "type": "object",
      "properties": {
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "messages": {
          "type": "object",
          "propertyNames": {
            "$ref": "#/$defs/typeName"
          },
          "additionalProperties": {
            "$ref": "#/$defs/message"
          },
          "minProperties": 1
        },
        "satisfies": {
          "$ref": "#/$defs/satisfies"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "messages"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "message": {
      "description": "AsyncAPI keywords. One event.",
      "type": "object",
      "properties": {
        "summary": {
          "type": "string",
          "minLength": 1
        },
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "payload": {
          "$ref": "#/$defs/field"
        },
        "satisfies": {
          "$ref": "#/$defs/satisfies"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "summary",
        "payload"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "dependency": {
      "description": "SpecArch keyword. One external system an operation calls.",
      "type": "object",
      "properties": {
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "timeout": {
          "description": "The time limit of one call: after it, the call is given up and the operation answers as it does when the dependency fails.",
          "$ref": "#/$defs/duration"
        },
        "satisfies": {
          "$ref": "#/$defs/satisfies"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "description",
        "timeout"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "page": {
      "description": "SpecArch keyword. A screen of one of four kinds: a list, a form or a view of an entity's records, or a task that submits to an operation without loading a record. Generators map it onto the target project's component library.",
      "type": "object",
      "properties": {
        "kind": {
          "type": "string",
          "enum": [
            "list",
            "form",
            "view",
            "task"
          ]
        },
        "title": {
          "type": "string",
          "minLength": 1
        },
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "route": {
          "description": "Client-side route; {param} marks a path parameter.",
          "type": "string",
          "pattern": "^/"
        },
        "entity": {
          "description": "The entity whose records a list, form or view shows. A task page shows none and leaves it out.",
          "$ref": "#/$defs/typeName"
        },
        "permission": {
          "description": "Fail-closed, like an operation.",
          "$ref": "#/$defs/permissionName"
        },
        "source": {
          "description": "operationId that loads the page's data. Required for list and view; a form that creates a new row has nothing to load and leaves it out, a form that edits an existing row names the operation that loads it.",
          "$ref": "#/$defs/memberName"
        },
        "submit": {
          "description": "For a form or a task: operationId that receives it. A task's fields are properties of its request body.",
          "$ref": "#/$defs/memberName"
        },
        "columns": {
          "description": "For a list: fields shown, in order.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/memberName"
          },
          "minItems": 1,
          "uniqueItems": true
        },
        "compactColumns": {
          "description": "For a list: the columns a compact screen keeps, in order. What compact means in points or pixels, and how the row is laid out, is the stack's.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/memberName"
          },
          "minItems": 1,
          "uniqueItems": true
        },
        "fields": {
          "description": "For a form or view: fields of its entity shown, in order. For a task: properties of the submit operation's request body, every required one among them.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/memberName"
          },
          "minItems": 1,
          "uniqueItems": true
        },
        "sections": {
          "description": "For a form or a view, in place of fields: its fields in groups, each with a title, in the order a person reads them, which is the focus order (WCAG 2.2, 2.4.3). How groups sit on a screen is the stack's.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/pageSection"
          },
          "minItems": 1
        },
        "childRows": {
          "description": "For a form: the records of a one-to-many relation of its entity, edited as rows under it, one entry per relation, in the order a person reads them.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/childRows"
          },
          "minItems": 1
        },
        "filters": {
          "description": "For a list: fields the user can filter on.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/memberName"
          },
          "uniqueItems": true
        },
        "actions": {
          "type": "array",
          "items": {
            "$ref": "#/$defs/action"
          }
        },
        "onSubmitted": {
          "description": "For a form: where a successful submission leads. For a task: what each success the submit operation answers leads to, keyed by its response status.",
          "type": "object"
        },
        "onSelect": {
          "description": "For a list: where selecting a row leads.",
          "$ref": "#/$defs/pageEvent"
        },
        "pickers": {
          "description": "For a form: the fields that hold the key of another entity's record, keyed by field, each picked from a list of that record. The field is the via of a many-to-one relation of the page's entity, which names the record's entity.",
          "type": "object",
          "additionalProperties": {
            "$ref": "#/$defs/picker"
          },
          "minProperties": 1
        },
        "fieldConditions": {
          "description": "For a form or a view: when a field it shows is read-only or hidden, keyed by field. Which mode a page is in, create, edit or view, is the page itself: a form without source creates, a form with one edits, and a view shows.",
          "type": "object",
          "additionalProperties": {
            "$ref": "#/$defs/fieldCondition"
          },
          "minProperties": 1
        },
        "checks": {
          "description": "For a form: rules across its fields, checked before it is sent, keyed in kebab case.",
          "type": "object",
          "propertyNames": {
            "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
          },
          "additionalProperties": {
            "$ref": "#/$defs/formCheck"
          },
          "minProperties": 1
        },
        "enteredTwice": {
          "description": "For a form: fields a person types twice, so a mistake shows before it is sent. The second entry is compared and never sent.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/memberName"
          },
          "minItems": 1,
          "uniqueItems": true
        },
        "states": {
          "$ref": "#/$defs/pageStates"
        },
        "satisfies": {
          "$ref": "#/$defs/satisfies"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "kind",
        "title",
        "route",
        "permission"
      ],
      "allOf": [
        {
          "if": {
            "properties": {
              "kind": {
                "const": "list"
              }
            }
          },
          "then": {
            "required": [
              "columns",
              "source",
              "entity"
            ],
            "properties": {
              "onSubmitted": {
                "$ref": "#/$defs/pageEvent"
              }
            }
          }
        },
        {
          "if": {
            "properties": {
              "kind": {
                "const": "form"
              }
            }
          },
          "then": {
            "required": [
              "submit",
              "entity"
            ],
            "properties": {
              "onSubmitted": {
                "$ref": "#/$defs/pageEvent"
              }
            }
          }
        },
        {
          "if": {
            "properties": {
              "kind": {
                "const": "view"
              }
            }
          },
          "then": {
            "required": [
              "source",
              "entity"
            ],
            "properties": {
              "onSubmitted": {
                "$ref": "#/$defs/pageEvent"
              }
            }
          }
        },
        {
          "if": {
            "properties": {
              "kind": {
                "const": "task"
              }
            }
          },
          "then": {
            "required": [
              "submit"
            ],
            "properties": {
              "onSubmitted": {
                "$ref": "#/$defs/statusEvents"
              }
            }
          }
        }
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "action": {
      "type": "object",
      "properties": {
        "label": {
          "type": "string",
          "minLength": 1
        },
        "kind": {
          "type": "string",
          "enum": [
            "navigate",
            "operation"
          ]
        },
        "target": {
          "description": "A page name for navigate, an operationId for operation.",
          "type": "string",
          "minLength": 1
        },
        "permission": {
          "description": "Shown only to callers holding it; defaults to the page's permission.",
          "$ref": "#/$defs/permissionName"
        },
        "confirm": {
          "description": "Question asked before an operation runs.",
          "type": "string"
        },
        "reason": {
          "description": "For an operation with confirm: the property of its request body that carries the reason the person gives in the confirmation, a string the body requires.",
          "$ref": "#/$defs/memberName"
        },
        "when": {
          "description": "An expression over the fields of the record the action is on, a row of a list or the record a view or an edit form shows, in the expression subset; the action is offered only while it is true.",
          "type": "string",
          "minLength": 1
        },
        "then": {
          "description": "For an operation: where it leads once it succeeds.",
          "$ref": "#/$defs/pageEvent"
        }
      },
      "required": [
        "label",
        "kind",
        "target"
      ],
      "dependentRequired": {
        "reason": [
          "confirm"
        ]
      },
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "pageEvent": {
      "description": "SpecArch keyword, after the navigation flow of OMG's IFML. Where an event of a page leads: to another page with the route parameters it needs, and with a message announced to the person (WCAG 2.2, 4.1.3). Geometry and animation are a stack's.",
      "type": "object",
      "properties": {
        "navigate": {
          "description": "The page the event leads to.",
          "type": "string",
          "minLength": 1
        },
        "with": {
          "description": "The route parameters of the page navigated to, each from a field of this page's entity: the record submitted, selected or acted on. On a task page, each from a property of the body of the response the event follows.",
          "type": "object",
          "additionalProperties": {
            "$ref": "#/$defs/memberName"
          },
          "minProperties": 1
        },
        "message": {
          "description": "A status message, a full sentence, announced without moving focus.",
          "type": "string",
          "minLength": 1
        }
      },
      "minProperties": 1,
      "dependentRequired": {
        "with": [
          "navigate"
        ]
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "statusEvents": {
      "description": "SpecArch keyword. What a task page does on each success its submit operation answers, keyed by the response status, each one the operation declares. A problem the operation answers is shown under the page's failed states, not here.",
      "type": "object",
      "propertyNames": {
        "pattern": "^2[0-9][0-9]$"
      },
      "additionalProperties": {
        "$ref": "#/$defs/pageEvent"
      },
      "minProperties": 1
    },
    "flow": {
      "type": "object",
      "properties": {
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "actor": {
          "description": "The role that does the task; it must be allowed to open every page on the way.",
          "$ref": "#/$defs/roleName"
        },
        "steps": {
          "description": "The pages in order, each with the event that leads to the next one's page.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/flowStep"
          },
          "minItems": 2
        },
        "satisfies": {
          "$ref": "#/$defs/satisfies"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "description",
        "actor",
        "steps"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "flowStep": {
      "type": "object",
      "properties": {
        "page": {
          "description": "The page of the step.",
          "type": "string",
          "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
        },
        "event": {
          "description": "What the person does on it: selects a row of a list, submits a form, or takes an action.",
          "type": "string",
          "enum": [
            "select",
            "submitted",
            "action"
          ]
        },
        "action": {
          "description": "For event action: the label of the action.",
          "type": "string",
          "minLength": 1
        }
      },
      "required": [
        "page",
        "event"
      ],
      "if": {
        "properties": {
          "event": {
            "const": "action"
          }
        }
      },
      "then": {
        "required": [
          "action"
        ]
      },
      "else": {
        "not": {
          "required": [
            "action"
          ]
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "workflow": {
      "type": "object",
      "properties": {
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "trigger": {
          "description": "The operationId that starts the workflow. Its request body is the workflow's form, and it answers 202: the request is accepted and waits.",
          "$ref": "#/$defs/memberName"
        },
        "subject": {
          "description": "The entity that holds the request while it waits.",
          "$ref": "#/$defs/typeName"
        },
        "steps": {
          "description": "The steps in order, after BPMN 2.0: an approval is a user task with its potential owners and a timer, an operation a service task. A refusal at an approval ends the request.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/workflowStep"
          },
          "minItems": 1
        },
        "satisfies": {
          "$ref": "#/$defs/satisfies"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "description",
        "trigger",
        "subject",
        "steps"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "workflowStep": {
      "type": "object",
      "properties": {
        "name": {
          "description": "The step's name, unique in the workflow.",
          "$ref": "#/$defs/memberName"
        },
        "kind": {
          "description": "approval: people with the permission approve or refuse the request; operation: the system calls an operation once every approval before it has passed.",
          "type": "string",
          "enum": [
            "approval",
            "operation"
          ]
        },
        "approvers": {
          "description": "For an approval: the roles that may approve, each granting the permission.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/roleName"
          },
          "minItems": 1,
          "uniqueItems": true
        },
        "permission": {
          "description": "For an approval: the permission the approval checks; never the trigger's.",
          "$ref": "#/$defs/permissionName"
        },
        "deadline": {
          "description": "For an approval: how long the request waits for an answer.",
          "$ref": "#/$defs/duration"
        },
        "onDeadline": {
          "description": "For an approval: what the deadline does when it passes. refuse ends the request; escalate moves it to the later approval step named under escalateTo.",
          "type": "string",
          "enum": [
            "refuse",
            "escalate"
          ]
        },
        "escalateTo": {
          "description": "With onDeadline escalate: the later approval step the request moves to.",
          "$ref": "#/$defs/memberName"
        },
        "operation": {
          "description": "For an operation step: the operationId the system calls.",
          "$ref": "#/$defs/memberName"
        }
      },
      "required": [
        "name",
        "kind"
      ],
      "if": {
        "properties": {
          "kind": {
            "const": "approval"
          }
        }
      },
      "then": {
        "required": [
          "approvers",
          "permission",
          "deadline",
          "onDeadline"
        ],
        "not": {
          "required": [
            "operation"
          ]
        },
        "if": {
          "properties": {
            "onDeadline": {
              "const": "escalate"
            }
          }
        },
        "then": {
          "required": [
            "escalateTo"
          ]
        },
        "else": {
          "not": {
            "required": [
              "escalateTo"
            ]
          }
        }
      },
      "else": {
        "required": [
          "operation"
        ],
        "allOf": [
          {
            "not": {
              "required": [
                "approvers"
              ]
            }
          },
          {
            "not": {
              "required": [
                "permission"
              ]
            }
          },
          {
            "not": {
              "required": [
                "deadline"
              ]
            }
          },
          {
            "not": {
              "required": [
                "onDeadline"
              ]
            }
          },
          {
            "not": {
              "required": [
                "escalateTo"
              ]
            }
          }
        ]
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "pageStates": {
      "description": "SpecArch keyword. What a page shows in each state but its content: empty and filtered empty for a list, and failed per problem type it can meet. Loading and submitting have no text of their own, and a stack draws them its way. Without states, a stack shows its own.",
      "type": "object",
      "properties": {
        "empty": {
          "description": "For a list: what it shows when there are no records.",
          "$ref": "#/$defs/pageState"
        },
        "filteredEmpty": {
          "description": "For a list with filters: what it shows when no record matches them.",
          "$ref": "#/$defs/pageState"
        },
        "failed": {
          "description": "What the page shows when an operation it calls fails, keyed by problem type, or default for every problem type not named.",
          "type": "object",
          "propertyNames": {
            "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
          },
          "additionalProperties": {
            "$ref": "#/$defs/pageState"
          },
          "minProperties": 1
        }
      },
      "minProperties": 1,
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "pageState": {
      "type": "object",
      "properties": {
        "message": {
          "description": "A full sentence the person reads, and a screen reader announces (WCAG 2.2, 3.3.1 and 4.1.3).",
          "type": "string",
          "minLength": 1
        },
        "field": {
          "description": "For a failed state of a form: the field the problem is about, so the message shows beside it.",
          "$ref": "#/$defs/memberName"
        }
      },
      "required": [
        "message"
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "picker": {
      "description": "SpecArch keyword. A field picked from a list of the records it may hold the key of.",
      "type": "object",
      "properties": {
        "source": {
          "description": "operationId of the list the record is picked from, an operation whose listOf names the relation's target.",
          "$ref": "#/$defs/memberName"
        },
        "shows": {
          "description": "The fields of the picked record a person reads to choose it, in order.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/memberName"
          },
          "minItems": 1,
          "uniqueItems": true
        },
        "fills": {
          "description": "Other fields of the form set from the picked record: the form's field, then the record's field of the same type.",
          "type": "object",
          "additionalProperties": {
            "$ref": "#/$defs/memberName"
          },
          "minProperties": 1
        }
      },
      "required": [
        "source",
        "shows"
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "fieldCondition": {
      "description": "SpecArch keyword. When a field a page shows is read-only or hidden. Each expression is over the fields of the record, in the expression subset.",
      "type": "object",
      "properties": {
        "readOnly": {
          "description": "For a form: the field is shown and never changed on this page.",
          "const": true
        },
        "readOnlyWhen": {
          "description": "For a form: the field is shown and not changed while this is true.",
          "type": "string",
          "minLength": 1
        },
        "hiddenWhen": {
          "description": "The field is not shown while this is true.",
          "type": "string",
          "minLength": 1
        }
      },
      "minProperties": 1,
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "formCheck": {
      "description": "SpecArch keyword. A rule across a form's fields, an expression over the fields it shows that must be true before it is sent.",
      "type": "object",
      "properties": {
        "expression": {
          "type": "string",
          "minLength": 1
        },
        "message": {
          "description": "A full sentence the person reads when the rule does not hold.",
          "type": "string",
          "minLength": 1
        },
        "field": {
          "description": "The field the message shows beside.",
          "$ref": "#/$defs/memberName"
        }
      },
      "required": [
        "expression",
        "message"
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "pageSection": {
      "type": "object",
      "properties": {
        "title": {
          "description": "The heading of the group.",
          "type": "string",
          "minLength": 1
        },
        "fields": {
          "type": "array",
          "items": {
            "$ref": "#/$defs/memberName"
          },
          "minItems": 1,
          "uniqueItems": true
        }
      },
      "required": [
        "title",
        "fields"
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "childRows": {
      "description": "SpecArch keyword. The records of one relation of a form's entity, edited as rows under the form: added, changed and removed with it. Each row is validated by the target entity's own schema.",
      "type": "object",
      "properties": {
        "relation": {
          "description": "A one-to-many relation of the form's entity; its target is the entity of each row.",
          "$ref": "#/$defs/memberName"
        },
        "title": {
          "description": "The heading of the rows.",
          "type": "string",
          "minLength": 1
        },
        "fields": {
          "description": "The fields of the relation's target shown in each row, in order.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/memberName"
          },
          "minItems": 1,
          "uniqueItems": true
        },
        "maximum": {
          "description": "The most rows the form holds, loaded and new together; a row past it is refused.",
          "type": "integer",
          "minimum": 1,
          "maximum": 2147483647
        },
        "lockLoadedRows": {
          "description": "true: the rows loaded with the record are shown and cannot be changed or removed; only rows added on the form change.",
          "const": true
        }
      },
      "required": [
        "relation",
        "title",
        "fields"
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "tokenGroup": {
      "description": "A group of design tokens, after the W3C Design Tokens Community Group's Format Module 2025.10: $type gives the type of every token it holds that does not give its own, and every other key is a token, which has a $value, or a group.",
      "type": "object",
      "properties": {
        "$type": {
          "type": "string",
          "minLength": 1
        },
        "$description": {
          "type": "string"
        },
        "$extensions": {
          "description": "The format's extensions, keyed by the reverse domain of the tool that writes them; SpecArch keeps them and reads none.",
          "type": "object"
        },
        "$deprecated": {
          "description": "The format's deprecation: true, or a sentence saying what to use instead.",
          "type": [
            "boolean",
            "string"
          ]
        }
      },
      "patternProperties": {
        "^[^${}.][^{}.]*$": {
          "if": {
            "type": "object",
            "required": [
              "$value"
            ]
          },
          "then": {
            "$ref": "#/$defs/designToken"
          },
          "else": {
            "$ref": "#/$defs/tokenGroup"
          }
        }
      },
      "additionalProperties": false
    },
    "designToken": {
      "description": "A design token: its $value, of its type or an alias of another token written {group.token}, its $type when no group gives it and it is not an alias, a $description, and the format's $extensions and $deprecated.",
      "type": "object",
      "properties": {
        "$value": {},
        "$type": {
          "type": "string",
          "minLength": 1
        },
        "$description": {
          "type": "string"
        },
        "$extensions": {
          "description": "The format's extensions, keyed by the reverse domain of the tool that writes them; SpecArch keeps them and reads none.",
          "type": "object"
        },
        "$deprecated": {
          "description": "The format's deprecation: true, or a sentence saying what to use instead.",
          "type": [
            "boolean",
            "string"
          ]
        }
      },
      "required": [
        "$value"
      ],
      "additionalProperties": false
    },
    "colorPair": {
      "type": "object",
      "properties": {
        "text": {
          "description": "The colour token used as text or as the part of a control, by its path.",
          "type": "string",
          "minLength": 1
        },
        "background": {
          "description": "The colour token it is shown on.",
          "type": "string",
          "minLength": 1
        },
        "use": {
          "description": "text asks 4.5:1 at AA (1.4.3) and 7:1 at AAA (1.4.6); largeText, 18 point or 14 point bold, asks 3:1 and 4.5:1; control, the parts of a control and graphics, asks 3:1 (1.4.11).",
          "type": "string",
          "enum": [
            "text",
            "largeText",
            "control"
          ]
        }
      },
      "required": [
        "text",
        "background",
        "use"
      ],
      "additionalProperties": false
    },
    "algorithm": {
      "description": "SpecArch keyword. All three of formula, examples and pseudocode are required; the examples become test cases.",
      "type": "object",
      "properties": {
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "inputs": {
          "type": "object",
          "propertyNames": {
            "$ref": "#/$defs/memberName"
          },
          "additionalProperties": {
            "$ref": "#/$defs/field"
          },
          "minProperties": 1
        },
        "output": {
          "$ref": "#/$defs/field"
        },
        "formula": {
          "description": "The computation as one expression in SpecArch's subset of CEL (docs/conventions.md). It is type-checked against the inputs' declared types, with no implicit conversion, and its result must have the output's type. The validator evaluates it on every worked example.",
          "type": "string",
          "minLength": 1
        },
        "examples": {
          "type": "array",
          "items": {
            "$ref": "#/$defs/workedExample"
          },
          "minItems": 1
        },
        "pseudocode": {
          "type": "string",
          "minLength": 1
        },
        "satisfies": {
          "$ref": "#/$defs/satisfies"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "inputs",
        "output",
        "formula",
        "examples",
        "pseudocode"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "workedExample": {
      "type": "object",
      "properties": {
        "name": {
          "type": "string",
          "minLength": 1
        },
        "inputs": {
          "type": "object",
          "propertyNames": {
            "$ref": "#/$defs/memberName"
          },
          "minProperties": 1
        },
        "expected": {},
        "note": {
          "description": "Why this case matters or how the number was reached.",
          "type": "string"
        }
      },
      "required": [
        "name",
        "inputs",
        "expected"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "decision": {
      "description": "SpecArch keyword. A short architecture decision record: the context (what was true and at stake), the decision, its consequences, and under 'why' the reasoning that led from the one to the other.",
      "type": "object",
      "properties": {
        "title": {
          "type": "string",
          "minLength": 1
        },
        "status": {
          "type": "string",
          "enum": [
            "proposed",
            "accepted",
            "deprecated",
            "superseded"
          ]
        },
        "date": {
          "type": "string",
          "format": "date"
        },
        "context": {
          "$ref": "#/$defs/markdown"
        },
        "decision": {
          "$ref": "#/$defs/markdown"
        },
        "consequences": {
          "$ref": "#/$defs/markdown"
        },
        "supersededBy": {
          "type": "string",
          "pattern": "^ADR-[0-9]{3,}$"
        },
        "answers": {
          "description": "SpecArch keyword. The open questions this decision answers, by ID. The questions themselves are removed from the specification once answered, so these IDs are text, not references; the validator refuses an accepted decision that answers a question still present.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/questionId"
          },
          "minItems": 1,
          "uniqueItems": true
        },
        "decidedBy": {
          "description": "SpecArch keyword. The stakeholder who decided, by key; a role, never a person. Required when the decision answers questions.",
          "type": "string",
          "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
        },
        "satisfies": {
          "$ref": "#/$defs/satisfies"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "title",
        "status",
        "date",
        "context",
        "decision",
        "consequences"
      ],
      "if": {
        "properties": {
          "status": {
            "const": "superseded"
          }
        }
      },
      "then": {
        "required": [
          "supersededBy"
        ]
      },
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false,
      "dependentRequired": {
        "answers": [
          "decidedBy"
        ]
      }
    },
    "question": {
      "description": "SpecArch keyword. One open question: something the sources do not say and a stakeholder must decide or provide. ISO/IEC/IEEE 29148:2018 (5.2.6) says a complete set of requirements holds no 'to be defined' clause and that resolving them is iterative within a time set by risk; IEEE Std 830-1998 (4.3.3) says such an item carries why it is open, what must be done, who is responsible and by when. A question is the one licence for an element to be incomplete: a required key may be missing exactly where a must question says it is unknown.",
      "type": "object",
      "properties": {
        "question": {
          "description": "What is asked, as one plain question.",
          "type": "string",
          "minLength": 1
        },
        "kind": {
          "description": "decision: the stakeholder decides; material: the stakeholder provides something to read, such as a document, a file, a screenshot or a record.",
          "type": "string",
          "enum": [
            "decision",
            "material"
          ]
        },
        "priority": {
          "description": "must: what it blocks is not defined, and nothing that reads it can be generated; should: what it blocks is written as inferred and the stakeholder should confirm it before it is built on; could: the answer would improve the specification, but nothing waits for it.",
          "type": "string",
          "enum": [
            "must",
            "should",
            "could"
          ]
        },
        "blocks": {
          "description": "What cannot be final until the question is answered: a stage name, a section name, or a pointer to an element (#/entities/Loan, #/paths/~1loans/post) or to one key of it (#/requirements/BR-3/priority). A must question covers the required keys missing at or under an element pointer, and the one key of a key pointer.",
          "type": "array",
          "items": {
            "type": "string",
            "minLength": 1
          },
          "minItems": 1,
          "uniqueItems": true
        },
        "decidedBy": {
          "description": "The stakeholder who decides or provides, by key; a role, never a person.",
          "type": "string",
          "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
        },
        "options": {
          "description": "For a decision with a known set of answers: the answers, two or more, so that the question can be put as a choice.",
          "type": "array",
          "items": {
            "type": "string",
            "minLength": 1
          },
          "minItems": 2,
          "uniqueItems": true
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        }
      },
      "required": [
        "question",
        "kind",
        "priority",
        "blocks",
        "decidedBy"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "commandName": {
      "description": "SpecArch keyword. One or more kebab-case words separated by single spaces, as typed after the program name.",
      "type": "string",
      "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*( [a-z][a-z0-9]*(-[a-z0-9]+)*)*$"
    },
    "command": {
      "description": "SpecArch keyword. One command of a command-line interface. Design only: how the arguments are parsed and which library does it belong in an implementation file.",
      "type": "object",
      "properties": {
        "summary": {
          "type": "string",
          "minLength": 1
        },
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "permission": {
          "description": "SpecArch keyword. The one permission a caller must hold. A local tool that anyone may run uses 'public'.",
          "$ref": "#/$defs/permissionName"
        },
        "arguments": {
          "description": "Positional arguments, in order.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/commandArgument"
          }
        },
        "options": {
          "description": "Named options, keyed by the long name without its leading dashes.",
          "type": "object",
          "propertyNames": {
            "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
          },
          "additionalProperties": {
            "$ref": "#/$defs/commandOption"
          }
        },
        "reads": {
          "description": "Files and folders the command reads.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/fileUse"
          }
        },
        "writes": {
          "description": "Files and folders the command writes. A command that writes nothing leaves this out.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/fileUse"
          }
        },
        "standardOutput": {
          "description": "What the command prints on standard output.",
          "$ref": "#/$defs/markdown"
        },
        "standardError": {
          "description": "What the command prints on standard error.",
          "$ref": "#/$defs/markdown"
        },
        "exitCodes": {
          "description": "Every exit status the command can return, keyed by number. Status 0 is required.",
          "type": "object",
          "propertyNames": {
            "pattern": "^(0|[1-9][0-9]?|1[0-9][0-9]|2[0-4][0-9]|25[0-5])$"
          },
          "additionalProperties": {
            "type": "string",
            "minLength": 1
          },
          "required": [
            "0"
          ]
        },
        "guard": {
          "$ref": "#/$defs/guard"
        },
        "algorithm": {
          "description": "SpecArch keyword. The algorithm this command runs, if any.",
          "$ref": "#/$defs/memberName"
        },
        "deprecated": {
          "type": "boolean"
        },
        "satisfies": {
          "$ref": "#/$defs/satisfies"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "summary",
        "permission",
        "exitCodes"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "commandArgument": {
      "type": "object",
      "properties": {
        "name": {
          "type": "string",
          "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
        },
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "required": {
          "type": "boolean"
        },
        "repeatable": {
          "description": "The argument takes one or more values; only the last argument may.",
          "type": "boolean"
        },
        "schema": {
          "$ref": "#/$defs/field"
        }
      },
      "required": [
        "name",
        "schema"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "commandOption": {
      "type": "object",
      "properties": {
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "schema": {
          "$ref": "#/$defs/field"
        },
        "required": {
          "type": "boolean"
        }
      },
      "required": [
        "description",
        "schema"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "fileUse": {
      "type": "object",
      "properties": {
        "path": {
          "description": "A path or a pattern; {name} marks a part taken from an argument or option.",
          "type": "string",
          "minLength": 1
        },
        "description": {
          "$ref": "#/$defs/markdown"
        }
      },
      "required": [
        "path",
        "description"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "stackSpecificKey": {
      "description": "Extension keys that belong to one implementation, not to the design: code-generator settings, a language or framework binding, servers, hosts, ports and deployments. They go in a SpecArch Implementation File (*.specarch-implementation.yaml), never in a design file. See 'Design and implementation' in docs/conventions.md.",
      "type": "string",
      "pattern": "^x-(oapi-codegen|ogen|openapi-generator|codegen|protoc|grpc|go|java|kotlin|python|typescript|javascript|rust|swift|dotnet|csharp|php|ruby|framework|router|middleware|cli-library|server|servers|host|port|deploy|deployment|environment)(-|$)"
    },
    "test": {
      "description": "SpecArch keyword. One test scenario. Exactly one subject: operation, command, page, job, flow, workflow, requirement, or entity, alone for its state machine or with one constraint or transition. In a folder tree it is the content of tests/<name>/test.yaml.",
      "type": "object",
      "properties": {
        "operation": {
          "description": "The operationId under test.",
          "$ref": "#/$defs/memberName"
        },
        "command": {
          "description": "The command under test.",
          "$ref": "#/$defs/commandName"
        },
        "page": {
          "description": "The page under test.",
          "type": "string",
          "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
        },
        "job": {
          "description": "The job the test is about.",
          "$ref": "#/$defs/memberName"
        },
        "workflow": {
          "description": "The workflow the test is about.",
          "type": "string",
          "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
        },
        "flow": {
          "description": "The flow the test walks.",
          "type": "string",
          "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
        },
        "requirement": {
          "description": "The requirement whose acceptance criteria the test shows met: its cases are 'acceptance 1', 'acceptance 2' and so on, one per criterion, and the test is usually level acceptance.",
          "$ref": "#/$defs/idKey"
        },
        "entity": {
          "description": "The entity under test: alone, its state machine, whose cases are its paths from an initial to a terminal state such as 'open to overdue to returned'; with constraint or transition, that one.",
          "$ref": "#/$defs/typeName"
        },
        "constraint": {
          "description": "With entity: the constraint under test.",
          "type": "string",
          "pattern": "^[a-z][a-z0-9]*(_[a-z0-9]+)*$"
        },
        "transition": {
          "description": "With entity: the transition under test.",
          "type": "object",
          "properties": {
            "from": {
              "type": "string",
              "minLength": 1
            },
            "to": {
              "type": "string",
              "minLength": 1
            }
          },
          "required": [
            "from",
            "to"
          ],
          "additionalProperties": false
        },
        "scenario": {
          "description": "golden for the path that succeeds, red for a path that fails.",
          "type": "string",
          "enum": [
            "golden",
            "red"
          ]
        },
        "level": {
          "description": "system: the test exercises the system through its interfaces as a client would; acceptance: it shows a stakeholder that a requirement is met, so it names one under 'verifies'. Unit and integration tests belong to an implementation file's suites. The levels are those of ISO/IEC/IEEE 29119-1.",
          "type": "string",
          "enum": [
            "system",
            "acceptance"
          ]
        },
        "given": {
          "description": "The state before, in plain words.",
          "type": "string",
          "minLength": 1
        },
        "when": {
          "description": "What happens, in plain words.",
          "type": "string",
          "minLength": 1
        },
        "then": {
          "description": "The outcome, in plain words.",
          "type": "string",
          "minLength": 1
        },
        "fixture": {
          "description": "SpecArch keyword. The state before the test, for a generator: caller, the role making the call or public, and records that exist, by entity name.",
          "type": "object",
          "properties": {
            "caller": {
              "type": "string",
              "minLength": 1
            }
          },
          "propertyNames": {
            "anyOf": [
              {
                "const": "caller"
              },
              {
                "$ref": "#/$defs/typeName"
              },
              {
                "pattern": "^x-"
              }
            ]
          },
          "additionalProperties": {
            "type": "array",
            "items": {
              "type": "object"
            },
            "minItems": 1
          }
        },
        "input": {
          "description": "SpecArch keyword. What the call carries, for a generator: an operation's parameters and body fields by name; a command's arguments and options; a page's route parameters and action, the label of one of its actions.",
          "type": "object",
          "minProperties": 1
        },
        "expect": {
          "description": "SpecArch keyword. The outcome, for a generator: an operation's status and the body fields that matter; a command's exit and lines of standardOutput; the state after, as records by entity name; and emits, the messages published, or emitsNothing.",
          "type": "object",
          "properties": {
            "status": {
              "description": "One of the operation's response statuses.",
              "type": "integer",
              "minimum": 100,
              "maximum": 599
            },
            "body": {
              "description": "Fields of the entity the response returns, with their values.",
              "type": "object",
              "minProperties": 1
            },
            "exit": {
              "description": "One of the command's exit codes.",
              "type": "integer",
              "minimum": 0,
              "maximum": 255
            },
            "standardOutput": {
              "description": "Lines the command prints, in order.",
              "type": "array",
              "items": {
                "type": "string"
              },
              "minItems": 1
            },
            "state": {
              "$ref": "#/$defs/testRecords"
            },
            "emits": {
              "description": "The messages published, as channel/Message.",
              "type": "array",
              "items": {
                "type": "string",
                "pattern": "^[a-z][a-z0-9]*(\\.[a-z][a-z0-9]*)*/[A-Z][A-Za-z0-9]*$"
              },
              "minItems": 1
            },
            "emitsNothing": {
              "description": "No message is published.",
              "type": "boolean",
              "const": true
            }
          },
          "minProperties": 1,
          "patternProperties": {
            "^x-": {}
          },
          "additionalProperties": false
        },
        "covers": {
          "description": "The derived cases this test covers, written exactly as the validator names them, such as 'missing memberId'.",
          "type": "array",
          "items": {
            "type": "string",
            "minLength": 1
          },
          "minItems": 1,
          "uniqueItems": true
        },
        "notApplicable": {
          "description": "Instead of given, when and then: why the derived cases in covers do not apply to this subject. Only with a written reason.",
          "type": "string",
          "minLength": 1
        },
        "verifies": {
          "$ref": "#/$defs/verifies"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "scenario",
        "level"
      ],
      "oneOf": [
        {
          "required": [
            "operation"
          ]
        },
        {
          "required": [
            "command"
          ]
        },
        {
          "required": [
            "page"
          ]
        },
        {
          "required": [
            "job"
          ]
        },
        {
          "required": [
            "workflow"
          ]
        },
        {
          "required": [
            "flow"
          ]
        },
        {
          "required": [
            "requirement"
          ]
        },
        {
          "required": [
            "entity"
          ]
        }
      ],
      "allOf": [
        {
          "if": {
            "required": [
              "entity"
            ]
          },
          "then": {
            "not": {
              "required": [
                "constraint",
                "transition"
              ]
            }
          }
        },
        {
          "dependentRequired": {
            "constraint": [
              "entity"
            ],
            "transition": [
              "entity"
            ]
          }
        },
        {
          "if": {
            "required": [
              "notApplicable"
            ]
          },
          "then": {
            "required": [
              "covers"
            ],
            "not": {
              "anyOf": [
                {
                  "required": [
                    "given"
                  ]
                },
                {
                  "required": [
                    "when"
                  ]
                },
                {
                  "required": [
                    "then"
                  ]
                }
              ]
            }
          },
          "else": {
            "required": [
              "given",
              "when",
              "then"
            ]
          }
        }
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "step": {
      "description": "SpecArch keyword. One step of a procedure or a check.",
      "type": "object",
      "properties": {
        "name": {
          "description": "A short name for the step, such as 'Tag the release'.",
          "type": "string",
          "minLength": 1
        },
        "action": {
          "description": "What is done, in plain words. The exact command belongs to the implementation file.",
          "type": "string",
          "minLength": 1
        },
        "check": {
          "description": "How the person running it knows the step worked.",
          "type": "string",
          "minLength": 1
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "name",
        "action"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "steps": {
      "type": "array",
      "items": {
        "$ref": "#/$defs/step"
      },
      "minItems": 1
    },
    "procedure": {
      "description": "SpecArch keyword. An ordered procedure: a release or a rollback.",
      "type": "object",
      "properties": {
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "steps": {
          "$ref": "#/$defs/steps"
        },
        "satisfies": {
          "$ref": "#/$defs/satisfies"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "steps"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "environment": {
      "description": "SpecArch keyword. One environment the system is installed in.",
      "type": "object",
      "properties": {
        "description": {
          "description": "What the environment is for and who uses it.",
          "type": "string",
          "minLength": 1
        },
        "promotesTo": {
          "description": "The environment a release goes to after it has passed here, by key. The last environment leaves it out.",
          "type": "string",
          "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
        },
        "satisfies": {
          "$ref": "#/$defs/satisfies"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "description"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "setting": {
      "description": "SpecArch keyword. One run-time setting.",
      "type": "object",
      "properties": {
        "description": {
          "description": "What the setting controls and where its value comes from.",
          "type": "string",
          "minLength": 1
        },
        "secret": {
          "description": "True when the value must never be written in a specification or a document: a password, a key, a token. The validator refuses a default or a deployment value for a secret.",
          "type": "boolean"
        },
        "schema": {
          "$ref": "#/$defs/field"
        },
        "satisfies": {
          "$ref": "#/$defs/satisfies"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "description",
        "secret",
        "schema"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "migration": {
      "description": "SpecArch keyword. One data migration a release carries.",
      "type": "object",
      "properties": {
        "description": {
          "type": "string",
          "minLength": 1
        },
        "steps": {
          "$ref": "#/$defs/steps"
        },
        "rollback": {
          "description": "How the data change is reversed. Left out only when the description says why it cannot be.",
          "$ref": "#/$defs/steps"
        },
        "satisfies": {
          "$ref": "#/$defs/satisfies"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "description",
        "steps"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "check": {
      "description": "SpecArch keyword. One commissioning check on the installed system.",
      "type": "object",
      "properties": {
        "kind": {
          "description": "smoke: the installed system starts and answers; end-to-end: a whole use goes through on the real infrastructure; performance: it meets its load and time targets; security: access and exposure are as designed; data: the data it holds or migrated is right.",
          "type": "string",
          "enum": [
            "smoke",
            "end-to-end",
            "performance",
            "security",
            "data"
          ]
        },
        "description": {
          "type": "string",
          "minLength": 1
        },
        "environment": {
          "description": "The environment the check runs in, by key.",
          "type": "string",
          "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
        },
        "steps": {
          "$ref": "#/$defs/steps"
        },
        "verifies": {
          "$ref": "#/$defs/verifies"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "kind",
        "description",
        "environment",
        "steps"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "monitor": {
      "description": "SpecArch keyword. One monitor of the live system.",
      "type": "object",
      "properties": {
        "description": {
          "description": "What is measured, in plain words.",
          "type": "string",
          "minLength": 1
        },
        "environment": {
          "description": "The environment the monitor watches, by key.",
          "type": "string",
          "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
        },
        "objective": {
          "description": "What the measurement must stay within, as one verifiable sentence, such as 'nine calls in ten answer within 300 ms over a day'.",
          "type": "string",
          "minLength": 1
        },
        "verifies": {
          "$ref": "#/$defs/verifies"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "description",
        "environment",
        "objective"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "signoff": {
      "description": "SpecArch keyword. The acceptance of the installed system.",
      "type": "object",
      "properties": {
        "criteria": {
          "description": "What must be true for the system to be accepted, one plain sentence each.",
          "type": "array",
          "items": {
            "type": "string",
            "minLength": 1
          },
          "minItems": 1
        },
        "signers": {
          "description": "The roles that sign the acceptance, never people's names.",
          "type": "array",
          "items": {
            "type": "object",
            "properties": {
              "role": {
                "type": "string",
                "minLength": 1
              },
              "description": {
                "$ref": "#/$defs/markdown"
              }
            },
            "required": [
              "role"
            ],
            "propertyNames": {
              "not": {
                "$ref": "#/$defs/stackSpecificKey"
              }
            },
            "patternProperties": {
              "^x-": {}
            },
            "additionalProperties": false
          },
          "minItems": 1
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "criteria",
        "signers"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "testRecords": {
      "description": "SpecArch keyword. Records by entity name: each a list of records, each naming the fields that matter with values of the field's type.",
      "type": "object",
      "propertyNames": {
        "$ref": "#/$defs/typeName"
      },
      "additionalProperties": {
        "type": "array",
        "items": {
          "type": "object"
        },
        "minItems": 1
      },
      "minProperties": 1
    },
    "listOf": {
      "description": "SpecArch keyword. The operation answers a page of an entity's records, or of a view's rows: which fields a free-text search covers, which may be filtered and sorted by, and the page size. A client must know the lists, and a request outside them is refused, never ignored. The parameter names and the answer's envelope are the paginated-list idiom.",
      "type": "object",
      "properties": {
        "entity": {
          "description": "The entity listed.",
          "$ref": "#/$defs/typeName"
        },
        "view": {
          "description": "The view listed, in place of an entity; the whitelists then name the view's fields.",
          "$ref": "#/$defs/typeName"
        },
        "searchable": {
          "type": "array",
          "items": {
            "$ref": "#/$defs/memberName"
          },
          "minItems": 1,
          "uniqueItems": true,
          "description": "The fields a free-text search matches."
        },
        "filterable": {
          "type": "array",
          "items": {
            "$ref": "#/$defs/memberName"
          },
          "minItems": 1,
          "uniqueItems": true,
          "description": "The fields a filter may name."
        },
        "sortable": {
          "type": "array",
          "items": {
            "$ref": "#/$defs/memberName"
          },
          "minItems": 1,
          "uniqueItems": true,
          "description": "The fields the list may be sorted by."
        },
        "pageSize": {
          "description": "How many records a page holds.",
          "type": "object",
          "properties": {
            "default": {
              "type": "integer",
              "minimum": 1,
              "maximum": 2147483647
            },
            "maximum": {
              "description": "The largest page a request may ask for; above it the request is refused.",
              "type": "integer",
              "minimum": 1,
              "maximum": 2147483647
            }
          },
          "required": [
            "maximum"
          ],
          "additionalProperties": false
        }
      },
      "required": [
        "pageSize"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false,
      "oneOf": [
        {
          "required": [
            "entity"
          ]
        },
        {
          "required": [
            "view"
          ]
        }
      ]
    },
    "limits": {
      "description": "SpecArch keyword. Limits a client must keep to: the largest request body, and how many requests are taken in a time. The runtime side is the rate-limit idiom.",
      "type": "object",
      "properties": {
        "maxRequestBytes": {
          "description": "The largest request body, in bytes; a larger one is refused.",
          "type": "integer",
          "minimum": 1,
          "maximum": 9007199254740991
        },
        "rate": {
          "description": "How many requests one caller may make in a time, and how many at once above that.",
          "type": "object",
          "properties": {
            "requests": {
              "type": "integer",
              "minimum": 1,
              "maximum": 2147483647
            },
            "per": {
              "$ref": "#/$defs/duration"
            },
            "burst": {
              "type": "integer",
              "minimum": 1,
              "maximum": 2147483647
            }
          },
          "required": [
            "requests",
            "per"
          ],
          "additionalProperties": false
        }
      },
      "minProperties": 1,
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "problemType": {
      "description": "SpecArch keyword. One kind of problem an operation answers with, in the shape of RFC 9457, Problem Details for HTTP APIs: its HTTP status, a short title that does not change between occurrences, and the condition under which it is answered.",
      "type": "object",
      "properties": {
        "status": {
          "type": "integer",
          "minimum": 400,
          "maximum": 599
        },
        "title": {
          "type": "string",
          "minLength": 1
        },
        "condition": {
          "$ref": "#/$defs/markdown"
        },
        "type": {
          "description": "The problem type's URI, when it is not derived from the name.",
          "type": "string",
          "minLength": 1
        },
        "satisfies": {
          "$ref": "#/$defs/satisfies"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "status",
        "title",
        "condition"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "job": {
      "description": "SpecArch keyword. Work the system does on its own: on a schedule, at an interval, or for each message it consumes. It acts as a role, reads and writes entities, may call dependencies and publish messages, and retries a failed item a bounded number of times.",
      "type": "object",
      "properties": {
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "trigger": {
          "description": "What starts the job: exactly one of schedule, a cron expression of five fields in UTC; every, a duration between runs; consumes, the channel/Message it handles one at a time.",
          "type": "object",
          "properties": {
            "schedule": {
              "type": "string",
              "pattern": "^\\S+( \\S+){4}$"
            },
            "every": {
              "$ref": "#/$defs/duration"
            },
            "consumes": {
              "type": "string",
              "pattern": "^[a-z][a-z0-9]*(\\.[a-z][a-z0-9]*)*/[A-Z][A-Za-z0-9]*$"
            }
          },
          "minProperties": 1,
          "maxProperties": 1,
          "additionalProperties": false
        },
        "role": {
          "description": "The role the job acts as; its permissions are what the job may do.",
          "$ref": "#/$defs/roleName"
        },
        "reads": {
          "description": "The entities the job reads.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/typeName"
          },
          "uniqueItems": true
        },
        "writes": {
          "description": "The entities the job writes.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/typeName"
          },
          "uniqueItems": true
        },
        "calls": {
          "description": "The dependencies the job calls.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/memberName"
          },
          "uniqueItems": true
        },
        "emits": {
          "description": "The messages the job publishes, as channel/Message.",
          "type": "array",
          "items": {
            "type": "string",
            "pattern": "^[a-z][a-z0-9]*(\\.[a-z][a-z0-9]*)*/[A-Z][A-Za-z0-9]*$"
          },
          "uniqueItems": true
        },
        "retries": {
          "description": "How often a failed item is tried again, and what becomes of it after the last try: deadLetter keeps it apart for a person, discard drops it.",
          "type": "object",
          "properties": {
            "limit": {
              "type": "integer",
              "minimum": 1,
              "maximum": 1000
            },
            "then": {
              "type": "string",
              "enum": [
                "deadLetter",
                "discard"
              ]
            }
          },
          "required": [
            "limit",
            "then"
          ],
          "additionalProperties": false
        },
        "satisfies": {
          "$ref": "#/$defs/satisfies"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "description",
        "trigger",
        "role"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "menuItem": {
      "description": "SpecArch keyword. One entry of a menu: a leaf that opens a page, or a group of entries.",
      "type": "object",
      "properties": {
        "title": {
          "type": "string",
          "minLength": 1
        },
        "page": {
          "description": "The page the entry opens; it is shown to who may open the page.",
          "type": "string",
          "minLength": 1
        },
        "items": {
          "type": "object",
          "propertyNames": {
            "$ref": "#/$defs/memberName"
          },
          "additionalProperties": {
            "$ref": "#/$defs/menuItem"
          },
          "minProperties": 1
        }
      },
      "required": [
        "title"
      ],
      "oneOf": [
        {
          "required": [
            "page"
          ]
        },
        {
          "required": [
            "items"
          ]
        }
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "valueObject": {
      "description": "An object schema with no key: JSON Schema's properties and required, and nothing that only a stored record has (a primary key, relations, constraints, states).",
      "type": "object",
      "properties": {
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "type": {
          "type": "string",
          "const": "object"
        },
        "properties": {
          "description": "JSON Schema keyword. The schema's fields.",
          "type": "object",
          "propertyNames": {
            "$ref": "#/$defs/memberName"
          },
          "additionalProperties": {
            "$ref": "#/$defs/field"
          },
          "minProperties": 1
        },
        "required": {
          "type": "array",
          "items": {
            "$ref": "#/$defs/memberName"
          },
          "uniqueItems": true
        },
        "satisfies": {
          "$ref": "#/$defs/satisfies"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "type",
        "properties"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "view": {
      "description": "SpecArch keyword. A read model: the entity it reads from, whose every field it carries, and the properties it adds. It may be the item of a response and the subject of a list, never the body of a request.",
      "type": "object",
      "properties": {
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "from": {
          "description": "The entity the view reads from; the view holds one row per record of it.",
          "$ref": "#/$defs/typeName"
        },
        "properties": {
          "description": "The fields the view adds, keyed by camelCase name; none may repeat a field of the entity.",
          "type": "object",
          "propertyNames": {
            "$ref": "#/$defs/memberName"
          },
          "additionalProperties": {
            "$ref": "#/$defs/viewProperty"
          },
          "minProperties": 1
        },
        "satisfies": {
          "$ref": "#/$defs/satisfies"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "from",
        "properties"
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "viewProperty": {
      "description": "A field a view adds: exactly one of path, relations separated by dots that each lead to one record and end in a field of the last, or count, a relation of the entity that leads to many records. A path has its field's type and is null when a relation has no record; a count is a 64-bit integer and leaves out softly deleted records.",
      "type": "object",
      "properties": {
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "path": {
          "type": "string",
          "pattern": "^[a-z][A-Za-z0-9]*(\\.[a-z][A-Za-z0-9]*)+$"
        },
        "count": {
          "$ref": "#/$defs/memberName"
        }
      },
      "oneOf": [
        {
          "required": [
            "path"
          ]
        },
        {
          "required": [
            "count"
          ]
        }
      ],
      "propertyNames": {
        "not": {
          "$ref": "#/$defs/stackSpecificKey"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    }
  }
}
"""#

let implementationSchemaJSON = #"""
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-implementation-0.1.schema.json",
  "title": "SpecArch Implementation File, meta-model 0.1",
  "description": "A SpecArch Implementation File (<name>.<stack>.specarch-implementation.yaml) says how one stack implements one SpecArch specification: the language and toolchain, the libraries and their licences, the package layout, how each design object maps onto the stack, the document and code targets with their settings, the build and test tasks, the deployments, and the implementation decisions. It points at the specification's root file (specarch.yaml) and cannot add or change design: entities, operations, commands, events, permissions, pages and algorithms exist only in the specification, and this schema has no keyword for them. One specification can have several implementation files, one per stack, each in implementation/<stack>/ of the tree. Every object rejects unknown keys; keys starting with 'x-' are allowed everywhere as extensions.",
  "type": "object",
  "properties": {
    "specarchImplementation": {
      "description": "Meta-model version this file is written against.",
      "type": "string",
      "const": "0.1"
    },
    "info": {
      "description": "What this implementation is.",
      "type": "object",
      "properties": {
        "title": {
          "type": "string",
          "minLength": 1
        },
        "version": {
          "description": "Version of this implementation file, semantic versioning.",
          "type": "string",
          "pattern": "^[0-9]+\\.[0-9]+\\.[0-9]+(-[0-9A-Za-z.-]+)?$"
        },
        "description": {
          "$ref": "#/$defs/markdown"
        }
      },
      "required": [
        "title",
        "version"
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "implements": {
      "description": "The specification this file implements.",
      "type": "object",
      "properties": {
        "file": {
          "description": "Path of the specification's root file, specarch.yaml, relative to this file.",
          "type": "string",
          "pattern": "(^|/)specarch\\.yaml$"
        },
        "version": {
          "description": "The specification's info.version this implementation was written against. The validator fails when they differ, so a design change is noticed by every implementation of it.",
          "type": "string",
          "pattern": "^[0-9]+\\.[0-9]+\\.[0-9]+(-[0-9A-Za-z.-]+)?$"
        }
      },
      "required": [
        "file",
        "version"
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "stack": {
      "description": "The stack: the language and toolchain with their versions, and the platforms the build targets.",
      "type": "object",
      "properties": {
        "language": {
          "$ref": "#/$defs/tool"
        },
        "toolchain": {
          "$ref": "#/$defs/tool"
        },
        "platforms": {
          "description": "Operating systems and architectures the build targets, such as 'darwin/arm64'.",
          "type": "array",
          "items": {
            "type": "string",
            "minLength": 1
          },
          "uniqueItems": true
        }
      },
      "required": [
        "language"
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "libraries": {
      "description": "Every third-party library, keyed by its package or module name.",
      "type": "object",
      "additionalProperties": {
        "$ref": "#/$defs/library"
      }
    },
    "layout": {
      "description": "Packages, modules or folders of the implementation, keyed by path, with the design objects each one implements.",
      "type": "object",
      "additionalProperties": {
        "type": "object",
        "properties": {
          "description": {
            "$ref": "#/$defs/markdown"
          },
          "implements": {
            "type": "array",
            "items": {
              "$ref": "#/$defs/designRef"
            },
            "uniqueItems": true
          }
        },
        "required": [
          "description"
        ],
        "patternProperties": {
          "^x-": {}
        },
        "additionalProperties": false
      }
    },
    "mappings": {
      "description": "How design objects map onto the stack, keyed by a pointer into the design file such as '#/entities/Loan' or '#/paths/~1loans/post'.",
      "type": "object",
      "propertyNames": {
        "$ref": "#/$defs/designRef"
      },
      "additionalProperties": {
        "$ref": "#/$defs/mapping"
      }
    },
    "bindings": {
      "description": "Framework binding per interface kind of the design file: the router and middleware for HTTP, the CLI library for commands, the client for messaging.",
      "type": "object",
      "propertyNames": {
        "enum": [
          "http",
          "messaging",
          "cli",
          "ui",
          "storage"
        ]
      },
      "additionalProperties": {
        "$ref": "#/$defs/binding"
      }
    },
    "targets": {
      "description": "The document and code targets this implementation produces, keyed by target name as given to specarch document or specarch generate, each with the folder it owns and its settings. Defaults: the sql target writes PostgreSQL; the ui target writes plain JavaScript for the web and SwiftUI for the iPhone.",
      "type": "object",
      "propertyNames": {
        "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
      },
      "additionalProperties": {
        "$ref": "#/$defs/target"
      }
    },
    "tasks": {
      "description": "Build, test, scan and CI commands, keyed by kebab-case name.",
      "type": "object",
      "propertyNames": {
        "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
      },
      "additionalProperties": {
        "$ref": "#/$defs/task"
      }
    },
    "testing": {
      "$ref": "#/$defs/testing"
    },
    "idioms": {
      "description": "SpecArch keyword. The idioms this implementation file deviates from; every other shipped idiom whose stacks and reads match applies by default. Each key names a shipped idiom or one of the project's own, with exclude and why, or override naming a file in the idioms folder beside this file.",
      "type": "object",
      "propertyNames": {
        "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
      },
      "additionalProperties": {
        "$ref": "#/$defs/idiomUse"
      }
    },
    "deployments": {
      "description": "Real deployments, keyed by environment name: the environment of the specification each one installs, its servers, hosts and ports, and the values of the non-secret settings.",
      "type": "object",
      "propertyNames": {
        "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
      },
      "additionalProperties": {
        "$ref": "#/$defs/deployment"
      }
    },
    "decisions": {
      "description": "Implementation decision records, keyed by ID. An ID must not also be used by a decision in the design file.",
      "type": "object",
      "propertyNames": {
        "pattern": "^ADR-[0-9]{3,}$"
      },
      "additionalProperties": {
        "$ref": "#/$defs/decision"
      }
    }
  },
  "required": [
    "specarchImplementation",
    "info",
    "implements",
    "stack"
  ],
  "patternProperties": {
    "^x-": {}
  },
  "additionalProperties": false,
  "$defs": {
    "markdown": {
      "description": "Prose in Markdown.",
      "type": "string"
    },
    "designRef": {
      "description": "A JSON pointer into the specification, naming one of its objects: '#/entities/Loan', '#/commands/validate', '#/paths/~1loans/post'. The validator checks it resolves.",
      "type": "string",
      "pattern": "^#/(entities|enums|permissions|roles|paths|commands|channels|pages|algorithms|decisions)/[^/]+(/.+)?$"
    },
    "satisfies": {
      "description": "SpecArch keyword. The requirements this element satisfies, by ID, as in the specification.",
      "type": "array",
      "items": {
        "type": "string",
        "pattern": "^[A-Z][A-Z0-9]{1,15}-[A-Za-z0-9._]+$"
      },
      "uniqueItems": true
    },
    "why": {
      "description": "SpecArch keyword. The rationale in plain words, as in the specification.",
      "type": "string",
      "minLength": 1
    },
    "citation": {
      "description": "SpecArch keyword. One citation, as in the specification.",
      "type": "object",
      "properties": {
        "source": {
          "description": "The key of a source declared under 'sources' in the specification's specarch.yaml.",
          "type": "string",
          "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
        },
        "clause": {
          "type": "string",
          "minLength": 1
        },
        "says": {
          "type": "string",
          "minLength": 1
        }
      },
      "required": [
        "source",
        "says"
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "citations": {
      "type": "array",
      "items": {
        "$ref": "#/$defs/citation"
      },
      "minItems": 1
    },
    "origin": {
      "description": "SpecArch keyword. How the element is known. stated: a source says it, and cites names the source and where in it. inferred: it was concluded from evidence (code, data, the source's silence), and why says from what. decided: a stakeholder settled it, answering a question or correcting the source, and decidedIn names the decision. An element without origin was written spec-first. What is not known at all is not an element with an origin but an open question under questions.",
      "type": "string",
      "enum": [
        "stated",
        "inferred",
        "decided"
      ]
    },
    "decidedIn": {
      "description": "SpecArch keyword. With origin decided: the decision record that settled this element, such as ADR-021.",
      "type": "string",
      "pattern": "^ADR-[0-9]{3,}$"
    },
    "tool": {
      "type": "object",
      "properties": {
        "name": {
          "type": "string",
          "minLength": 1
        },
        "version": {
          "description": "Exact version or the minimum, as the tool itself writes it.",
          "type": "string",
          "minLength": 1
        },
        "description": {
          "$ref": "#/$defs/markdown"
        }
      },
      "required": [
        "name",
        "version"
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "library": {
      "type": "object",
      "properties": {
        "version": {
          "description": "The exact pinned version.",
          "type": "string",
          "minLength": 1
        },
        "licence": {
          "description": "SPDX licence expression. Must be OSI-approved.",
          "type": "string",
          "pattern": "^[A-Za-z0-9.+-]+( (AND|OR) [A-Za-z0-9.+-]+)*$"
        },
        "purpose": {
          "type": "string",
          "minLength": 1
        },
        "scope": {
          "type": "string",
          "enum": [
            "runtime",
            "test",
            "build",
            "tool"
          ],
          "default": "runtime"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "version",
        "licence",
        "purpose"
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "mapping": {
      "type": "object",
      "properties": {
        "target": {
          "description": "The name on the stack: a type, a table, a function, a handler.",
          "type": "string",
          "minLength": 1
        },
        "ownedBy": {
          "description": "SpecArch keyword. The stakeholder of the specification that owns the element, when it is not this project: another team, a vendor or a shared service. The specification still describes the element, and validate, diff, gaps and the gates still read it, but no generator that builds code or data writes it (the tests target, which checks, keeps it), and an element that refers to it, such as a foreign key or a schema reference, still refers to it.",
          "type": "string",
          "minLength": 1
        },
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "settings": {
          "description": "Generator- or framework-specific settings for this object, such as code-generator extensions.",
          "type": "object"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "target"
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "binding": {
      "type": "object",
      "properties": {
        "framework": {
          "description": "The framework or library, by its name in 'libraries' or 'standard library'.",
          "type": "string",
          "minLength": 1
        },
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "settings": {
          "type": "object"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "framework"
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "target": {
      "type": "object",
      "properties": {
        "output": {
          "description": "The folder this target owns, relative to this implementation file. A document target writes <target>.md there.",
          "type": "string",
          "minLength": 1
        },
        "tool": {
          "description": "The downstream tool that takes the generator's output, if any, such as an OpenAPI code generator.",
          "type": "string"
        },
        "dialect": {
          "description": "The dialect the target writes, which is also a stack of the implementation file for its idioms: for the sql target a database dialect, postgresql by default; for the openapi target dxlib, the document dxlib binds, when the service runs on dxlib. Without it the openapi target writes standard OpenAPI.",
          "type": "string",
          "enum": [
            "postgresql",
            "sqlserver",
            "oracle",
            "mariadb",
            "dxlib"
          ],
          "default": "postgresql"
        },
        "platform": {
          "description": "For the ui target: where the screens run.",
          "type": "string",
          "enum": [
            "web",
            "iphone"
          ]
        },
        "framework": {
          "description": "For the ui target: the component library or UI framework. The default for platform web is plain-javascript: native ES modules in the browser, no package install, no bundler or build step, every third-party library a pinned file committed beside the generated code, and pages and components that talk through one small generated event bus (event names declared once as UPPERCASE constants with their payload shape, subscriptions registered where a component is created, no wildcard or computed names, an optional debug log of every event). For platform iphone the default is swiftui.",
          "type": "string",
          "minLength": 1
        },
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "settings": {
          "type": "object"
        },
        "reads": {
          "description": "The sections of the specification this code target reads. A target that names none reads every section. specarch generate refuses to run a target while a must or should question blocks a section it reads, and specarch gaps says per target what waits.",
          "type": "array",
          "items": {
            "type": "string",
            "enum": [
              "stakeholders",
              "needs",
              "requirements",
              "glossary",
              "assumptions",
              "constraints",
              "enums",
              "entities",
              "permissions",
              "roles",
              "session",
              "paths",
              "commands",
              "channels",
              "dependencies",
              "pages",
              "algorithms",
              "tests",
              "decisions",
              "environments",
              "configuration",
              "release",
              "rollback",
              "migrations",
              "checks",
              "signoff",
              "monitors"
            ]
          },
          "minItems": 1,
          "uniqueItems": true
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "output"
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false,
      "allOf": [
        {
          "if": {
            "properties": {
              "platform": {
                "const": "web"
              }
            },
            "required": [
              "platform"
            ]
          },
          "then": {
            "properties": {
              "framework": {
                "default": "plain-javascript"
              }
            }
          }
        },
        {
          "if": {
            "properties": {
              "platform": {
                "const": "iphone"
              }
            },
            "required": [
              "platform"
            ]
          },
          "then": {
            "properties": {
              "framework": {
                "default": "swiftui"
              }
            }
          }
        }
      ]
    },
    "task": {
      "type": "object",
      "properties": {
        "run": {
          "type": "string",
          "minLength": 1
        },
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "ci": {
          "description": "The task runs in CI on every change.",
          "type": "boolean"
        }
      },
      "required": [
        "run"
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "deployment": {
      "type": "object",
      "properties": {
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "environment": {
          "description": "The environment of the specification this deployment installs, by key. Required when the specification declares environments.",
          "type": "string",
          "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
        },
        "servers": {
          "type": "array",
          "items": {
            "type": "object",
            "properties": {
              "url": {
                "type": "string",
                "minLength": 1
              },
              "description": {
                "$ref": "#/$defs/markdown"
              }
            },
            "required": [
              "url"
            ],
            "patternProperties": {
              "^x-": {}
            },
            "additionalProperties": false
          },
          "minItems": 1
        },
        "configuration": {
          "description": "The value of each non-secret setting of the specification's configuration in this deployment, keyed by setting name. A secret's value is never written here; say in the setting's description where it comes from.",
          "type": "object",
          "propertyNames": {
            "pattern": "^[a-z][A-Za-z0-9]*$"
          },
          "additionalProperties": {
            "type": [
              "string",
              "integer",
              "number",
              "boolean"
            ]
          }
        },
        "monitors": {
          "description": "How this deployment watches each monitor of the specification, keyed by monitor name: the tool that measures, how, and where an alert goes.",
          "type": "object",
          "propertyNames": {
            "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
          },
          "additionalProperties": {
            "type": "object",
            "properties": {
              "tool": {
                "description": "The tool that measures, such as a metrics system or an uptime checker.",
                "type": "string",
                "minLength": 1
              },
              "description": {
                "$ref": "#/$defs/markdown"
              },
              "alert": {
                "description": "Where an alert goes when the objective is missed: a channel or a role, never a person.",
                "type": "string",
                "minLength": 1
              },
              "settings": {
                "description": "Free-form settings of the tool, such as the query.",
                "type": "object"
              }
            },
            "required": [
              "tool"
            ],
            "patternProperties": {
              "^x-": {}
            },
            "additionalProperties": false
          }
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "description"
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "decision": {
      "type": "object",
      "properties": {
        "title": {
          "type": "string",
          "minLength": 1
        },
        "status": {
          "type": "string",
          "enum": [
            "proposed",
            "accepted",
            "deprecated",
            "superseded"
          ]
        },
        "date": {
          "type": "string",
          "format": "date"
        },
        "context": {
          "$ref": "#/$defs/markdown"
        },
        "decision": {
          "$ref": "#/$defs/markdown"
        },
        "consequences": {
          "$ref": "#/$defs/markdown"
        },
        "supersededBy": {
          "type": "string",
          "pattern": "^ADR-[0-9]{3,}$"
        },
        "satisfies": {
          "$ref": "#/$defs/satisfies"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "title",
        "status",
        "date",
        "context",
        "decision",
        "consequences"
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false,
      "if": {
        "properties": {
          "status": {
            "const": "superseded"
          }
        }
      },
      "then": {
        "required": [
          "supersededBy"
        ]
      }
    },
    "testing": {
      "description": "How this implementation is tested: the framework, how to run the tests, and the suites. Design tests are defined in the design file; a suite here only says how they are run on this stack.",
      "type": "object",
      "properties": {
        "framework": {
          "type": "string",
          "minLength": 1
        },
        "run": {
          "description": "The command that runs every suite.",
          "type": "string",
          "minLength": 1
        },
        "platforms": {
          "description": "Operating systems and architectures the tests run on.",
          "type": "array",
          "items": {
            "type": "string",
            "minLength": 1
          },
          "uniqueItems": true
        },
        "fixtures": {
          "$ref": "#/$defs/markdown"
        },
        "mocks": {
          "description": "What is replaced by a stand-in in tests, and why.",
          "$ref": "#/$defs/markdown"
        },
        "performance": {
          "description": "Performance and load targets.",
          "$ref": "#/$defs/markdown"
        },
        "suites": {
          "type": "object",
          "propertyNames": {
            "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
          },
          "additionalProperties": {
            "$ref": "#/$defs/suite"
          },
          "minProperties": 1
        }
      },
      "required": [
        "framework",
        "run",
        "suites"
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "suite": {
      "description": "One test suite. It runs design tests (by name, or every test of a subject) or is implementation-only, never neither.",
      "type": "object",
      "properties": {
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "run": {
          "type": "string",
          "minLength": 1
        },
        "level": {
          "description": "unit: one piece of this implementation's code alone; integration: several pieces, or the code with a real database or service; system: the design's system tests run through this implementation; acceptance: the design's acceptance tests. The levels are those of ISO/IEC/IEEE 29119-1. A suite that runs design tests is system or acceptance; an implementation-only suite is unit or integration.",
          "type": "string",
          "enum": [
            "unit",
            "integration",
            "system",
            "acceptance"
          ]
        },
        "designTests": {
          "description": "Names of design tests this suite runs.",
          "type": "array",
          "items": {
            "type": "string",
            "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
          },
          "minItems": 1,
          "uniqueItems": true
        },
        "designTestsOf": {
          "description": "Subjects whose design tests this suite runs, every one of them, such as {command: validate}.",
          "type": "array",
          "items": {
            "type": "object",
            "properties": {
              "operation": {
                "type": "string"
              },
              "command": {
                "type": "string"
              },
              "page": {
                "type": "string"
              }
            },
            "minProperties": 1,
            "maxProperties": 1,
            "additionalProperties": false
          },
          "minItems": 1
        },
        "implementationOnly": {
          "description": "The suite tests this implementation's own code and runs no design test.",
          "type": "boolean",
          "const": true
        },
        "ci": {
          "description": "The suite runs in CI on every change.",
          "type": "boolean"
        },
        "why": {
          "$ref": "#/$defs/why"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        },
        "origin": {
          "$ref": "#/$defs/origin"
        },
        "decidedIn": {
          "$ref": "#/$defs/decidedIn"
        }
      },
      "required": [
        "description",
        "run",
        "level"
      ],
      "oneOf": [
        {
          "required": [
            "designTests"
          ]
        },
        {
          "required": [
            "designTestsOf"
          ]
        },
        {
          "required": [
            "implementationOnly"
          ]
        }
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false,
      "allOf": [
        {
          "if": {
            "required": [
              "implementationOnly"
            ]
          },
          "then": {
            "properties": {
              "level": {
                "enum": [
                  "unit",
                  "integration"
                ]
              }
            }
          },
          "else": {
            "properties": {
              "level": {
                "enum": [
                  "system",
                  "acceptance"
                ]
              }
            }
          }
        }
      ]
    },
    "idiomUse": {
      "type": "object",
      "properties": {
        "exclude": {
          "description": "true: the idiom does not apply to this file; why says what the project does instead.",
          "const": true
        },
        "override": {
          "description": "The override file, relative to this file, in its idioms folder.",
          "type": "string",
          "pattern": "^idioms/[a-z][a-z0-9]*(-[a-z0-9]+)*\\.specarch-idiom\\.yaml$"
        },
        "why": {
          "$ref": "#/$defs/why"
        }
      },
      "oneOf": [
        {
          "required": [
            "exclude"
          ]
        },
        {
          "required": [
            "override"
          ]
        }
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    }
  }
}
"""#

let recordSchemaJSON = #"""
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-record-0.1.schema.json",
  "title": "SpecArch record, 0.1",
  "description": "A record is a fact about one event in the life of a system: a change request, a defect, a release, an incident in production, a commissioning run or an approval. Records live in records/ beside the specification's folder, one file per record, in a folder per kind; they point into the specification, and the specification never points back at them. A record names people only by role. Every object rejects unknown keys; keys starting with 'x-' are allowed everywhere as extensions.",
  "type": "object",
  "properties": {
    "specarchRecord": {
      "description": "Record format version this file is written against.",
      "type": "string",
      "const": "0.1"
    },
    "kind": {
      "description": "What the record is; it decides the folder it lives in and the keys it holds.",
      "type": "string",
      "enum": [
        "change",
        "defect",
        "release",
        "incident",
        "commissioning",
        "approval"
      ]
    }
  },
  "required": [
    "specarchRecord",
    "kind"
  ],
  "allOf": [
    {
      "if": {
        "properties": {
          "kind": {
            "const": "change"
          }
        },
        "required": [
          "kind"
        ]
      },
      "then": {
        "$ref": "#/$defs/change"
      }
    },
    {
      "if": {
        "properties": {
          "kind": {
            "const": "defect"
          }
        },
        "required": [
          "kind"
        ]
      },
      "then": {
        "$ref": "#/$defs/defect"
      }
    },
    {
      "if": {
        "properties": {
          "kind": {
            "const": "release"
          }
        },
        "required": [
          "kind"
        ]
      },
      "then": {
        "$ref": "#/$defs/release"
      }
    },
    {
      "if": {
        "properties": {
          "kind": {
            "const": "incident"
          }
        },
        "required": [
          "kind"
        ]
      },
      "then": {
        "$ref": "#/$defs/incident"
      }
    },
    {
      "if": {
        "properties": {
          "kind": {
            "const": "commissioning"
          }
        },
        "required": [
          "kind"
        ]
      },
      "then": {
        "$ref": "#/$defs/commissioning"
      }
    },
    {
      "if": {
        "properties": {
          "kind": {
            "const": "approval"
          }
        },
        "required": [
          "kind"
        ]
      },
      "then": {
        "$ref": "#/$defs/approval"
      }
    }
  ],
  "$defs": {
    "role": {
      "description": "A role: the key of a stakeholder of the specification, never a person's name.",
      "type": "string",
      "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
    },
    "name": {
      "description": "The kebab-case name of an element of the specification, such as a test, an environment, a monitor or a check.",
      "type": "string",
      "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
    },
    "date": {
      "description": "A date, quoted, as YYYY-MM-DD.",
      "type": "string",
      "format": "date"
    },
    "version": {
      "description": "A Semantic Versioning 2.0.0 version, such as 1.4.0.",
      "type": "string",
      "pattern": "^[0-9]+\\.[0-9]+\\.[0-9]+(-[0-9A-Za-z.-]+)?$"
    },
    "recordId": {
      "description": "The ID of a change request, defect or incident: an upper-case prefix, a dash and a number or name, such as CR-12, DEF-7 or INC-3. It is the record's file name, or an ID in a tracker declared under the specification's sources.",
      "type": "string",
      "pattern": "^[A-Z][A-Z0-9]{1,15}-[A-Za-z0-9._]+$"
    },
    "reference": {
      "description": "A reference into the specification: a requirement ID such as LIB-5, or a #/ pointer into the merged specification such as #/entities/Loan, #/tests/borrow-limit or #/environments/production.",
      "type": "string",
      "pattern": "^([A-Z][A-Z0-9]{1,15}-[A-Za-z0-9._]+|#/.+)$"
    },
    "references": {
      "description": "References into the specification.",
      "type": "array",
      "items": {
        "$ref": "#/$defs/reference"
      },
      "minItems": 1,
      "uniqueItems": true
    },
    "run": {
      "description": "A commissioning run, by the name of its record: its date and environment, such as 2026-10-07-production.",
      "type": "string",
      "pattern": "^[0-9]{4}-[0-9]{2}-[0-9]{2}-[a-z][a-z0-9]*(-[a-z0-9]+)*$"
    },
    "change": {
      "description": "A change request (ISO/IEC/IEEE 12207:2017, 6.4.13; ISO/IEC/IEEE 14764:2022): a request for the system to be different. Kept in records/changes/, named by its ID.",
      "type": "object",
      "properties": {
        "specarchRecord": {},
        "kind": {},
        "id": {
          "$ref": "#/$defs/recordId"
        },
        "title": {
          "description": "One line saying what is asked for.",
          "type": "string",
          "minLength": 1
        },
        "raisedBy": {
          "$ref": "#/$defs/role"
        },
        "raised": {
          "$ref": "#/$defs/date"
        },
        "phase": {
          "description": "Where it was raised: during development, or in production.",
          "type": "string",
          "enum": [
            "development",
            "production"
          ]
        },
        "type": {
          "description": "The maintenance type of ISO/IEC/IEEE 14764:2022 other than corrective, which is a defect: additive (a new capability), perfective (a better one), adaptive (a new environment) or preventive (a problem headed off).",
          "type": "string",
          "enum": [
            "additive",
            "perfective",
            "adaptive",
            "preventive"
          ]
        },
        "urgency": {
          "description": "The change type of ITIL 4 change enablement, which sets how much assessment it gets before approval: standard (pre-approved and low risk), normal or emergency.",
          "type": "string",
          "enum": [
            "standard",
            "normal",
            "emergency"
          ]
        },
        "reason": {
          "description": "Why it is asked for, in plain words.",
          "type": "string",
          "minLength": 1
        },
        "affects": {
          "description": "What it changes in the specification, written when it is analysed.",
          "type": "object",
          "properties": {
            "adds": {
              "description": "What it adds: requirement IDs or #/ pointers that resolve once it is implemented.",
              "type": "array",
              "items": {
                "$ref": "#/$defs/reference"
              },
              "minItems": 1,
              "uniqueItems": true
            },
            "changes": {
              "description": "What it changes: requirement IDs or #/ pointers that resolve before and after.",
              "type": "array",
              "items": {
                "$ref": "#/$defs/reference"
              },
              "minItems": 1,
              "uniqueItems": true
            },
            "removes": {
              "description": "What it removes: requirement IDs or #/ pointers that no longer resolve once it is implemented; a removed requirement may instead stay with status retired.",
              "type": "array",
              "items": {
                "$ref": "#/$defs/reference"
              },
              "minItems": 1,
              "uniqueItems": true
            }
          },
          "required": [],
          "minProperties": 1,
          "patternProperties": {
            "^x-": {}
          },
          "additionalProperties": false
        },
        "impact": {
          "description": "The version step it needs under Semantic Versioning 2.0.0: major when a client written for the previous version breaks, minor when the public interface only grows, patch when it does not change.",
          "type": "string",
          "enum": [
            "major",
            "minor",
            "patch"
          ]
        },
        "decision": {
          "description": "The decision on the request.",
          "type": "object",
          "properties": {
            "by": {
              "$ref": "#/$defs/role"
            },
            "date": {
              "$ref": "#/$defs/date"
            },
            "outcome": {
              "description": "Approved or rejected.",
              "type": "string",
              "enum": [
                "approved",
                "rejected"
              ]
            },
            "why": {
              "description": "Why it was decided so.",
              "type": "string",
              "minLength": 1
            }
          },
          "required": [
            "by",
            "date",
            "outcome",
            "why"
          ],
          "patternProperties": {
            "^x-": {}
          },
          "additionalProperties": false
        },
        "release": {
          "$ref": "#/$defs/version"
        },
        "status": {
          "description": "Where it stands: proposed, analysed (affects and impact written), approved or rejected, implemented (the specification and the code carry it), released, or withdrawn by the requester.",
          "type": "string",
          "enum": [
            "proposed",
            "analysed",
            "approved",
            "rejected",
            "implemented",
            "released",
            "withdrawn"
          ]
        }
      },
      "required": [
        "specarchRecord",
        "kind",
        "id",
        "title",
        "raisedBy",
        "raised",
        "phase",
        "type",
        "urgency",
        "reason",
        "status"
      ],
      "allOf": [
        {
          "if": {
            "properties": {
              "status": {
                "enum": [
                  "analysed",
                  "approved",
                  "rejected",
                  "implemented",
                  "released"
                ]
              }
            },
            "required": [
              "status"
            ]
          },
          "then": {
            "required": [
              "affects",
              "impact"
            ]
          }
        },
        {
          "if": {
            "properties": {
              "status": {
                "const": "released"
              }
            },
            "required": [
              "status"
            ]
          },
          "then": {
            "required": [
              "release"
            ]
          }
        }
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "defect": {
      "description": "A defect: the system not doing what its specification says, the corrective maintenance of ISO/IEC/IEEE 14764:2022. Kept in records/defects/, named by its ID.",
      "type": "object",
      "properties": {
        "specarchRecord": {},
        "kind": {},
        "id": {
          "$ref": "#/$defs/recordId"
        },
        "title": {
          "description": "One line saying what goes wrong.",
          "type": "string",
          "minLength": 1
        },
        "reportedBy": {
          "$ref": "#/$defs/role"
        },
        "reported": {
          "$ref": "#/$defs/date"
        },
        "phase": {
          "description": "Where it was found: during development, or in production.",
          "type": "string",
          "enum": [
            "development",
            "production"
          ]
        },
        "environment": {
          "$ref": "#/$defs/name"
        },
        "severity": {
          "description": "How badly it stops the system from meeting its requirements.",
          "type": "string",
          "enum": [
            "critical",
            "major",
            "minor",
            "cosmetic"
          ]
        },
        "violates": {
          "description": "The requirement IDs, or #/ pointers to the design elements, it breaks.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/reference"
          },
          "minItems": 1,
          "uniqueItems": true
        },
        "test": {
          "$ref": "#/$defs/name"
        },
        "incidents": {
          "description": "The incidents it caused, by ID.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/recordId"
          },
          "minItems": 1,
          "uniqueItems": true
        },
        "duplicateOf": {
          "$ref": "#/$defs/recordId"
        },
        "change": {
          "$ref": "#/$defs/recordId"
        },
        "release": {
          "$ref": "#/$defs/version"
        },
        "status": {
          "description": "Where it stands: reported; confirmed, not-a-defect or duplicate after triage against the specification; fixed (the fix is merged and the test passes); released.",
          "type": "string",
          "enum": [
            "reported",
            "confirmed",
            "not-a-defect",
            "duplicate",
            "fixed",
            "released"
          ]
        }
      },
      "required": [
        "specarchRecord",
        "kind",
        "id",
        "title",
        "reportedBy",
        "reported",
        "phase",
        "severity",
        "violates",
        "status"
      ],
      "allOf": [
        {
          "if": {
            "properties": {
              "status": {
                "const": "released"
              }
            },
            "required": [
              "status"
            ]
          },
          "then": {
            "required": [
              "release"
            ]
          }
        },
        {
          "if": {
            "properties": {
              "phase": {
                "const": "production"
              }
            },
            "required": [
              "phase"
            ]
          },
          "then": {
            "required": [
              "environment"
            ]
          }
        }
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "release": {
      "description": "A release of the system: one version, what it includes and what accepted it. Kept in records/releases/, named by its version.",
      "type": "object",
      "properties": {
        "specarchRecord": {},
        "kind": {},
        "version": {
          "$ref": "#/$defs/version"
        },
        "status": {
          "description": "Planned while it is being built, released once it ships, withdrawn if it is pulled.",
          "type": "string",
          "enum": [
            "planned",
            "released",
            "withdrawn"
          ]
        },
        "date": {
          "$ref": "#/$defs/date"
        },
        "includes": {
          "description": "The change requests and defects it carries, by ID.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/recordId"
          },
          "minItems": 1,
          "uniqueItems": true
        },
        "specificationVersion": {
          "$ref": "#/$defs/version"
        },
        "implementations": {
          "description": "The version of each implementation file at release, keyed by stack.",
          "type": "object",
          "propertyNames": {
            "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
          },
          "additionalProperties": {
            "$ref": "#/$defs/version"
          },
          "minProperties": 1
        },
        "commissioning": {
          "description": "The commissioning runs that accepted it, by record name.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/run"
          },
          "minItems": 1,
          "uniqueItems": true
        },
        "commit": {
          "description": "The commit the release was built from.",
          "type": "string",
          "minLength": 1
        }
      },
      "required": [
        "specarchRecord",
        "kind",
        "version",
        "status"
      ],
      "allOf": [
        {
          "if": {
            "properties": {
              "status": {
                "const": "released"
              }
            },
            "required": [
              "status"
            ]
          },
          "then": {
            "required": [
              "date"
            ]
          }
        }
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "incident": {
      "description": "An incident in production (ISO/IEC/IEEE 12207:2017, 6.4.12): the live system misbehaving. Kept in records/incidents/, named by its ID.",
      "type": "object",
      "properties": {
        "specarchRecord": {},
        "kind": {},
        "id": {
          "$ref": "#/$defs/recordId"
        },
        "detected": {
          "$ref": "#/$defs/date"
        },
        "environment": {
          "$ref": "#/$defs/name"
        },
        "monitor": {
          "$ref": "#/$defs/name"
        },
        "reportedBy": {
          "$ref": "#/$defs/role"
        },
        "summary": {
          "description": "What happened, in plain words.",
          "type": "string",
          "minLength": 1
        },
        "impact": {
          "description": "What it did to the people who use the system.",
          "type": "string",
          "minLength": 1
        },
        "status": {
          "description": "Open while it lasts, resolved once the system is back to what the specification says.",
          "type": "string",
          "enum": [
            "open",
            "resolved"
          ]
        },
        "defects": {
          "description": "The defects that follow from it, by ID.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/recordId"
          },
          "minItems": 1,
          "uniqueItems": true
        },
        "changes": {
          "description": "The change requests that follow from it, by ID.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/recordId"
          },
          "minItems": 1,
          "uniqueItems": true
        },
        "noChange": {
          "description": "For a resolved incident that leads to no defect and no change, why.",
          "type": "string",
          "minLength": 1
        },
        "rollbackUsed": {
          "description": "Whether the deployment stage's rollback was used.",
          "type": "boolean"
        }
      },
      "required": [
        "specarchRecord",
        "kind",
        "id",
        "detected",
        "environment",
        "summary",
        "impact",
        "status"
      ],
      "if": {
        "not": {
          "required": [
            "monitor"
          ]
        }
      },
      "then": {
        "required": [
          "reportedBy"
        ]
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "commissioning": {
      "description": "One commissioning run: the checks of the commissioning stage run on the installed system. Kept in records/commissioning/, named by its date and environment.",
      "type": "object",
      "properties": {
        "specarchRecord": {},
        "kind": {},
        "environment": {
          "$ref": "#/$defs/name"
        },
        "date": {
          "$ref": "#/$defs/date"
        },
        "version": {
          "$ref": "#/$defs/version"
        },
        "build": {
          "description": "The build that was installed, such as an image tag or a commit.",
          "type": "string",
          "minLength": 1
        },
        "operator": {
          "$ref": "#/$defs/role"
        },
        "results": {
          "description": "The result of each check, keyed by check name.",
          "type": "object",
          "propertyNames": {
            "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
          },
          "minProperties": 1,
          "additionalProperties": {
            "description": "The result of one check.",
            "type": "object",
            "properties": {
              "result": {
                "description": "Whether the check passed.",
                "type": "string",
                "enum": [
                  "pass",
                  "fail",
                  "skipped"
                ]
              },
              "note": {
                "description": "What was seen, or why it was skipped.",
                "type": "string",
                "minLength": 1
              }
            },
            "required": [
              "result"
            ],
            "patternProperties": {
              "^x-": {}
            },
            "additionalProperties": false
          }
        },
        "signoff": {
          "description": "Who signed the run off, and when.",
          "type": "object",
          "properties": {
            "by": {
              "$ref": "#/$defs/role"
            },
            "date": {
              "$ref": "#/$defs/date"
            }
          },
          "required": [
            "by",
            "date"
          ],
          "patternProperties": {
            "^x-": {}
          },
          "additionalProperties": false
        }
      },
      "required": [
        "specarchRecord",
        "kind",
        "environment",
        "date",
        "version",
        "operator",
        "results"
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "approval": {
      "description": "An approval for code generation, written by specarch approve. Kept in records/approvals/, named by its version.",
      "type": "object",
      "properties": {
        "specarchRecord": {},
        "kind": {},
        "version": {
          "$ref": "#/$defs/version"
        },
        "approvedBy": {
          "$ref": "#/$defs/role"
        },
        "date": {
          "$ref": "#/$defs/date"
        },
        "documents": {
          "description": "The documents the stakeholder read, by target name.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/name"
          },
          "minItems": 1,
          "uniqueItems": true
        },
        "digest": {
          "description": "The SHA-256 of every .yaml file of the specification when it was approved.",
          "type": "string",
          "pattern": "^sha256:[0-9a-f]{64}$"
        }
      },
      "required": [
        "specarchRecord",
        "kind",
        "version",
        "approvedBy",
        "date",
        "documents",
        "digest"
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    }
  }
}
"""#

let idiomSchemaJSON = #"""
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-idiom-0.1.schema.json",
  "title": "SpecArch idiom 0.1",
  "description": "An idiom: a named, versioned statement of how one recurring implementation concern is done, with a contract that holds on every stack, renderings per stack, and the tests it implies. SpecArch ships a set under idioms/; a project overrides one, or adds its own, in implementation/<stack>/idioms/. docs/idioms.md is the design.",
  "type": "object",
  "properties": {
    "specarchIdiom": {
      "description": "The meta-model version of the idiom format.",
      "const": "0.1"
    },
    "name": {
      "type": "string",
      "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$",
      "description": "The idiom's name, kebab-case: the key an implementation file and an override use. An override carries the name of the idiom it overrides."
    },
    "version": {
      "description": "The idiom's own version, semantic. It moves when a contract statement, a part or a test changes.",
      "type": "string",
      "pattern": "^[0-9]+\\.[0-9]+\\.[0-9]+(-[0-9A-Za-z.-]+)?$"
    },
    "concern": {
      "description": "The concern the idiom settles, from a closed list.",
      "type": "string",
      "enum": [
        "type-rendering",
        "request-validation",
        "error-response",
        "list-operations",
        "authorization",
        "pii-logging",
        "audit-fields",
        "soft-delete",
        "identifiers",
        "transactions",
        "retries",
        "idempotency",
        "configuration",
        "secrets",
        "background-jobs",
        "migrations",
        "encryption",
        "health",
        "rate-limit",
        "other"
      ]
    },
    "stacks": {
      "description": "The stacks the idiom renders: a language such as go, swift or dart, a SQL dialect such as postgresql, sqlserver, oracle or mariadb, or any for one that is stack-neutral.",
      "type": "array",
      "items": {
        "type": "string",
        "pattern": "^(any|[a-z][a-z0-9]*(-[a-z0-9]+)*)$"
      },
      "minItems": 1,
      "uniqueItems": true
    },
    "description": {
      "$ref": "#/$defs/markdown"
    },
    "reads": {
      "description": "The design sections or keywords the idiom applies to. It applies only to a specification that uses one of them.",
      "type": "array",
      "items": {
        "type": "string",
        "minLength": 1
      },
      "minItems": 1,
      "uniqueItems": true
    },
    "why": {
      "$ref": "#/$defs/markdown"
    },
    "sources": {
      "description": "The standards and documents the idiom cites, in the shape of a specification's sources.",
      "type": "object",
      "propertyNames": {
        "type": "string",
        "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
      },
      "additionalProperties": {
        "$ref": "#/$defs/source"
      }
    },
    "cites": {
      "$ref": "#/$defs/citations"
    },
    "contract": {
      "description": "Statements that hold on every stack, keyed by id.",
      "type": "object",
      "propertyNames": {
        "type": "string",
        "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
      },
      "additionalProperties": {
        "$ref": "#/$defs/statement"
      }
    },
    "parts": {
      "description": "The renderings, keyed by part name.",
      "type": "object",
      "propertyNames": {
        "type": "string",
        "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
      },
      "additionalProperties": {
        "$ref": "#/$defs/part"
      }
    },
    "tests": {
      "description": "The cases the idiom implies for every element it applies to.",
      "type": "array",
      "items": {
        "$ref": "#/$defs/case"
      }
    },
    "overrides": {
      "$ref": "#/$defs/overrides"
    }
  },
  "required": [
    "specarchIdiom",
    "name",
    "version",
    "description"
  ],
  "patternProperties": {
    "^x-": {}
  },
  "additionalProperties": false,
  "$defs": {
    "markdown": {
      "description": "Prose in Markdown.",
      "type": "string",
      "minLength": 1
    },
    "source": {
      "description": "One source an idiom cites.",
      "type": "object",
      "properties": {
        "kind": {
          "type": "string",
          "enum": [
            "standard",
            "regulation",
            "document",
            "system",
            "code"
          ]
        },
        "title": {
          "type": "string",
          "minLength": 1
        },
        "edition": {
          "type": "string",
          "minLength": 1
        },
        "author": {
          "type": "string",
          "minLength": 1
        },
        "url": {
          "type": "string",
          "minLength": 1
        }
      },
      "required": [
        "kind",
        "title"
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "citation": {
      "description": "One citation: which source, where in it, and what it says here.",
      "type": "object",
      "properties": {
        "source": {
          "type": "string",
          "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$",
          "description": "The key of a source under the idiom's sources."
        },
        "clause": {
          "type": "string",
          "minLength": 1
        },
        "says": {
          "type": "string",
          "minLength": 1
        }
      },
      "required": [
        "source",
        "says"
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "citations": {
      "type": "array",
      "items": {
        "$ref": "#/$defs/citation"
      },
      "minItems": 1
    },
    "statement": {
      "description": "One contract statement.",
      "type": "object",
      "properties": {
        "statement": {
          "type": "string",
          "minLength": 1
        },
        "check": {
          "description": "What checks it: schema (the validator, against the design and the implementation file), name (a derivation rule for names), document (the generated document or code must contain it), test (a derived test case), or guidance (no check; advice for whoever writes the code).",
          "type": "string",
          "enum": [
            "schema",
            "name",
            "document",
            "test",
            "guidance"
          ]
        },
        "why": {
          "$ref": "#/$defs/markdown"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        }
      },
      "required": [
        "statement",
        "check"
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "part": {
      "description": "One part of the idiom, rendered per stack.",
      "type": "object",
      "properties": {
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "why": {
          "$ref": "#/$defs/markdown"
        },
        "stack": {
          "description": "The rendering per stack; any for the stack-neutral one.",
          "type": "object",
          "propertyNames": {
            "type": "string",
            "pattern": "^(any|[a-z][a-z0-9]*(-[a-z0-9]+)*)$"
          },
          "additionalProperties": {
            "$ref": "#/$defs/rendering"
          },
          "minProperties": 1
        }
      },
      "required": [
        "description",
        "stack"
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "rendering": {
      "description": "How one stack renders a part.",
      "type": "object",
      "properties": {
        "libraries": {
          "description": "The libraries the rendering uses, as an implementation file's libraries: version and licence.",
          "type": "object",
          "additionalProperties": {
            "type": "object",
            "properties": {
              "version": {
                "type": "string",
                "minLength": 1
              },
              "licence": {
                "type": "string",
                "minLength": 1
              },
              "purpose": {
                "type": "string",
                "minLength": 1
              }
            },
            "required": [
              "version",
              "licence"
            ],
            "patternProperties": {
              "^x-": {}
            },
            "additionalProperties": false
          }
        },
        "settings": {
          "description": "Settings of the rendering a project may change in an override.",
          "type": "object"
        },
        "names": {
          "description": "Derived names, by the role they play.",
          "type": "object",
          "additionalProperties": {
            "type": "string",
            "minLength": 1
          }
        },
        "rows": {
          "description": "For a type rendering: ordered rows, the first that matches a field renders it.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/row"
          },
          "minItems": 1
        },
        "code": {
          "$ref": "#/$defs/markdown"
        },
        "why": {
          "$ref": "#/$defs/markdown"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        }
      },
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "row": {
      "description": "One row of a type rendering.",
      "type": "object",
      "properties": {
        "type": {
          "description": "The field's JSON type, with null dropped from a type list. A $ref to an entity is an object.",
          "type": "string",
          "enum": [
            "string",
            "integer",
            "number",
            "boolean",
            "array",
            "object"
          ]
        },
        "format": {
          "description": "The field's format; a row without one matches any format.",
          "type": "string",
          "minLength": 1
        },
        "enum": {
          "description": "true: the row matches only an enum field, a $ref into #/enums/ or an enum list.",
          "const": true
        },
        "maxLengthAtMost": {
          "description": "The row matches only a field whose maxLength is at most this.",
          "type": "integer",
          "minimum": 1
        },
        "precisionAtMost": {
          "description": "The row matches only a decimal whose precision is at most this.",
          "type": "integer",
          "minimum": 1
        },
        "itemsType": {
          "description": "For an array: the items' JSON type.",
          "type": "string",
          "enum": [
            "string",
            "integer",
            "number",
            "boolean",
            "array",
            "object"
          ]
        },
        "itemsFormat": {
          "description": "For an array: the items' format.",
          "type": "string",
          "minLength": 1
        },
        "render": {
          "description": "What the field becomes: a type, with words in braces a generator fills from the field: {maxLength}, {precision}, {scale}, {longestValue} (the length of an enum's longest value), {items} (the rendering of the items' row). For a SQL dialect, the column type only; a constraint goes under check.",
          "type": "string",
          "minLength": 1
        },
        "check": {
          "description": "A check constraint the column needs beside its type, with words in braces a generator fills: {column}, the column's name; {values}, an enum's values, quoted and separated by commas.",
          "type": "string",
          "minLength": 1
        },
        "why": {
          "$ref": "#/$defs/markdown"
        },
        "cites": {
          "$ref": "#/$defs/citations"
        }
      },
      "required": [
        "type",
        "render"
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "case": {
      "description": "One test case the idiom implies.",
      "type": "object",
      "properties": {
        "case": {
          "type": "string",
          "minLength": 1
        },
        "scenario": {
          "type": "string",
          "enum": [
            "golden",
            "red"
          ]
        },
        "given": {
          "type": "string",
          "minLength": 1
        },
        "when": {
          "type": "string",
          "minLength": 1
        },
        "then": {
          "type": "string",
          "minLength": 1
        }
      },
      "required": [
        "case",
        "scenario"
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    },
    "overrides": {
      "description": "In a project's file: the shipped idiom it overrides, and what it replaces.",
      "type": "object",
      "properties": {
        "idiom": {
          "type": "string",
          "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$",
          "description": "The shipped idiom this file overrides."
        },
        "version": {
          "description": "The shipped version this file was copied from.",
          "type": "string",
          "pattern": "^[0-9]+\\.[0-9]+\\.[0-9]+(-[0-9A-Za-z.-]+)?$"
        },
        "parts": {
          "description": "The parts this file replaces; the others come from the shipped idiom.",
          "type": "array",
          "items": {
            "type": "string",
            "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
          },
          "minItems": 1,
          "uniqueItems": true
        },
        "whole": {
          "description": "true: this file replaces every part.",
          "const": true
        },
        "staysBehind": {
          "description": "true: the project stays on this version on purpose, which why must say; idiom_version_behind is then silent.",
          "const": true
        }
      },
      "required": [
        "idiom",
        "version"
      ],
      "patternProperties": {
        "^x-": {}
      },
      "additionalProperties": false
    }
  }
}
"""#

/// The shipped idioms, by their path under idioms/, as idioms/embed.go embeds them.
let shippedIdiomFiles: [(path: String, text: String)] = [
    ("idioms/audit-fields/audit-fields.specarch-idiom.yaml", #"""
# yaml-language-server: $schema=https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-idiom-0.1.schema.json
specarchIdiom: "0.1"
name: audit-fields
version: 1.1.0
concern: audit-fields
stacks: [go, any]
reads: [audited]
description: The columns an audited entity carries and who sets them.
why: Who changed a record and when is only worth having if no caller can set it.
contract:
  columns:
    statement: An audited entity's table carries the creation and last-change time and user columns.
    check: document
  set-by-system:
    statement: The library sets the columns, and a caller's values for them are overwritten.
    check: guidance
parts:
  columns:
    description: The column names, by the design's field names.
    stack:
      any:
        names: { createdAt: created_at, createdBy: created_by_user_id, createdByName: created_by_user_nameid, lastModifiedAt: last_modified_at, lastModifiedBy: last_modified_by_user_id, lastModifiedByName: last_modified_by_user_nameid }
      go:
        code: |
          Set in the table layer on every insert and update from the
          session's user, as dxlib's tables/tables_table.go does; the user's
          name id is kept beside the id, so a row still says who changed it
          after the user is gone.
"""#),
    ("idioms/authorization/authorization-check.specarch-idiom.yaml", #"""
# yaml-language-server: $schema=https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-idiom-0.1.schema.json
specarchIdiom: "0.1"
name: authorization-check
version: 1.0.0
concern: authorization
stacks: [go]
reads: [permissions]
description: Where the permission an operation names is checked, and what a caller without it is told.
why: Access is fail-closed in the design; the code has to be fail-closed in the same place every time.
contract:
  denied-case:
    statement: A caller without the operation's permission is refused, and that case is tested.
    check: test
  order:
    statement: The check runs after authentication and before the handler; an unauthenticated caller gets 401, an authenticated one without the permission 403.
    check: guidance
  fail-closed:
    statement: An operation whose permission is not known to the check is refused, never let through.
    check: guidance
parts:
  middleware:
    description: Where the check sits.
    stack:
      go:
        code: |
          A middleware reads the caller's session, then the permission the
          generated document names in x-specarch-permission, and asks the
          role store whether a role of the caller grants it. dxlib does this
          in the session middleware of dxlib_module's self module, against
          an endpoint's Privileges.
"""#),
    ("idioms/background-jobs/background-jobs.specarch-idiom.yaml", #"""
# yaml-language-server: $schema=https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-idiom-0.1.schema.json
specarchIdiom: "0.1"
name: background-jobs
version: 1.0.0
concern: background-jobs
stacks: [go]
reads: [jobs]
description: How a job runs, stops and retries.
why: A job runs when no one is watching, so it must be safe to run twice and must say when it gives up.
contract:
  cases:
    statement: A job's derived cases, runs twice and its dependencies failing, are tested.
    check: test
  lifecycle:
    statement: A job runs once or repeats with a delay, stops on shutdown, retries a failed item a bounded number of times, and then sets it aside or drops it as the design says.
    check: guidance
parts:
  runner:
    description: The runner.
    stack:
      go:
        names: { once: once, repeat: always, delay: after_delay_sec }
        code: |
          A task with once or always and a delay, as dxlib's task package
          has; a queue is drained in a loop that marks each item done,
          retried or dead, as the notification module does.
"""#),
    ("idioms/configuration/configuration-and-secrets.specarch-idiom.yaml", #"""
# yaml-language-server: $schema=https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-idiom-0.1.schema.json
specarchIdiom: "0.1"
name: configuration-and-secrets
version: 1.0.0
concern: configuration
stacks: [go]
reads: [configuration]
description: How settings and secrets reach a process.
why: A secret in a file is a secret in every copy of the file.
contract:
  no-secret-value:
    statement: No secret's value is written in any file of the specification or the implementation.
    check: schema
  secret-from-vault:
    statement: A secret is read from a vault into locked memory and resolved only where it is used.
    check: guidance
parts:
  settings:
    description: Where settings and secrets come from.
    stack:
      go:
        code: |
          Settings come from a file and the environment, the environment
          winning; a setting marked secret is read from a vault into locked
          memory and handed out only where it is used, as dxlib's
          configuration (with SensitiveDataKey), secure_memory and vault
          packages do.
"""#),
    ("idioms/encryption/encrypted-column.specarch-idiom.yaml", #"""
# yaml-language-server: $schema=https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-idiom-0.1.schema.json
specarchIdiom: "0.1"
name: encrypted-column
version: 1.1.0
concern: encryption
stacks: [go, any, postgresql, sqlserver, oracle, mariadb]
reads: [atRest]
description: How a field encrypted at rest is stored and still found.
why: Encryption in the engine keeps the plain value out of backups and dumps; a hash beside it keeps the field usable as a key.
contract:
  rendered-or-refused:
    statement: A field encrypted at rest renders through this idiom on every SQL dialect of the implementation, or generation fails.
    check: document
  key-in-memory:
    statement: The encryption runs in the engine with a session key from locked memory, and a salted hash is kept beside a field looked up by hash.
    check: guidance
parts:
  column:
    description: The encrypted column and its hash.
    stack:
      any:
        names: { hashSuffix: _hash }
        why: The hash is a hex SHA-256 of the salted value, 64 characters, so it can be a key and unique where the ciphertext cannot.
      postgresql:
        names: { ciphertext: BYTEA, hash: CHAR(64) }
      sqlserver:
        names: { ciphertext: VARBINARY(MAX), hash: CHAR(64) }
      oracle:
        names: { ciphertext: BLOB, hash: CHAR(64) }
      mariadb:
        names: { ciphertext: LONGBLOB, hash: CHAR(64) }
      go:
        code: |
          The column holds the ciphertext from the engine's own function
          with the session key; a field with lookup hash gets a companion
          column, its name and _hash, holding a salted hash used for
          equality and uniqueness. dxlib's EncryptionColumnDef with
          HashFieldName, and the per-dialect expressions of
          databases/db/encryption_expression.go, do this.
"""#),
    ("idioms/error-response/error-response.specarch-idiom.yaml", #"""
# yaml-language-server: $schema=https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-idiom-0.1.schema.json
specarchIdiom: "0.1"
name: error-response
version: 1.1.0
concern: error-response
stacks: [go, any, dxlib]
reads: [errors]
description: How a refusal is answered, from the design's catalogue of problem types.
why: A client branches on a problem's type, never on its message, so the type and its status are fixed by the design and every refusal has one shape.
sources:
  rfc-9457:
    kind: standard
    title: RFC 9457, Problem Details for HTTP APIs
    edition: "2023"
    author: IETF
    url: https://www.rfc-editor.org/rfc/rfc9457
contract:
  problem-document:
    statement: Every 4xx and 5xx response is an application/problem+json document of a type from the catalogue, with its status.
    check: document
    cites:
      - { source: rfc-9457, clause: "3", says: "A problem details object carries type, title, status, detail and instance." }
  every-refusal-typed:
    statement: Once the catalogue exists, every 4xx and 5xx response of the design names its type.
    check: schema
  validation-lists-fields:
    statement: A validation problem lists each failing field with its reason.
    check: guidance
parts:
  shape:
    description: The body of a refusal.
    stack:
      any:
        code: |
          { "type": "<the type's URI, or about:blank>", "title": "<the type's title>",
            "status": 409, "detail": "<this occurrence>", "instance": "<this request>" }
      dxlib:
        names: { status: status, statusCode: status_code, reason: reason, reasonMessage: reason_message }
        code: |
          { "status": "<the status text>", "status_code": 409, "reason": "<the problem type's name>",
            "reason_message": "<this occurrence>" }
        why: dxlib answers every refusal in this shape today; the problem type's name goes in reason until the runtime answers with a problem document.
      go:
        code: |
          One function writes every problem document from the catalogue's
          name, setting Content-Type to application/problem+json. A library
          that answers in its own shape ({status, status_code, reason,
          reason_message} in dxlib) has its shape rendered from the problem
          until the runtime answers with a problem document.
"""#),
    ("idioms/health/health-endpoint.specarch-idiom.yaml", #"""
# yaml-language-server: $schema=https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-idiom-0.1.schema.json
specarchIdiom: "0.1"
name: health-endpoint
version: 1.0.0
concern: health
stacks: [go]
reads: [paths]
description: How a monitor asks a service whether it is up.
why: A monitor needs one cheap request that touches nothing and says which build answered.
contract:
  ping:
    statement: A public operation answers the service's name and version.
    check: guidance
parts:
  ping:
    description: The operation.
    stack:
      go:
        code: GET /ping, public, answering the name and version, as dxlib_module's oam module does.
"""#),
    ("idioms/identifiers/identifiers.specarch-idiom.yaml", #"""
# yaml-language-server: $schema=https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-idiom-0.1.schema.json
specarchIdiom: "0.1"
name: identifiers
version: 1.1.0
concern: identifiers
stacks: [go, any]
reads: [entities]
description: The keys a record carries and which of them leave the service.
why: An integer key is fast and must never be guessable from outside; an opaque id is safe to show and slow to index; a record often needs both.
contract:
  columns:
    statement: A table carries the internal key and, when the design exposes one, the public id, under the idiom's column names.
    check: document
  internal-key-stays-in:
    statement: The internal integer key never leaves the service.
    check: guidance
  public-id:
    statement: A public id is opaque, at most 255 characters, and cannot be enumerated.
    check: guidance
parts:
  columns:
    description: The key columns.
    stack:
      any:
        names: { internalKey: id, publicId: uid, nameId: nameid, versionTag: utag }
      go:
        code: |
          id is a 64-bit generated key; uid is the hexadecimal microsecond
          time followed by a UUID, generated in the application or by the
          engine, so it sorts roughly by creation and cannot be guessed;
          nameid is an optional unique human-readable id; utag an optional
          version tag for optimistic writes.
"""#),
    ("idioms/list-operations/paginated-list.specarch-idiom.yaml", #"""
# yaml-language-server: $schema=https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-idiom-0.1.schema.json
specarchIdiom: "0.1"
name: paginated-list
version: 1.1.0
concern: list-operations
stacks: [any, dxlib]
reads: [listOf]
description: |
  How an operation that lists an entity takes its search, filter, sort and
  page, and what it answers. The design says which fields may be searched,
  filtered and sorted by and how large a page may be (listOf); this idiom
  gives the names on the wire and the answer's envelope, so every list of
  every service has one shape and one client component reads them all.
why: |
  Every list answers the same four questions: which page, how large, in
  what order, matching what. One shape means one client component and one
  set of tests.

contract:
  page:
    statement: The operation takes a page number from 1 and a page size, and answers the page's records with the total count of records and of pages.
    check: document
  page-size-limit:
    statement: A page size above the maximum of listOf is refused, never cut down silently.
    check: test
  sortable:
    statement: A sort by a field not in sortable is refused, never ignored.
    check: test
  filterable:
    statement: A filter on a field not in filterable is refused, never ignored.
    check: test
  search:
    statement: Free-text search matches any of the searchable fields, case-insensitively.
    check: guidance
  safe-totals:
    statement: The totals are integers a JSON reader holds exactly, at most 2^53 - 1.
    check: document

parts:
  parameters:
    description: The names of the request's parts. A GET takes them as query parameters; any other method takes them as properties of its request body.
    stack:
      any:
        names: { search: search, filter: filter, sort: sort, page: page, pageSize: pageSize }
        code: |
          search is free text. filter is an object whose properties are the
          filterable fields, each matched by equality; in a query it is
          written filter[status]=open (OpenAPI style deepObject). sort is a
          sortable field, ascending, or the field after a minus sign,
          descending. page counts from 1. pageSize has the default and the
          maximum of listOf.
        why: Plain words a client developer reads without a glossary; the deepObject style is the one OpenAPI defines for an object in a query.
      dxlib:
        names: { search: search_text, filter: filter_key_values, sort: order_by, sortField: field_name, sortDirection: direction, page: page_index, pageSize: row_per_page, includeDeleted: is_include_deleted }
        code: |
          Every list is a POST with a JSON body. search_text is free text;
          filter_key_values an object of the filterable fields; order_by a
          list of {field_name, direction}, direction asc or desc; page_index
          counts from 0; row_per_page is the page size; is_include_deleted
          asks for softly deleted records too. These are the names dxlib's
          tables package reads in RequestSearchPagingList.
  envelope:
    description: The answer's shape.
    stack:
      any:
        names: { items: items, totalItems: totalItems, totalPages: totalPages }
        code: |
          { "items": [ ... ], "totalItems": 0, "totalPages": 0 }
        why: The records under one name, so the envelope can grow without breaking a client, and both totals, so a client can draw the pages without counting.
      dxlib:
        names: { items: list.rows, totalItems: list.total_rows, totalPages: list.total_page }
        code: |
          { "list": { "rows": [ ... ], "total_rows": 0, "total_page": 0 } }

tests:
  - { case: page beyond last, scenario: golden, then: an empty page with the true totals }
  - { case: page size above maximum, scenario: red, then: it is refused }
  - { case: sort by a field not sortable, scenario: red, then: it is refused }
  - { case: filter by a field not filterable, scenario: red, then: it is refused }
"""#),
    ("idioms/migrations/migrations.specarch-idiom.yaml", #"""
# yaml-language-server: $schema=https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-idiom-0.1.schema.json
specarchIdiom: "0.1"
name: migrations
version: 1.0.0
concern: migrations
stacks: [go]
reads: [entities]
description: How a schema change reaches a database.
why: A migration that edits an older one changes history that other databases already ran.
contract:
  new-files-only:
    statement: A change is a new migration file; a destructive step is in a file of its own.
    check: document
  model-is-source:
    statement: The model is the source and the DDL is derived from it.
    check: guidance
parts:
  files:
    description: The migration files.
    stack:
      go:
        code: |
          The DDL is derived from the model (models.ModelDBTable and
          CreateDDL in dxlib), and a snapshot of the model beside the
          generated SQL shows what changed since the last migration.
"""#),
    ("idioms/pii-logging/pii-in-logs.specarch-idiom.yaml", #"""
# yaml-language-server: $schema=https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-idiom-0.1.schema.json
specarchIdiom: "0.1"
name: pii-in-logs
version: 1.0.0
concern: pii-logging
stacks: [go]
reads: [sensitivity]
description: How a personal or secret value is kept out of logs and out of the answers it does not belong in.
why: A log is read by more people than the data it records, and is kept longer.
contract:
  credential-not-answered:
    statement: A credential field never appears in a response unless it is writeOnly.
    check: schema
  masked-in-logs:
    statement: Every personal field is masked by its rule in every log line, request dump and response dump; credential headers are masked whole.
    check: guidance
parts:
  masks:
    description: The mask rule per kind of value.
    stack:
      go:
        names: { partial: MaskPartial, email: MaskEmail, initials: MaskInitials, location: MaskLocation, dump: MaskForLog }
        code: |
          partial keeps a few characters at the front and the back; an email
          keeps two characters of each part; a name keeps the first letter
          of each word; a location is rounded to two decimals. Every dump of
          a request or a response goes through MaskForLog, which applies
          the rule of each field's sensitivity. These are the rules of
          dxlib's utils/utils.go.
"""#),
    ("idioms/request-validation/request-validation.specarch-idiom.yaml", #"""
# yaml-language-server: $schema=https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-idiom-0.1.schema.json
specarchIdiom: "0.1"
name: request-validation
version: 1.1.0
concern: request-validation
stacks: [go, any, dxlib]
reads: [paths]
description: |
  How a request is checked before its handler runs: every bound, format,
  enum and required field of the design, with one refusal per failing
  field, and how a field that may be left out differs from one that may be
  null.
why: |
  A handler that receives only valid input has one job. Checking in one
  place, from the design, means no handler forgets a bound and every
  refusal looks the same.
contract:
  constraints-in-document:
    statement: Every bound, format, enum and required field of the design is in the emitted interface document, so the generated server checks it.
    check: document
  red-cases:
    statement: Every boundary of a field has its derived red case, just outside the limit, and its golden case on it.
    check: test
  refusal-names-field:
    statement: A refusal names the path of each failing field, such as customer.email.
    check: guidance
  absent-and-null:
    statement: A field that may be left out (required false) and a field whose value may be null (a type list with null) are told apart, and a handler never reads one as the other, except on a stack whose rendering reads both as one case, not given; there the handler does what the specification says for a value not given, its default or the branch it names.
    check: guidance
parts:
  validation:
    description: Where and in what order the checks run.
    stack:
      go:
        code: |
          Before the handler runs, in this order: a required field is
          present; its value has the declared type (a JSON number for an
          integer arrives as a float without a fraction or as a string of
          digits, since a query string carries only strings); it is inside
          its bounds, matches its format and pattern, and is one of its enum
          values; the children of an object are checked the same way. The
          generated strict server of oapi-codegen does the shape, and a
          middleware the rest, from the OpenAPI document.
        why: dxlib checks the same list in api/api_endpoint_request.go before a handler is called, and its services have relied on it for years.
  absence-and-null:
    description: How a field that may be left out and a field that may be null are held.
    stack:
      go:
        code: |
          Left out only (required false, not nullable): the plain Go type,
          with a presence flag or an accessor the request layer sets, such
          as Has(name); the zero value never stands for "not given". Null
          only (required, a type list with null): a pointer. Both: a pointer
          and the presence flag.
        why: In a REST request, absence and null are two different things a client can send; a pointer alone cannot say which one it was.
      dxlib:
        code: |
          One case, not given. dxlib reads a parameter sent as null and one
          left out alike: its value stays nil and the getter answers
          isExist false, for a nullable-X type and for any parameter that
          is not required. A field that may be null takes dxlib's nullable
          type where its base has one (nullable-string, nullable-int32,
          nullable-int64) and is never required, since a required one would
          refuse the null. The handler reads it with the base getter into
          the plain Go type and a Has flag, and when Has is false it does
          what the specification says for a value not given: takes the
          field's default, or follows the branch the operation names.
        why: In dxlib, nullable means the parameter may be not given, whether left out or sent as null; a handler that tried to tell the two apart would find nothing to tell them by, and the specification's default or branch is what gives "not given" its meaning.
"""#),
    ("idioms/soft-delete/soft-delete.specarch-idiom.yaml", #"""
# yaml-language-server: $schema=https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-idiom-0.1.schema.json
specarchIdiom: "0.1"
name: soft-delete
version: 1.1.0
concern: soft-delete
stacks: [go, any]
reads: [deletion]
description: How a softly deleted record is kept and hidden.
why: A delete that can be undone keeps the record for audit and recovery, and must still look like a delete to every caller.
contract:
  column:
    statement: The table carries a boolean deleted column, false by default.
    check: document
  hidden:
    statement: A deleted record is not listed and reads as not found.
    check: test
  hard-delete-separate:
    statement: A hard delete is a separate operation with its own permission.
    check: guidance
parts:
  column:
    description: The flag and the filter.
    stack:
      any:
        names: { deleted: is_deleted }
      go:
        names: { softDelete: RequestSoftDelete, hardDelete: RequestHardDelete }
        code: |
          Every list and read adds is_deleted = false unless the caller asks
          for deleted records and may see them; the delete operation sets
          the flag, as dxlib's RequestSoftDelete does beside
          RequestHardDelete.
"""#),
    ("idioms/transactions/transactions.specarch-idiom.yaml", #"""
# yaml-language-server: $schema=https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-idiom-0.1.schema.json
specarchIdiom: "0.1"
name: transactions
version: 1.0.0
concern: transactions
stacks: [go]
reads: [paths]
description: How a change is made all or nothing.
why: Half a change is worse than none, because nothing records that it happened.
contract:
  one-per-change:
    statement: One transaction per operation that changes data, with its audit entry inside it, rolled back on any error.
    check: guidance
parts:
  transaction:
    description: The transaction and its forms.
    stack:
      go:
        code: |
          Begin with the database's transaction (DXDatabaseTx through
          TransactionBegin in dxlib), use the Tx forms of insert, update and
          delete, and commit only when the handler returns without an
          error; a panic or an error rolls back.
"""#),
    ("idioms/type-rendering/type-rendering.specarch-idiom.yaml", #"""
# yaml-language-server: $schema=https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-idiom-0.1.schema.json
specarchIdiom: "0.1"
name: type-rendering
version: 1.2.0
concern: type-rendering
stacks: [go, postgresql, sqlserver, oracle, mariadb, any]
reads: [entities]
description: |
  What each field type of the design becomes on a stack: a Go type, and a
  column type on PostgreSQL, SQL Server, Oracle and MariaDB. The design
  says what a value is (type, format, width, precision and scale, null);
  this idiom says how each stack holds it. A row matches a field by its
  JSON type, its format, whether it is an enum, and its width or precision;
  the first row that matches renders it.
why: |
  One design type, many targets. The rows are kept where the readers are,
  a generator and the agent that writes the code, so a field is rendered
  the same way in every service on the stack. The SQL rows are the ones a
  library proven in production carries, with the reasons it wrote next to
  them, and changed where SpecArch's types say more.

sources:
  oracle-data-types:
    kind: standard
    title: Oracle Database SQL Language Reference, Data Types
    edition: "19c"
    author: Oracle
    url: https://docs.oracle.com/en/database/oracle/oracle-database/19/sqlrf/Data-Types.html
  oracle-max-string-size:
    kind: standard
    title: Oracle Database Reference, MAX_STRING_SIZE
    edition: "19c"
    author: Oracle
    url: https://docs.oracle.com/en/database/oracle/oracle-database/19/refrn/MAX_STRING_SIZE.html
  oracle-lob-restrictions:
    kind: standard
    title: Oracle Database SecureFiles and Large Objects Developer's Guide, LOB Restrictions
    edition: "23ai"
    author: Oracle
    url: https://docs.oracle.com/en/database/oracle/oracle-database/23/adlob/LOB-restrictions.html
  sqlserver-nvarchar:
    kind: standard
    title: nchar and nvarchar (Transact-SQL)
    author: Microsoft
    url: https://learn.microsoft.com/en-us/sql/t-sql/data-types/nchar-and-nvarchar-transact-sql
  ieee-754:
    kind: standard
    title: IEEE Standard for Floating-Point Arithmetic, IEEE 754
    edition: "2019"
    author: IEEE

contract:
  every-field-rendered:
    statement: Every field of every entity has a row for every stack of the implementation file that this idiom renders.
    check: schema
  exact-decimal:
    statement: A decimal renders as an exact decimal on every stack, and never as a binary floating-point number; a money amount is a decimal.
    check: document
    why: A binary float cannot hold most decimal fractions exactly, so a sum of amounts drifts by fractions of a cent.
    cites:
      - { source: ieee-754, says: "Binary formats represent a value as a binary significand times a power of two, so 0.1 has no exact binary representation." }
  nullable-column:
    statement: An entity field that is nullable, or not in the entity's required list, renders as a nullable column; every other field renders as NOT NULL.
    check: document
  enum-as-text:
    statement: An enum renders as text as wide as its longest value, with a check constraint on its values, never as a native enum type.
    check: document
    why: The native enum types of PostgreSQL and MariaDB make renaming or removing a value a change of the column's type; text with a check constraint changes with one constraint.
  oracle-varchar2:
    statement: On Oracle, text renders as VARCHAR2 with character length semantics, never as VARCHAR.
    check: document
  key-width:
    statement: A key or a unique text column is at most 255 characters wide.
    check: document
    why: SQL Server indexes at most 900 bytes and MariaDB with utf8mb4 at most 3072; 255 characters fit under both.
  absent-and-null:
    statement: A field that may be left out of a request and a field whose value may be null are two things, and each stack renders them apart.
    check: guidance

parts:
  types:
    description: The rows, per stack.
    stack:
      go:
        libraries:
          github.com/shopspring/decimal:
            version: v1.4.0
            licence: MIT
            purpose: Exact decimal arithmetic for decimal fields, money included.
        rows:
          - { type: string, enum: true, render: "a named string type with one constant per value" }
          - type: string
            format: decimal
            render: decimal.Decimal
            why: The standard library has no exact decimal type, and neither float64 nor int64 holds an amount of a currency with many zeros and an exact fraction; shopspring/decimal does, with arbitrary precision.
          - { type: string, format: int64, render: int64 }
          - { type: string, format: uint64, render: uint64 }
          - { type: string, format: date-time, render: time.Time }
          - { type: string, format: date, render: time.Time, why: "The standard library has no calendar date type; the date is a time.Time at midnight UTC, and only its year, month and day are read." }
          - { type: string, format: time, render: string, why: "The standard library has no time-of-day type; the value stays the RFC 3339 partial time, HH:MM:SS." }
          - { type: string, format: duration, render: time.Duration }
          - { type: string, format: byte, render: "[]byte" }
          - { type: string, format: binary, render: "[]byte" }
          - { type: string, render: string }
          - { type: integer, format: int32, render: int32 }
          - { type: integer, format: int64, render: int64 }
          - { type: integer, format: uint64, render: uint64 }
          - { type: number, render: float64 }
          - { type: boolean, render: bool }
          - { type: array, render: "[]{items}" }
          - { type: object, render: "the entity's struct, or map[string]any for an object without properties" }
      postgresql:
        rows:
          - { type: string, enum: true, render: "VARCHAR({longestValue})", check: "{column} IN ({values})" }
          - { type: string, format: decimal, precisionAtMost: 1000, render: "NUMERIC({precision},{scale})" }
          - { type: string, format: int64, render: BIGINT }
          - { type: string, format: uint64, render: "NUMERIC(20,0)", why: "BIGINT is signed, so an unsigned 64-bit value above 2^63 - 1 needs twenty digits." }
          - { type: string, format: date-time, render: TIMESTAMP WITH TIME ZONE }
          - { type: string, format: date, render: DATE }
          - { type: string, format: time, render: TIME }
          - { type: string, format: duration, render: INTERVAL }
          - { type: string, format: uuid, render: UUID }
          - { type: string, format: byte, render: BYTEA }
          - { type: string, format: binary, render: BYTEA }
          - { type: string, maxLengthAtMost: 10485760, render: "VARCHAR({maxLength})" }
          - { type: string, render: TEXT }
          - { type: integer, format: int32, render: INT }
          - { type: integer, format: int64, render: BIGINT }
          - { type: integer, format: uint64, render: "NUMERIC(20,0)" }
          - { type: number, render: DOUBLE PRECISION }
          - { type: boolean, render: BOOLEAN }
          - { type: array, render: "{items}[]", why: "PostgreSQL has array columns, so a list is one." }
          - { type: object, render: JSONB }
      sqlserver:
        rows:
          - { type: string, enum: true, render: "NVARCHAR({longestValue})", check: "{column} IN ({values})" }
          - type: string
            format: decimal
            precisionAtMost: 38
            render: "DECIMAL({precision},{scale})"
            why: 38 digits is SQL Server's ceiling; a wider decimal has no row and is refused.
          - { type: string, format: int64, render: BIGINT }
          - { type: string, format: uint64, render: "DECIMAL(20,0)" }
          - { type: string, format: date-time, render: DATETIMEOFFSET }
          - { type: string, format: date, render: DATE }
          - { type: string, format: time, render: TIME }
          - { type: string, format: duration, render: "VARCHAR(32)", why: "SQL Server has no interval type; the value is ISO 8601 duration text." }
          - { type: string, format: uuid, render: UNIQUEIDENTIFIER }
          - { type: string, format: byte, render: "VARBINARY(MAX)" }
          - { type: string, format: binary, render: "VARBINARY(MAX)" }
          - type: string
            maxLengthAtMost: 4000
            render: "NVARCHAR({maxLength})"
            why: VARCHAR holds the code page of the column's collation unless the collation is a UTF-8 one, so it can lose characters; NVARCHAR holds Unicode on every collation, up to 4000 characters.
            cites:
              - { source: sqlserver-nvarchar, says: "nvarchar [ ( n | max ) ]: n defines the string size in byte-pairs, and can be a value from 1 through 4,000. max indicates that the maximum storage size is 2^31-1 bytes." }
          - { type: string, render: "NVARCHAR(MAX)" }
          - { type: integer, format: int32, render: INT }
          - { type: integer, format: int64, render: BIGINT }
          - { type: integer, format: uint64, render: "DECIMAL(20,0)" }
          - { type: number, render: FLOAT }
          - { type: boolean, render: BIT }
          - { type: array, render: "NVARCHAR(MAX)", check: "ISJSON({column}) = 1", why: "SQL Server has no array type; the list is JSON text, a documented representation rather than a silent loss." }
          - { type: object, render: "NVARCHAR(MAX)", check: "ISJSON({column}) = 1" }
      oracle:
        settings:
          maxStringSize: standard
        rows:
          - { type: string, enum: true, render: "VARCHAR2({longestValue} CHAR)", check: "{column} IN ({values})" }
          - type: string
            format: decimal
            precisionAtMost: 38
            render: "NUMBER({precision},{scale})"
            why: 38 digits is Oracle's ceiling; a wider decimal has no row and is refused.
          - { type: string, format: int64, render: "NUMBER(19)" }
          - { type: string, format: uint64, render: "NUMBER(20)" }
          - { type: string, format: date-time, render: TIMESTAMP WITH TIME ZONE }
          - { type: string, format: date, render: DATE, why: "Oracle's DATE also holds a time of day; a date field keeps it at midnight." }
          - { type: string, format: time, render: "INTERVAL DAY(0) TO SECOND(0)", why: "Oracle has no time-of-day type; an interval from midnight compares and sorts as a time does, which a DATE on a fixed day only does by convention." }
          - { type: string, format: duration, render: INTERVAL DAY TO SECOND }
          - { type: string, format: uuid, render: "VARCHAR2(36 CHAR)" }
          - { type: string, format: byte, render: BLOB }
          - { type: string, format: binary, render: BLOB }
          - type: string
            maxLengthAtMost: 1000
            render: "VARCHAR2({maxLength} CHAR)"
            why: |
              VARCHAR2 is Oracle's text type; VARCHAR is reserved and must not
              be used. Its limit is in bytes: 4000 under MAX_STRING_SIZE =
              STANDARD, the default, and 32767 under EXTENDED. A length in
              characters is a limit, not a capacity, since a character may take
              four bytes, so 1000 characters is the most a column always holds
              under STANDARD (8191 under EXTENDED, which a project sets in an
              override of this part).
            cites:
              - { source: oracle-data-types, clause: VARCHAR2 Data Type, says: "Maximum size is 32767 bytes or characters if MAX_STRING_SIZE = EXTENDED; 4000 bytes or characters if MAX_STRING_SIZE = STANDARD. The value of size in characters is a length constraint, not guaranteed capacity: to always store size characters, use a size of at most 8191 if EXTENDED, or 1000 if STANDARD." }
              - { source: oracle-max-string-size, says: "The default value is STANDARD, under which the limits before Oracle Database 12c apply: 4000 bytes for VARCHAR2." }
          - type: string
            render: CLOB
            why: |
              Text wider than VARCHAR2 can always hold. It costs: a CLOB cannot
              be part of an ordinary index key, cannot appear in ORDER BY,
              GROUP BY, DISTINCT or a join, and cannot be compared with = or
              IN in SQL, so such a field is never a key, a sort field or a
              filter by equality.
            cites:
              - { source: oracle-lob-restrictions, says: "LOB columns cannot be specified in an ORDER BY clause, a GROUP BY clause, an aggregate function, SELECT DISTINCT, a join, or as part of an index key." }
          - { type: integer, format: int32, render: "NUMBER(10)" }
          - { type: integer, format: int64, render: "NUMBER(19)" }
          - { type: integer, format: uint64, render: "NUMBER(20)" }
          - { type: number, render: BINARY_DOUBLE }
          - { type: boolean, render: "NUMBER(1)", check: "{column} IN (0, 1)" }
          - { type: array, render: CLOB, check: "{column} IS JSON", why: "Oracle has no array column type; the list is JSON text, a documented representation rather than a silent loss." }
          - { type: object, render: CLOB, check: "{column} IS JSON" }
      mariadb:
        rows:
          - { type: string, enum: true, render: "VARCHAR({longestValue})", check: "{column} IN ({values})" }
          - type: string
            format: decimal
            precisionAtMost: 65
            render: "DECIMAL({precision},{scale})"
            why: 65 digits is MariaDB's ceiling; a wider decimal has no row and is refused.
          - { type: string, format: int64, render: BIGINT }
          - { type: string, format: uint64, render: BIGINT UNSIGNED }
          - type: string
            format: date-time
            render: "VARCHAR(35)"
            why: MariaDB has no column type that keeps an offset, as DATETIME drops it and TIMESTAMP reads through the session's time zone. Fixed-width UTC text with nine fractional digits, 2006-01-02T15:04:05.000000000Z, is the one form whose byte order is time order, which ORDER BY, BETWEEN, MIN and MAX depend on.
          - { type: string, format: date, render: DATE }
          - { type: string, format: time, render: TIME }
          - { type: string, format: duration, render: "VARCHAR(32)", why: "MariaDB has no interval type; the value is ISO 8601 duration text." }
          - { type: string, format: uuid, render: "CHAR(36)" }
          - { type: string, format: byte, render: LONGBLOB }
          - { type: string, format: binary, render: LONGBLOB }
          - { type: string, maxLengthAtMost: 8000, render: "VARCHAR({maxLength})" }
          - { type: string, render: LONGTEXT, why: "A row of MariaDB holds at most 65535 bytes across its VARCHAR columns, so wider text goes out of the row." }
          - { type: integer, format: int32, render: INT }
          - { type: integer, format: int64, render: BIGINT }
          - { type: integer, format: uint64, render: BIGINT UNSIGNED }
          - { type: number, render: DOUBLE }
          - { type: boolean, render: BOOLEAN }
          - { type: array, render: JSON, why: "MariaDB has no array column type; the list is JSON text, a documented representation rather than a silent loss." }
          - { type: object, render: JSON }

  defaults:
    description: The literals a column default is written with on each dialect.
    stack:
      postgresql:
        names: { "true": "true", "false": "false", now: CURRENT_TIMESTAMP }
      sqlserver:
        names: { "true": "1", "false": "0", now: SYSDATETIMEOFFSET() }
      oracle:
        names: { "true": "1", "false": "0", now: SYSTIMESTAMP }
        why: Oracle refuses DEFAULT after NOT NULL, so a generator writes the default first, as dxlib does.
      mariadb:
        names: { "true": "true", "false": "false", now: CURRENT_TIMESTAMP }

  money:
    description: How a money amount is declared, so that every stack holds it exactly.
    stack:
      any:
        code: |
          A money amount is a decimal: type string, format decimal, with the
          precision and scale the field needs. For a currency with many
          zeros, precision 23 and scale 4 holds 19 integer digits and 4
          decimals, which fits the 38-digit ceiling of SQL Server and Oracle
          and the 65 of MariaDB. The currency is a field of its own, or fixed
          by the design, never part of the type.
        why: Large amounts with an exact fraction overflow a 64-bit integer of minor units once the fraction is kept, and a double loses digits; a decimal of declared precision holds both on every stack.
      go:
        libraries:
          github.com/shopspring/decimal:
            version: v1.4.0
            licence: MIT
            purpose: Exact decimal arithmetic for money.
        code: |
          A money field is a decimal.Decimal. It is read with
          decimal.NewFromString from the JSON string, never through a
          float64, and written back as a string, which is the library's
          default JSON form. Arithmetic stays in decimal.Decimal (Add, Sub,
          Mul, and Round or RoundBank to the field's scale before storing);
          converting to float64 or int64 on the way is the mistake this
          rendering exists to prevent. At the database driver, the value
          goes in and out as the NUMERIC text, not as a float.
        why: This is how dxlib carries money, with the same library, because Go's built-in number types cannot hold a large amount of a currency with many zeros and an exact fraction at once.

  absence-and-null:
    description: A field left out of a request against a field whose value is null.
    why: |
      They are two ideas. required false on a request field says the caller
      may leave the field out; a type list with null says the value may be
      null. A REST parameter that may be absent need not be a pointer in Go,
      because the request layer records whether it was given. A library
      whose type names say "nullable" for a parameter that may be left out
      is describing absence, and maps to required false, not to a type list
      with null.
    stack:
      any:
        code: |
          JSON: an absent field has no key; a null field has the key with the
          value null. JavaScript: undefined against null; never treat one as
          the other. SQL: a nullable column holds null; absence has no column
          of its own, so an entity field that is not required is a nullable
          column, as one that is nullable is.
      go:
        code: |
          Absent only (required false, not nullable): the plain type, with a
          presence flag or an accessor the request layer sets, such as
          Has(name). Null only (required, type list with null): a pointer, or
          a sql.Null type at the storage layer. Both: a pointer and the
          presence flag. Never a pointer to say only that a field may be left
          out.

  wire:
    description: How a value travels in JSON and is read in JavaScript.
    stack:
      any:
        code: |
          The design already says how each value travels: a decimal and a
          64-bit integer above 2^53 as a JSON string, everything else as its
          JSON type. In JavaScript a decimal string is never turned into a
          Number, since a Number is a binary double and loses digits; it is
          kept as text, or read into an exact decimal type. A 64-bit integer
          carried as a string is read as a BigInt.
        cites:
          - { source: ieee-754, says: "A binary64 value has a 53-bit significand, so integers above 2^53 and most decimal fractions are not exact." }
"""#),
]
