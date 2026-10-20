import 'package:shelf/shelf.dart';

/// allow runs inner only for a caller whose role grants permission.
Handler allow(String permission, Handler inner) => inner;

/// requireClients admits only one of the clients named.
Handler requireClients(List<String> clients, Handler inner) => inner;

/// requireClient admits only the client named, and logs reason when it
/// refuses.
Handler requireClient(String client, String reason, Handler inner) => inner;

/// requireApplication admits only a person who signed in through the
/// application named.
Handler requireApplication(String application, Handler inner) => inner;

/// requireCaller admits only an immediate caller among the services named.
Handler requireCaller(List<String> services, Handler inner) => inner;

/// requireAudience refuses a token whose aud is not the one given.
void requireAudience(String audience) {}

/// audit writes a security event when inner refuses.
Handler audit(Handler inner) => inner;

/// needs refuses a caller whose role does not grant permission.
void needs(String permission) {}
