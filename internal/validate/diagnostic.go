// Package validate checks SpecArch specifications and implementation files:
// the JSON Schema of their kind first, then the rules a schema cannot express.
package validate

import (
	"fmt"
	"sort"
)

// Rule names one check. The values are the Rule enum of
// spec/specarch.specarch-design.yaml.
type Rule string

const (
	RuleFileKind               Rule = "file_kind"
	RuleYAMLSyntax             Rule = "yaml_syntax"
	RuleUnquotedDate           Rule = "unquoted_date"
	RuleDuplicateKey           Rule = "duplicate_key"
	RuleSchema                 Rule = "schema"
	RuleStackKey               Rule = "stack_key"
	RuleDesignKey              Rule = "design_key"
	RuleRefType                Rule = "ref_type"
	RuleRelationTarget         Rule = "relation_target"
	RuleRelationVia            Rule = "relation_via"
	RuleField                  Rule = "field"
	RuleStateField             Rule = "state_field"
	RuleStateValue             Rule = "state_value"
	RuleTrigger                Rule = "trigger"
	RuleRequirement            Rule = "requirement"
	RuleEmits                  Rule = "emits"
	RuleOperation              Rule = "operation"
	RulePage                   Rule = "page"
	RuleAlgorithm              Rule = "algorithm"
	RuleDecision               Rule = "decision"
	RuleEnumValue              Rule = "enum_value"
	RulePathParameter          Rule = "path_parameter"
	RuleDuplicateOperation     Rule = "duplicate_operation"
	RulePermissionUndeclared   Rule = "permission_undeclared"
	RulePermissionUngranted    Rule = "permission_ungranted"
	RuleExpressionSyntax       Rule = "expression_syntax"
	RuleExpressionName         Rule = "expression_name"
	RuleExpressionType         Rule = "expression_type"
	RuleExampleInput           Rule = "example_input"
	RuleExampleExpected        Rule = "example_expected"
	RuleExampleMismatch        Rule = "example_mismatch"
	RuleExampleError           Rule = "example_error"
	RuleImplements             Rule = "implements"
	RuleDesignRef              Rule = "design_ref"
	RuleTestSubject            Rule = "test_subject"
	RuleTestCase               Rule = "test_case"
	RuleTestGoldenMissing      Rule = "test_golden_missing"
	RuleTestRedMissing         Rule = "test_red_missing"
	RuleTestCaseMissing        Rule = "test_case_missing"
	RuleSuite                  Rule = "suite"
	RuleChangeLog              Rule = "change_log"
	RuleUnsafeInteger          Rule = "unsafe_integer"
	RuleLayout                 Rule = "layout"
	RuleNeed                   Rule = "need"
	RuleStakeholder            Rule = "stakeholder"
	RuleSource                 Rule = "source"
	RuleEnvironment            Rule = "environment"
	RuleSetting                Rule = "setting"
	RuleSecretValue            Rule = "secret_value"
	RuleNeedUnrefined          Rule = "need_unrefined"
	RuleAcceptanceMissing      Rule = "acceptance_missing"
	RuleRequirementUnsatisfied Rule = "requirement_unsatisfied"
	RuleRequirementUnverified  Rule = "requirement_unverified"
)

// Rules lists every rule, in the order of the design's Rule enum.
var Rules = []Rule{
	RuleFileKind,
	RuleYAMLSyntax,
	RuleUnquotedDate,
	RuleDuplicateKey,
	RuleSchema,
	RuleStackKey,
	RuleDesignKey,
	RuleRefType,
	RuleRelationTarget,
	RuleRelationVia,
	RuleField,
	RuleStateField,
	RuleStateValue,
	RuleTrigger,
	RuleRequirement,
	RuleEmits,
	RuleOperation,
	RulePage,
	RuleAlgorithm,
	RuleDecision,
	RuleEnumValue,
	RulePathParameter,
	RuleDuplicateOperation,
	RulePermissionUndeclared,
	RulePermissionUngranted,
	RuleExpressionSyntax,
	RuleExpressionName,
	RuleExpressionType,
	RuleExampleInput,
	RuleExampleExpected,
	RuleExampleMismatch,
	RuleExampleError,
	RuleImplements,
	RuleDesignRef,
	RuleTestSubject,
	RuleTestCase,
	RuleTestGoldenMissing,
	RuleTestRedMissing,
	RuleTestCaseMissing,
	RuleSuite,
	RuleChangeLog,
	RuleUnsafeInteger,
	RuleLayout,
	RuleNeed,
	RuleStakeholder,
	RuleSource,
	RuleEnvironment,
	RuleSetting,
	RuleSecretValue,
	RuleNeedUnrefined,
	RuleAcceptanceMissing,
	RuleRequirementUnsatisfied,
	RuleRequirementUnverified,
}

// Severity says whether a diagnostic makes the file invalid.
type Severity string

const (
	// Error makes the file invalid.
	Error Severity = "error"
	// Warning is printed but leaves the file valid: missing test scenarios,
	// change-log phrases and traceability gaps.
	Warning Severity = "warning"
)

// Diagnostic is one problem in one file.
type Diagnostic struct {
	File     string
	Line     int
	Severity Severity
	Path     string
	Rule     Rule
	Message  string
}

// String is the one-line form: file:line: severity: /yaml/path: rule: message.
func (d Diagnostic) String() string {
	return fmt.Sprintf("%s:%d: %s: %s: %s: %s", d.File, d.Line, d.Severity, d.Path, d.Rule, d.Message)
}

// Errors counts the diagnostics that make a file invalid.
func Errors(ds []Diagnostic) int {
	n := 0
	for _, d := range ds {
		if d.Severity == Error {
			n++
		}
	}
	return n
}

// Sort orders diagnostics by file, line, path, rule and message.
func Sort(ds []Diagnostic) {
	sort.SliceStable(ds, func(i, j int) bool {
		a, b := ds[i], ds[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Rule != b.Rule {
			return a.Rule < b.Rule
		}
		return a.Message < b.Message
	})
}
