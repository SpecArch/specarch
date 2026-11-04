import Foundation
import Testing
@testable import SpecArchKit

/// The portable SHA-256 the validator uses where CryptoKit is missing gives
/// the FIPS 180-2 examples' digests, across the padding's block boundaries,
/// and the same digest as sha256Hex, CryptoKit's on macOS.
@Test func portableSHA256MatchesTheStandardsExamples() {
    let cases: [(String, String)] = [
        ("", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"),
        ("abc", "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"),
        ("abcdbcdecdefdefgefghfghighijhijkijkljklmklmnlmnomnopnopq",
         "248d6a61d20638b8e5c026930c3e6039a33ce45964ff2167f6ecedd419db06c1"),
    ]
    for (text, want) in cases {
        #expect(PortableSHA256.hex(Data(text.utf8)) == want)
        #expect(sha256Hex(Data(text.utf8)) == want)
    }
    let million = Data(repeating: UInt8(ascii: "a"), count: 1_000_000)
    #expect(PortableSHA256.hex(million) == "cdc76e5c9914fb9281a1c7e284d73e67f1809a48a497200e046d39ccc7112cd0")
    for length in [55, 56, 63, 64, 65, 119, 120, 128] {
        let data = Data((0..<length).map { UInt8(truncatingIfNeeded: $0 &* 31 &+ 7) })
        #expect(PortableSHA256.hex(data) == sha256Hex(data), "length \(length)")
    }
}
