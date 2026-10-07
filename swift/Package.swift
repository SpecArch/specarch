// swift-tools-version:6.0
import PackageDescription

let package = Package(
    name: "specarch",
    platforms: [.macOS(.v13)],
    products: [
        .executable(name: "specarch", targets: ["specarch"]),
    ],
    dependencies: [
        .package(url: "https://github.com/jpsim/Yams.git", exact: "6.2.2"),
    ],
    targets: [
        .target(name: "SpecArchKit", dependencies: ["Yams"]),
        .executableTarget(name: "specarch", dependencies: ["SpecArchKit"]),
        .testTarget(name: "SpecArchKitTests", dependencies: ["SpecArchKit", "Yams"]),
    ]
)
