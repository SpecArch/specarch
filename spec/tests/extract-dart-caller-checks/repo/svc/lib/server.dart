import 'dart:io';

import 'package:shelf/shelf.dart';
import 'package:shelf_router/shelf_router.dart';

import 'guard.dart';

final callers = (Platform.environment['VOID_CALLERS'] ?? '').split(',');

final api = Router()
  ..get('/invoices', requireClients(['reporting', 'archive'], allow('invoices.read', _list)))
  ..delete('/invoices/<invoiceId>', requireCaller(['middleware'], requireApplication('admin-web', audit(allow('invoices.delete', _delete)))))
  ..post('/invoices/<invoiceId>/void', requireCaller(callers, allow('invoices.delete', _void)))
  ..post('/invoices/<invoiceId>/archive', allow('invoices.delete', _archive))
  ..post('/invoices/<invoiceId>/send', requireClients(['reporting', 'mailer'], requireClient('mailer', 'only the mailer sends invoices', allow('invoices.read', _send))));

Response _list(Request req) => Response.ok('[]');

Response _delete(Request req, String invoiceId) => Response(204);

Response _void(Request req, String invoiceId) => Response.ok(invoiceId);

Response _archive(Request req, String invoiceId) => Response.ok(invoiceId);

Response _send(Request req, String invoiceId) => Response.ok(invoiceId);
