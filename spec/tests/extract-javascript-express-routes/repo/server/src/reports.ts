import type { Express } from 'express';

export function reports(app: Express) {
  app.get('/reports/daily', (_req, res) => {
    res.json([]);
  });
}
