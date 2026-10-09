import SwiftUI

struct ItemFormView: View {
    @State private var draft = ItemDraft()
    @State private var code = ""

    var body: some View {
        NavigationStack {
            Form {
                TextField("Name", text: $draft.name)
                SecureField("Code", text: $code)
                Picker("Kind", selection: $draft.kind) {
                    Text("Book")
                }
            }
            .navigationTitle("New item")
            .navigationTitle("Add")
        }
    }
}

struct ItemDraft {
    var name = ""
    var kind = ""
}
