import { Router } from 'express';

const unused = Router();
unused.get('/never', (_req, res) => res.send('never'));
