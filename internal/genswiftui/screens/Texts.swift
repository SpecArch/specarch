import Foundation

/// The texts a page shows, by string key.
public typealias Texts = [String: String]

/// The text of a key, or the key when the texts hold none.
public func say(_ texts: Texts, _ key: String) -> String {
    texts[key] ?? key
}

/// The text of a message, its placeholders filled.
public func say(_ texts: Texts, _ message: Message) -> String {
    var text = texts[message.text] ?? message.text
    if let count = message.count {
        text = text.replacingOccurrences(of: "{count}", with: countText(count))
    }
    if let pattern = message.pattern {
        text = text.replacingOccurrences(of: "{pattern}", with: pattern)
    }
    return text
}

/// A count as the text shows it: whole when it is, else as written.
func countText(_ count: Double) -> String {
    if count == count.rounded(), abs(count) < 1e15 {
        return String(Int(count))
    }
    return "\(count)"
}
