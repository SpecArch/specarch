import SwiftUI

struct SettingsView: View {
    @AppStorage("readerName") private var readerName = ""
    private var heading: String { readerName.isEmpty ? "Settings" : readerName }

    var body: some View {
        NavigationStack {
            Form {
                TextField("Your name", text: $readerName)
            }
            .navigationTitle(heading)
        }
    }
}
