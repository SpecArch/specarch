import Foundation

/// Every check the validator makes: the Rule enum of
/// the specification in spec/.
public enum Rule: String, CaseIterable, Sendable {
    case fileKind = "file_kind"
    case yamlSyntax = "yaml_syntax"
    case unquotedDate = "unquoted_date"
    case duplicateKey = "duplicate_key"
    case schema = "schema"
    case stackKey = "stack_key"
    case designKey = "design_key"
    case refType = "ref_type"
    case relationTarget = "relation_target"
    case relationVia = "relation_via"
    case field = "field"
    case stateField = "state_field"
    case stateValue = "state_value"
    case trigger = "trigger"
    case requirement = "requirement"
    case emits = "emits"
    case dependency = "dependency"
    case idempotencyKey = "idempotency_key"
    case validity = "validity"
    case session = "session"
    case guardRule = "guard"
    case operation = "operation"
    case page = "page"
    case algorithm = "algorithm"
    case decision = "decision"
    case enumValue = "enum_value"
    case pathParameter = "path_parameter"
    case duplicateOperation = "duplicate_operation"
    case permissionUndeclared = "permission_undeclared"
    case permissionUngranted = "permission_ungranted"
    case expressionSyntax = "expression_syntax"
    case expressionName = "expression_name"
    case expressionType = "expression_type"
    case exampleInput = "example_input"
    case exampleExpected = "example_expected"
    case exampleMismatch = "example_mismatch"
    case exampleError = "example_error"
    case implements = "implements"
    case designRef = "design_ref"
    case unsafeInteger = "unsafe_integer"
    case changeLog = "change_log"
    case testSubject = "test_subject"
    case testCase = "test_case"
    case testGoldenMissing = "test_golden_missing"
    case testRedMissing = "test_red_missing"
    case testCaseMissing = "test_case_missing"
    case suite = "suite"
    case testData = "test_data"
    case layout = "layout"
    case need = "need"
    case stakeholder = "stakeholder"
    case source = "source"
    case environment = "environment"
    case setting = "setting"
    case secretValue = "secret_value"
    case monitor = "monitor"
    case recordName = "record_name"
    case recordRef = "record_ref"
    case changeApplied = "change_applied"
    case changeDecision = "change_decision"
    case defectTest = "defect_test"
    case defectDuplicate = "defect_duplicate"
    case incidentLink = "incident_link"
    case commissioningRecord = "commissioning_record"
    case releaseContents = "release_contents"
    case releaseBump = "release_bump"
    case releaseVersion = "release_version"
    case needUnrefined = "need_unrefined"
    case acceptanceMissing = "acceptance_missing"
    case requirementUnsatisfied = "requirement_unsatisfied"
    case requirementUnverified = "requirement_unverified"
    case questionBlock = "question_block"
    case questionStage = "question_stage"
    case questionAnswered = "question_answered"
    case originCitation = "origin_citation"
    case originReason = "origin_reason"
    case originDecision = "origin_decision"
    case originMissing = "origin_missing"
    case idiomUnknown = "idiom_unknown"
    case idiomPartUnknown = "idiom_part_unknown"
    case idiomOverrideReason = "idiom_override_reason"
    case idiomStack = "idiom_stack"
    case idiomVersionBehind = "idiom_version_behind"
    case idiomContract = "idiom_contract"
    case sensitivityExposed = "sensitivity_exposed"
    case atRest = "at_rest"
    case audited = "audited"
    case listOf = "list_of"
    case limits = "limits"
    case problem = "problem"
    case job = "job"
    case menu = "menu"
    case view = "view"
    case flow = "flow"
    case state = "state"
}

/// Whether a diagnostic makes the file invalid.
public enum Severity: String, Sendable {
    case error
    case warning
}

/// One problem in one file.
public struct Diagnostic: CustomStringConvertible {
    public let file: String
    public let line: Int
    public var severity: Severity
    public let path: String
    public let rule: Rule
    public let message: String

    /// The one-line form: file:line: severity: /yaml/path: rule: message.
    public var description: String {
        "\(file):\(line): \(severity.rawValue): \(path): \(rule.rawValue): \(message)"
    }
}

/// Compares strings byte by byte, as every SpecArch implementation sorts.
public func byteLess(_ a: String, _ b: String) -> Bool {
    a.utf8.lexicographicallyPrecedes(b.utf8)
}

/// Orders diagnostics by file, line, path, rule and message.
public func sortDiagnostics(_ ds: inout [Diagnostic]) {
    ds = ds.enumerated().sorted { x, y in
        let a = x.element, b = y.element
        if a.file != b.file { return byteLess(a.file, b.file) }
        if a.line != b.line { return a.line < b.line }
        if a.path != b.path { return byteLess(a.path, b.path) }
        if a.rule != b.rule { return byteLess(a.rule.rawValue, b.rule.rawValue) }
        if a.message != b.message { return byteLess(a.message, b.message) }
        return x.offset < y.offset
    }.map { $0.element }
}

/// Counts the diagnostics that make a file invalid.
public func errorCount(_ ds: [Diagnostic]) -> Int {
    ds.filter { $0.severity == .error }.count
}
