import 'dart:io';

import 'package:shelf/shelf.dart';
import 'package:shelf_router/shelf_router.dart';

final port = Platform.environment['PORT'] ?? '8080';

final api = Router()
  ..get('/invoices', _list)
  ..post('/invoices/<invoiceId>/void', _void)
  ..post('/invoices/<invoiceId>/send', (Request req, String invoiceId) {
    if (req.headers['Txn-Token'] == null) return Response(401);
    return Response.ok(invoiceId);
  });

Response _list(Request req) => Response.ok(req.headers['accept-language'] ?? '');

Response _void(Request req, String invoiceId) {
  if (req.headers['txn-token'] == null) return Response(401);
  return Response.ok(invoiceId);
}
