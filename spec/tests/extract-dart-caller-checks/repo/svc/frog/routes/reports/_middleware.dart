import 'package:dart_frog/dart_frog.dart';

import '../../../lib/guard.dart';

Handler middleware(Handler handler) {
  requireAudience('reports-api');
  return handler;
}
