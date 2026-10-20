// swift-tools-version:6.0
import PackageDescription

// The library lending screens for the iPhone: the generated Screens module,
// the app around it, and the tests generate derives. The package has no
// dependency: the screens are built on SwiftUI and Foundation alone.
let package = Package(
    name: "LibraryLending",
    platforms: [.iOS(.v17), .macOS(.v14)],
    products: [
        .library(name: "Screens", targets: ["Screens"]),
        .executable(name: "LibraryLendingApp", targets: ["LibraryLendingApp"]),
    ],
    targets: [
        .target(name: "Screens"),
        .executableTarget(name: "LibraryLendingApp", dependencies: ["Screens"]),
        .testTarget(name: "ScreensTests", dependencies: ["Screens"]),
    ]
)
