// Written by embed-schemas.sh from ../schema. Do not edit; run the script.

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
      "description": "A JSON pointer into the specification, such as '#/entities/Member' or '#/enums/LoanStatus'. The pointer names the object wherever its file is in the tree.",
      "type": "string",
      "pattern": "^#/(entities|enums)/[A-Z][A-Za-z0-9]*$"
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
          "description": "What the source is: a published standard; a law or regulation; a document (a book, a manual, a report, an internal document); an interview with a stakeholder; an existing system that was observed; or an external tracker that links point into: a requirement set for requirement links, a change set for the change requests and a defect set for the defects that records name.",
          "type": "string",
          "enum": [
            "standard",
            "regulation",
            "document",
            "interview",
            "system",
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
        "prefix": {
          "description": "For kind requirement-set, change-set or defect-set: the prefix of the IDs in that set, such as JIRA, so that a link JIRA-12 resolves to it. One tracker that holds several kinds is declared once per kind, and the prefixes may be the same.",
          "type": "string",
          "pattern": "^[A-Z][A-Z0-9]{1,15}$"
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
      "description": "SpecArch keyword. A screen of one of three kinds. Generators map it onto the target project's component library.",
      "type": "object",
      "properties": {
        "kind": {
          "type": "string",
          "enum": [
            "list",
            "form",
            "view"
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
          "description": "For a form: operationId that receives it.",
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
        "fields": {
          "description": "For a form or view: fields shown, in order.",
          "type": "array",
          "items": {
            "$ref": "#/$defs/memberName"
          },
          "minItems": 1,
          "uniqueItems": true
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
        "entity",
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
              "source"
            ]
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
              "fields",
              "submit"
            ]
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
              "fields",
              "source"
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
        }
      },
      "required": [
        "label",
        "kind",
        "target"
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
      "description": "SpecArch keyword. One test scenario. Exactly one subject: operation, command, page, requirement, or entity, alone for its state machine or with one constraint or transition. In a folder tree it is the content of tests/<name>/test.yaml.",
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
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "settings": {
          "description": "Generator- or framework-specific settings for this object, such as code-generator extensions.",
          "type": "object"
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
          "description": "For the sql target: the database dialect. The default is postgresql, the only dialect so far.",
          "type": "string",
          "enum": [
            "postgresql"
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
