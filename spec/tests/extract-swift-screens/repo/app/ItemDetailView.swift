import SwiftUI

struct ItemDetailView: View {
    @Bindable var item: Item

    var body: some View {
        Form {
            TextField("Name", text: $item.name)
            Toggle("Lent out", isOn: $item.lent)
            DatePicker("Bought", selection: $item.bought)
            TextField("City", text: $item.place.city)
            NavigationLink(destination: HistoryView()) {
                Text("History")
            }
            NavigationLink {
                NotesView()
            } label: {
                Label("Notes", systemImage: "note.text")
            }
            ItemRow(item: item)
        }
        .navigationTitle(item.name)
    }
}

struct ItemRow: View {
    let item: Item

    var body: some View {
        NavigationLink("Share") {
            ShareView()
        }
    }
}
