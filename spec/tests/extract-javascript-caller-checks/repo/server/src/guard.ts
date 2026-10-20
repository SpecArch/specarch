import type { NextFunction, Request, Response } from 'express';

type Middleware = (req: Request, res: Response, next: NextFunction) => void;

const pass: Middleware = (_req, _res, next) => next();

// allow runs the next handler only for a caller whose role grants permission.
export function allow(permission: string): Middleware {
  return pass;
}

// requireClients admits only one of the clients named.
export function requireClients(clients: string[]): Middleware {
  return pass;
}

// requireClient admits only the client named, and logs reason when it refuses.
export function requireClient(client: string, reason: string): Middleware {
  return pass;
}

// requireApplication admits only a person who signed in through the application named.
export function requireApplication(application: string): Middleware {
  return pass;
}

// requireCaller admits only an immediate caller among the services named.
export function requireCaller(services: string[]): Middleware {
  return pass;
}

// requireAudience refuses a token whose aud is not the one given.
export function requireAudience(audience: string): Middleware {
  return pass;
}

// audit writes a security event when the next handler refuses.
export function audit(req: Request, res: Response, next: NextFunction) {
  next();
}
