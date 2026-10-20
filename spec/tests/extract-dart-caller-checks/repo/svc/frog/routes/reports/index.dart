import 'package:dart_frog/dart_frog.dart';

import '../../../lib/guard.dart';

Response onRequest(RequestContext context) {
  if (context.request.method == HttpMethod.get) {
    needs('reports.read');
    return Response(body: '[]');
  }
  return Response(statusCode: 405);
}
