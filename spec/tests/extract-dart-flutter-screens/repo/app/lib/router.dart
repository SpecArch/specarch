import 'package:go_router/go_router.dart';

import 'screens/notes.dart';
import 'screens/settings.dart';

final router = GoRouter(
  redirect: (context, state) => null,
  routes: [
    StatefulShellRoute.indexedStack(
      builder: (context, state, shell) => shell,
      branches: [
        StatefulShellBranch(routes: [
          GoRoute(
            path: '/notes',
            builder: (context, state) => const NotesScreen(),
            routes: [
              GoRoute(path: 'new', builder: (context, state) => const NoteFormScreen()),
              GoRoute(path: ':noteId', builder: (context, state) => NoteScreen(id: state.pathParameters['noteId']!)),
            ],
          ),
        ]),
        StatefulShellBranch(routes: [
          GoRoute(path: '/settings', pageBuilder: (context, state) => const MaterialPage(child: SettingsScreen())),
        ]),
      ],
    ),
    GoRoute(path: '/files/*', builder: (context, state) => const NotesScreen()),
    GoRoute(path: '/about', builder: (context, state) => const AboutBox()),
  ],
);
