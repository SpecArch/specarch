import SwiftUI

struct ReviewListView: View {
    let isbn: String
    @State private var reviews: [Review] = []
    private let client = ReviewClient()

    var body: some View {
        List(reviews) { review in
            Text(review.text)
        }
        .navigationTitle("Reviews")
        .task {
            reviews = (try? await client.reviews(isbn: isbn)) ?? []
        }
    }
}
