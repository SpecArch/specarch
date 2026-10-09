import express from 'express';
import { tickets } from './routes/tickets';
import { reports } from './reports';

const app = express();
const api = express.Router();
const methods = ['get', 'post'];

app.use('/api', api);
api.use('/tickets', tickets);
api.get('/health', (_req, res) => res.send('ok'));
app.get('/files/*', (_req, res) => res.sendStatus(404));
app.get('/users/:userId?', (_req, res) => res.sendStatus(404));
app.all('/ping', (_req, res) => res.send('pong'));
app.get('/api/health', (_req, res) => res.send('ok'));

for (const m of methods) {
  app.get(`/loop/${m}`, (_req, res) => res.send(m));
}
if (process.env.DEBUG_ROUTES === 'true') {
  app.get('/debug', (_req, res) => res.send('debug'));
}
const base = '/v' + 2;
app.get(base + '/status', (_req, res) => res.send('ok'));

reports(app);
app.listen(Number(process.env.PORT ?? '3000'));
