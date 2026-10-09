// swift-tools-version:6.0
import PackageDescription

// The Swift reader of specarch extract: it parses Swift source with
// SwiftSyntax and writes the facts specarch extract swift reads, as a
// code-facts dump (docs/reading-code.md). tools/code-facts/dump-swift.sh
// runs it. The version of swift-syntax is pinned exactly, since
// specarch refuses a dump that another parser version made.
let package = Package(
    name: "code-facts-swift",
    platforms: [.macOS(.v13)],
    products: [
        .executable(name: "code-facts-swift", targets: ["code-facts-swift"]),
    ],
    dependencies: [
        .package(url: "https://github.com/swiftlang/swift-syntax.git", exact: "604.0.0"),
    ],
    targets: [
        .executableTarget(name: "code-facts-swift", dependencies: [
            .product(name: "SwiftSyntax", package: "swift-syntax"),
            .product(name: "SwiftParser", package: "swift-syntax"),
        ]),
    ]
)
