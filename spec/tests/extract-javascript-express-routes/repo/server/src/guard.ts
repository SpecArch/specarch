import type { NextFunction, Request, Response } from 'express';

export function allow(permission: string) {
  return (req: Request, res: Response, next: NextFunction) => {
    if ((req as any).user?.can(permission)) {
      next();
    } else {
      res.sendStatus(403);
    }
  };
}
