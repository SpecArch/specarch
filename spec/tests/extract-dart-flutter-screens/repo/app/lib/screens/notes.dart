import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

class NotesScreen extends StatelessWidget {
  const NotesScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Notes')),
      floatingActionButton: FloatingActionButton(onPressed: () => context.go('/notes/new')),
      body: ListTile(onTap: () => context.push('/notes/42')),
    );
  }
}

class NoteScreen extends StatelessWidget {
  const NoteScreen({super.key, required this.id});
  final String id;

  @override
  Widget build(BuildContext context) {
    return TextButton(
      onPressed: () => Navigator.push(context, MaterialPageRoute(builder: (_) => const HistoryScreen())),
      child: const Text('History'),
    );
  }
}

class HistoryScreen extends StatelessWidget {
  const HistoryScreen({super.key});

  @override
  Widget build(BuildContext context) => const Scaffold(appBar: AppBar(title: Text('History')));
}

class NoteFormScreen extends StatefulWidget {
  const NoteFormScreen({super.key});

  @override
  State<NoteFormScreen> createState() => _NoteFormScreenState();
}

class _NoteFormScreenState extends State<NoteFormScreen> {
  final _draft = NoteDraft();
  final _tagController = TextEditingController();

  @override
  Widget build(BuildContext context) {
    return Form(
      child: Column(children: [
        TextFormField(onSaved: (v) => _draft.title = v!, validator: (v) => v == null || v.isEmpty ? 'Required' : null),
        TextFormField(onSaved: (v) => _draft.body = v!, validator: (v) => _check(v)),
        TextFormField(controller: _tagController),
      ]),
    );
  }
}
