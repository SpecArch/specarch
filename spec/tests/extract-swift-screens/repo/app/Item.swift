import Foundation
import SwiftData

@Model
final class Item {
    var name: String
    var lent: Bool
    var bought: Date
    var place: Place

    init(name: String, place: Place) {
        self.name = name
        self.lent = false
        self.bought = .now
        self.place = place
    }
}

struct Place: Codable {
    var city: String
}
