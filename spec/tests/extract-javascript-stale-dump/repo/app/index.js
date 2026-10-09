import express from 'express';

export const app = express();
app.get('/notes', (_req, res) => res.json([]));
