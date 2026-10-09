import 'package:dart_frog/dart_frog.dart';

import '../../../lib/auth.dart';

Response onRequest(RequestContext context, String id) {
  switch (context.request.method) {
    case HttpMethod.get:
      needs('notes.read');
      return Response(body: id);
    case HttpMethod.delete:
      return Response(statusCode: 204);
    default:
      return Response(statusCode: 405);
  }
}
