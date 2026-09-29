// swift-tools-version: 6.3

import PackageDescription

let package = Package(
    name: "SwiftixEditors",
    platforms: [.macOS(.v14)],
    dependencies: [
        .package(path: "../Swiftix"),
    ],
    targets: [
        .executableTarget(
            name: "EditorsPackageBuilder",
            dependencies: [
                .product(name: "SwiftixGo", package: "Swiftix"),
                .product(name: "SwiftixPackages", package: "Swiftix"),
            ]
        ),
        .testTarget(
            name: "EditorsTests",
            dependencies: [
                .product(name: "Swiftix", package: "Swiftix"),
                .product(name: "SwiftixGo", package: "Swiftix"),
                .product(name: "SwiftixPackages", package: "Swiftix"),
            ]
        ),
    ],
    swiftLanguageModes: [.v6]
)
