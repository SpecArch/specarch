import 'package:dart_frog/dart_frog.dart';

Response onRequest(RequestContext context) {
  if (context.request.method == HttpMethod.get) {
    return Response(body: '[]');
  }
  if (context.request.method == HttpMethod.post) {
    if (context.request.headers['txn-token'] == null) return Response(statusCode: 401);
    return Response(statusCode: 201);
  }
  return Response(statusCode: 405);
}
