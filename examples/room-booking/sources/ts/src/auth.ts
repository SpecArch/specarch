import type { NextFunction, Request, Response } from 'express';

// requirePermission lets a request through when its session holds the
// permission, and answers 403 otherwise.
export function requirePermission(permission: string) {
  return (req: Request, res: Response, next: NextFunction) => {
    const granted: string[] = (req as any).session?.permissions ?? [];
    if (!granted.includes(permission)) {
      res.status(403).json({ error: 'forbidden' });
      return;
    }
    next();
  };
}
