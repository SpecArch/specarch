import 'dart:io';

import 'package:shelf/shelf.dart';
import 'package:shelf_router/shelf_router.dart';

import 'auth.dart';

final api = Router()
  ..get('/notes', requirePermission('notes.read', _list))
  ..post('/notes', _create)
  ..get('/notes/<noteId>', (Request req, String noteId) => Response.ok(noteId))
  ..get('/files/<path|.*>', (Request req, String path) => Response.ok(path));

final app = Router()..mount('/api/', api.call);

Response _list(Request req) => Response.ok('[]');

Response _create(Request req) {
  requirePermission('notes.write', _list);
  return Response.ok('created');
}

final port = int.fromEnvironment('PORT', defaultValue: 8080);
const debug = bool.fromEnvironment('DEBUG');
final secret = Platform.environment['NOTES_SECRET'];
