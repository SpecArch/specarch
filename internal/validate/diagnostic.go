// Package validate checks SpecArch specifications and implementation files:
// the JSON Schema of their kind first, then the rules a schema cannot express.
package validate

import (
	"fmt"
	"sort"

	"github.com/SpecArch/specarch/internal/source"
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
	RuleReference              Rule = "reference"
	RuleModule                 Rule = "module"
	RuleField                  Rule = "field"
	RuleStateField             Rule = "state_field"
	RuleStateValue             Rule = "state_value"
	RuleTrigger                Rule = "trigger"
	RuleRequirement            Rule = "requirement"
	RuleEmits                  Rule = "emits"
	RuleDependency             Rule = "dependency"
	RuleIntegration            Rule = "integration"
	RuleChannel                Rule = "channel"
	RuleIdempotencyKey         Rule = "idempotency_key"
	RuleValidity               Rule = "validity"
	RuleTimeZone               Rule = "time_zone"
	RuleSession                Rule = "session"
	RuleGuard                  Rule = "guard"
	RuleOperation              Rule = "operation"
	RulePage                   Rule = "page"
	RuleAlgorithm              Rule = "algorithm"
	RuleDecision               Rule = "decision"
	RuleEnumValue              Rule = "enum_value"
	RulePathParameter          Rule = "path_parameter"
	RuleDuplicateOperation     Rule = "duplicate_operation"
	RulePermissionUndeclared   Rule = "permission_undeclared"
	RulePermissionUngranted    Rule = "permission_ungranted"
	RuleSeparationOfDuties     Rule = "separation_of_duties"
	RuleImplies                Rule = "implies"
	RuleExpressionSyntax       Rule = "expression_syntax"
	RuleExpressionName         Rule = "expression_name"
	RuleExpressionType         Rule = "expression_type"
	RuleExampleInput           Rule = "example_input"
	RuleExampleExpected        Rule = "example_expected"
	RuleExampleMismatch        Rule = "example_mismatch"
	RuleExampleError           Rule = "example_error"
	RuleDecisionTable          Rule = "decision_table"
	RuleTableOverlap           Rule = "table_overlap"
	RuleTableGap               Rule = "table_gap"
	RuleImplements             Rule = "implements"
	RuleDesignRef              Rule = "design_ref"
	RuleTestSubject            Rule = "test_subject"
	RuleTestCase               Rule = "test_case"
	RuleTestGoldenMissing      Rule = "test_golden_missing"
	RuleTestRedMissing         Rule = "test_red_missing"
	RuleTestCaseMissing        Rule = "test_case_missing"
	RuleSuite                  Rule = "suite"
	RuleTestData               Rule = "test_data"
	RuleChangeLog              Rule = "change_log"
	RuleUnsafeInteger          Rule = "unsafe_integer"
	RuleLayout                 Rule = "layout"
	RuleDependsOn              Rule = "depends_on"
	RuleServer                 Rule = "server"
	RuleLink                   Rule = "link"
	RuleLinkMissing            Rule = "link_missing"
	RuleNeed                   Rule = "need"
	RuleStakeholder            Rule = "stakeholder"
	RuleSource                 Rule = "source"
	RuleSourceClause           Rule = "source_clause"
	RuleEnvironment            Rule = "environment"
	RuleSetting                Rule = "setting"
	RuleSecretValue            Rule = "secret_value"
	RuleMonitor                Rule = "monitor"
	RuleRecordName             Rule = "record_name"
	RuleRecordRef              Rule = "record_ref"
	RuleChangeApplied          Rule = "change_applied"
	RuleChangeDecision         Rule = "change_decision"
	RuleDefectTest             Rule = "defect_test"
	RuleDefectDuplicate        Rule = "defect_duplicate"
	RuleIncidentLink           Rule = "incident_link"
	RuleIdeaDecision           Rule = "idea_decision"
	RuleIdeaQuestion           Rule = "idea_question"
	RuleCommissioningRecord    Rule = "commissioning_record"
	RuleReleaseContents        Rule = "release_contents"
	RuleReleaseBump            Rule = "release_bump"
	RuleReleaseVersion         Rule = "release_version"
	RuleNeedUnrefined          Rule = "need_unrefined"
	RuleAcceptanceMissing      Rule = "acceptance_missing"
	RuleRequirementUnsatisfied Rule = "requirement_unsatisfied"
	RuleRequirementUnverified  Rule = "requirement_unverified"
	RuleQuestionBlock          Rule = "question_block"
	RuleQuestionStage          Rule = "question_stage"
	RuleQuestionAnswered       Rule = "question_answered"
	RuleOriginCitation         Rule = "origin_citation"
	RuleOriginReason           Rule = "origin_reason"
	RuleOriginDecision         Rule = "origin_decision"
	RuleOriginMissing          Rule = "origin_missing"
	RuleIdiomUnknown           Rule = "idiom_unknown"
	RuleIdiomPartUnknown       Rule = "idiom_part_unknown"
	RuleIdiomOverrideReason    Rule = "idiom_override_reason"
	RuleIdiomStack             Rule = "idiom_stack"
	RuleIdiomVersionBehind     Rule = "idiom_version_behind"
	RuleIdiomContract          Rule = "idiom_contract"
	RuleSensitivityExposed     Rule = "sensitivity_exposed"
	RuleAtRest                 Rule = "at_rest"
	RuleAudited                Rule = "audited"
	RuleListOf                 Rule = "list_of"
	RuleLimits                 Rule = "limits"
	RuleProblem                Rule = "problem"
	RuleJob                    Rule = "job"
	RuleGateway                Rule = "gateway"
	RuleMenu                   Rule = "menu"
	RuleView                   Rule = "view"
	RuleFlow                   Rule = "flow"
	RuleRoleHome               Rule = "role_home"
	RuleNavigation             Rule = "navigation"
	RuleState                  Rule = "state"
	RuleAccessibility          Rule = "accessibility"
	RuleTheme                  Rule = "theme"
	RuleWorkflow               Rule = "workflow"
	RulePicker                 Rule = "picker"
	RuleAction                 Rule = "action"
	RuleFormField              Rule = "form_field"
	RuleValueObject            Rule = "value_object"
	RuleValueObjectUnchecked   Rule = "value_object_unchecked"
	RuleWireName               Rule = "wire_name"
	RuleOutputFolder           Rule = "output_folder"
	RuleMetaModelVersion       Rule = "meta_model_version"
	RuleMetaModelDeprecated    Rule = "meta_model_deprecated"
	RuleAudience               Rule = "audience"
	RuleQuestionOpened         Rule = "question_opened"
	RuleHandover               Rule = "handover"
	RuleBuildTask              Rule = "build_task"
	RuleRequestBodyOpen        Rule = "request_body_open"
	RuleSuccessResponseMissing Rule = "success_response_missing"
	RuleMiddleware             Rule = "middleware"
	RuleMiddlewareMissing      Rule = "middleware_missing"
	RuleHandlerSource          Rule = "handler_source"
	RuleHandshake              Rule = "handshake"
	RuleStream                 Rule = "stream"
	RuleCallerProperty         Rule = "caller_property"
	RuleRowPolicies            Rule = "row_policies"
	RuleTenant                 Rule = "tenant"
	RuleReservedName           Rule = "reserved_name"
	RuleClient                 Rule = "client"
	RuleClientUnused           Rule = "client_unused"
	RuleSoftwareVersion        Rule = "software_version"
	RuleThrough                Rule = "through"
	RuleCallers                Rule = "callers"
	RuleProof                  Rule = "proof"
	RuleToken                  Rule = "token"
	RuleClaim                  Rule = "claim"
	RuleVia                    Rule = "via"
	RuleTokenAudience          Rule = "token_audience"
	RuleSecurityEvents         Rule = "security_events"
	RuleSignIn                 Rule = "sign_in"
	RuleExitCode               Rule = "exit_code"
	RuleDecisionCycle          Rule = "decision_cycle"
	RuleDecisionSuperseded     Rule = "decision_superseded"
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
	RuleReference,
	RuleModule,
	RuleField,
	RuleStateField,
	RuleStateValue,
	RuleTrigger,
	RuleRequirement,
	RuleEmits,
	RuleDependency,
	RuleIntegration,
	RuleChannel,
	RuleIdempotencyKey,
	RuleValidity,
	RuleTimeZone,
	RuleSession,
	RuleGuard,
	RuleOperation,
	RulePage,
	RuleAlgorithm,
	RuleDecision,
	RuleEnumValue,
	RulePathParameter,
	RuleDuplicateOperation,
	RulePermissionUndeclared,
	RulePermissionUngranted,
	RuleSeparationOfDuties,
	RuleImplies,
	RuleExpressionSyntax,
	RuleExpressionName,
	RuleExpressionType,
	RuleExampleInput,
	RuleExampleExpected,
	RuleExampleMismatch,
	RuleExampleError,
	RuleDecisionTable,
	RuleTableOverlap,
	RuleTableGap,
	RuleImplements,
	RuleDesignRef,
	RuleTestSubject,
	RuleTestCase,
	RuleTestGoldenMissing,
	RuleTestRedMissing,
	RuleTestCaseMissing,
	RuleSuite,
	RuleTestData,
	RuleChangeLog,
	RuleUnsafeInteger,
	RuleLayout,
	RuleDependsOn,
	RuleServer,
	RuleLink,
	RuleLinkMissing,
	RuleNeed,
	RuleStakeholder,
	RuleSource,
	RuleSourceClause,
	RuleEnvironment,
	RuleSetting,
	RuleSecretValue,
	RuleMonitor,
	RuleRecordName,
	RuleRecordRef,
	RuleChangeApplied,
	RuleChangeDecision,
	RuleDefectTest,
	RuleDefectDuplicate,
	RuleIncidentLink,
	RuleIdeaDecision,
	RuleIdeaQuestion,
	RuleCommissioningRecord,
	RuleReleaseContents,
	RuleReleaseBump,
	RuleReleaseVersion,
	RuleNeedUnrefined,
	RuleAcceptanceMissing,
	RuleRequirementUnsatisfied,
	RuleRequirementUnverified,
	RuleQuestionBlock,
	RuleQuestionStage,
	RuleQuestionAnswered,
	RuleOriginCitation,
	RuleOriginReason,
	RuleOriginDecision,
	RuleOriginMissing,
	RuleIdiomUnknown,
	RuleIdiomPartUnknown,
	RuleIdiomOverrideReason,
	RuleIdiomStack,
	RuleIdiomVersionBehind,
	RuleIdiomContract,
	RuleSensitivityExposed,
	RuleAtRest,
	RuleAudited,
	RuleListOf,
	RuleLimits,
	RuleProblem,
	RuleJob,
	RuleGateway,
	RuleMenu,
	RuleView,
	RuleFlow,
	RuleRoleHome,
	RuleNavigation,
	RuleState,
	RuleAccessibility,
	RuleTheme,
	RuleWorkflow,
	RulePicker,
	RuleAction,
	RuleFormField,
	RuleValueObject,
	RuleValueObjectUnchecked,
	RuleWireName,
	RuleOutputFolder,
	RuleMetaModelVersion,
	RuleMetaModelDeprecated,
	RuleAudience,
	RuleQuestionOpened,
	RuleHandover,
	RuleBuildTask,
	RuleRequestBodyOpen,
	RuleSuccessResponseMissing,
	RuleMiddleware,
	RuleMiddlewareMissing,
	RuleHandlerSource,
	RuleHandshake,
	RuleStream,
	RuleCallerProperty,
	RuleRowPolicies,
	RuleTenant,
	RuleReservedName,
	RuleClient,
	RuleClientUnused,
	RuleSoftwareVersion,
	RuleThrough,
	RuleCallers,
	RuleProof,
	RuleToken,
	RuleClaim,
	RuleVia,
	RuleTokenAudience,
	RuleSecurityEvents,
	RuleSignIn,
	RuleExitCode,
	RuleDecisionCycle,
	RuleDecisionSuperseded,
}

// Severity says whether a diagnostic makes the file invalid.
type Severity string

const (
	// Error makes the file invalid.
	Error Severity = "error"
	// Warning is printed but leaves the file valid: missing test scenarios,
	// change-log phrases, traceability gaps and incidents left unexplained.
	Warning Severity = "warning"
)

// Diagnostic is one problem in one file.
type Diagnostic struct {
	File     string
	Line     int
	Column   int
	Severity Severity
	Path     string
	Rule     Rule
	Message  string
	// ID names what is wrong, not where it is printed (ADR-065): set by
	// a Placer.
	ID string
	// Detail is the YAML parser's own message for a yaml_syntax error,
	// which differs between parsers; it is printed on standard error, never
	// in the problem line (ADR-177).
	Detail string
}

// String is the problem line: file:line:column: severity: /yaml/path:
// rule: message [id].
func (d Diagnostic) String() string {
	return fmt.Sprintf("%s:%d:%d: %s: %s: %s: %s [%s]", d.File, d.Line, d.Column, d.Severity, d.Path, d.Rule, d.Message, d.ID)
}

// DetailLine is the line that prints the YAML parser's own message on
// standard error, after the fixed marker "yaml parser"; empty when there is
// none.
func (d Diagnostic) DetailLine() string {
	if d.Detail == "" {
		return ""
	}
	return fmt.Sprintf("%s:%d:%d: yaml parser (%s): %s", d.File, d.Line, d.Column, source.ParserName, d.Detail)
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

// Sort orders diagnostics by file, line, column, path, rule, message and
// id.
func Sort(ds []Diagnostic) {
	sort.SliceStable(ds, func(i, j int) bool {
		a, b := ds[i], ds[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Column != b.Column {
			return a.Column < b.Column
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Rule != b.Rule {
			return a.Rule < b.Rule
		}
		if a.Message != b.Message {
			return a.Message < b.Message
		}
		return a.ID < b.ID
	})
}
