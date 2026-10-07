// Written by embed-schemas.sh from ../schema. Do not edit; run the script.

let designSchemaJSON = #"""
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-design-0.1.schema.json",
  "title": "SpecArch Definition File, meta-model 0.1",
  "description": "A SpecArch Definition File (*.specarch-design.yaml) describes one system: its entities, enums, relations, constraints, endpoints, commands, events, roles and permissions, pages, algorithms, requirement links and decisions. Field keywords come from JSON Schema, endpoint keywords from OpenAPI 3, event keywords from AsyncAPI. Keywords with no standard origin are marked 'SpecArch keyword' in their description. Every object rejects unknown keys; keys starting with 'x-' are allowed everywhere as extensions.",
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
    "requirementSources": {
      "description": "SpecArch keyword. External requirement sets that requirement links point into, keyed by the prefix used in link IDs.",
      "type": "object",
      "propertyNames": {
        "pattern": "^[A-Z][A-Z0-9]{1,15}$"
      },
      "additionalProperties": {
        "$ref": "#/$defs/requirementSource"
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
      "description": "SpecArch keyword. Design tests: what must hold whatever the stack, in plain given, when and then sentences. Each test is about one subject (an operation, a command, a page, or an entity's constraint or transition) and is marked golden (the path that succeeds) or red (a path that fails). The validator derives the cases each subject needs from the rest of the file and reports every one no test covers; see 'Tests' in docs/conventions.md.",
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
    "requirementLink": {
      "description": "SpecArch keyword. An ID in an external requirement set: the prefix names a requirementSources entry, the rest is the ID inside that set. Every named object carries a 'requirements' list of these (entity, enum, relation, constraint, transition, permission, role, operation, command, channel, message, page, algorithm, decision); fields, parameters, responses, actions and worked examples trace through the object that holds them.",
      "type": "string",
      "pattern": "^[A-Z][A-Z0-9]{1,15}-[A-Za-z0-9._]+$"
    },
    "requirementLinks": {
      "type": "array",
      "items": {
        "$ref": "#/$defs/requirementLink"
      },
      "uniqueItems": true
    },
    "markdown": {
      "description": "Prose in Markdown.",
      "type": "string"
    },
    "schemaRef": {
      "description": "A JSON pointer into this document, such as '#/entities/Member' or '#/enums/LoanStatus'.",
      "type": "string",
      "pattern": "^#/(entities|enums)/[A-Z][A-Za-z0-9]*$"
    },
    "info": {
      "description": "OpenAPI keyword. What the system is.",
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
    "requirementSource": {
      "type": "object",
      "properties": {
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "url": {
          "description": "Where the requirement set lives; may be a URL template with {id}.",
          "type": "string"
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
        "requirements": {
          "$ref": "#/$defs/requirementLinks"
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
        "requirements": {
          "$ref": "#/$defs/requirementLinks"
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
        "requirements": {
          "$ref": "#/$defs/requirementLinks"
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
        "requirements": {
          "$ref": "#/$defs/requirementLinks"
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
        "requirements": {
          "$ref": "#/$defs/requirementLinks"
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
    "permission": {
      "type": "object",
      "properties": {
        "description": {
          "$ref": "#/$defs/markdown"
        },
        "requirements": {
          "$ref": "#/$defs/requirementLinks"
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
        "requirements": {
          "$ref": "#/$defs/requirementLinks"
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
        "algorithm": {
          "description": "SpecArch keyword. The algorithm this operation runs, if any.",
          "$ref": "#/$defs/memberName"
        },
        "requirements": {
          "$ref": "#/$defs/requirementLinks"
        },
        "deprecated": {
          "type": "boolean"
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
        "requirements": {
          "$ref": "#/$defs/requirementLinks"
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
        "requirements": {
          "$ref": "#/$defs/requirementLinks"
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
        "requirements": {
          "$ref": "#/$defs/requirementLinks"
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
        "requirements": {
          "$ref": "#/$defs/requirementLinks"
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
      "description": "SpecArch keyword. A short architecture decision record.",
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
        "requirements": {
          "$ref": "#/$defs/requirementLinks"
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
        "algorithm": {
          "description": "SpecArch keyword. The algorithm this command runs, if any.",
          "$ref": "#/$defs/memberName"
        },
        "requirements": {
          "$ref": "#/$defs/requirementLinks"
        },
        "deprecated": {
          "type": "boolean"
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
      "description": "SpecArch keyword. One test scenario. Exactly one subject: operation, command, page, or entity with constraint or transition.",
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
        "entity": {
          "description": "The entity whose constraint or transition is under test.",
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
        "requirements": {
          "$ref": "#/$defs/requirementLinks"
        }
      },
      "required": [
        "scenario"
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
            "oneOf": [
              {
                "required": [
                  "constraint"
                ]
              },
              {
                "required": [
                  "transition"
                ]
              }
            ]
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
    }
  }
}
"""#

let implementationSchemaJSON = #"""
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-implementation-0.1.schema.json",
  "title": "SpecArch Implementation File, meta-model 0.1",
  "description": "A SpecArch Implementation File (*.specarch-implementation.yaml) says how one stack implements one SpecArch Definition File: the language and toolchain, the libraries and their licences, the package layout, how each design object maps onto the stack, the generator settings, the build and test tasks, the deployments, and the implementation decisions. It points at the design file it implements and cannot add or change design: entities, operations, commands, events, permissions, pages and algorithms exist only in the design file, and this schema has no keyword for them. One design file can have several implementation files, one per stack. Every object rejects unknown keys; keys starting with 'x-' are allowed everywhere as extensions.",
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
      "description": "The design file this file implements.",
      "type": "object",
      "properties": {
        "file": {
          "description": "Path of the design file, relative to this file. Must end in .specarch-design.yaml.",
          "type": "string",
          "pattern": "\\.specarch-design\\.yaml$"
        },
        "version": {
          "description": "The design file's info.version this implementation was written against. The validator fails when they differ, so a design change is noticed by every implementation of it.",
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
    "target": {
      "description": "The language and toolchain.",
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
    "generators": {
      "description": "Generator targets this implementation uses, keyed by target name, with their settings. Defaults: the sql target writes PostgreSQL; the ui target writes plain JavaScript for the web and SwiftUI for the iPhone.",
      "type": "object",
      "propertyNames": {
        "pattern": "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"
      },
      "additionalProperties": {
        "$ref": "#/$defs/generator"
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
      "description": "Real deployments, keyed by environment name: servers, hosts and ports.",
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
    "target"
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
      "description": "A JSON pointer into the design file, naming one of its objects: '#/entities/Loan', '#/commands/validate', '#/paths/~1loans/post'. The validator checks it resolves.",
      "type": "string",
      "pattern": "^#/(entities|enums|permissions|roles|paths|commands|channels|pages|algorithms|decisions)/[^/]+(/.+)?$"
    },
    "requirementLinks": {
      "type": "array",
      "items": {
        "type": "string",
        "pattern": "^[A-Z][A-Z0-9]{1,15}-[A-Za-z0-9._]+$"
      },
      "uniqueItems": true
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
    "generator": {
      "type": "object",
      "properties": {
        "output": {
          "description": "The folder this generator owns, relative to this implementation file.",
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
        "requirements": {
          "$ref": "#/$defs/requirementLinks"
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
        }
      },
      "required": [
        "description",
        "run"
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
      "additionalProperties": false
    }
  }
}
"""#
