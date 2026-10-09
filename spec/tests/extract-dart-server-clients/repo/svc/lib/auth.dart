import 'package:shelf/shelf.dart';

Handler requirePermission(String permission, Handler inner) => inner;

void needs(String permission) {}
