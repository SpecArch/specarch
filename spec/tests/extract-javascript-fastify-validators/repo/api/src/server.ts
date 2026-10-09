import Fastify from 'fastify';
import { notesRoutes } from './notes';

const app = Fastify({ logger: true });
app.register(notesRoutes, { prefix: '/notes' });
app.get('/status', async () => ({ ok: true }));
app.listen({ port: 8080 });
