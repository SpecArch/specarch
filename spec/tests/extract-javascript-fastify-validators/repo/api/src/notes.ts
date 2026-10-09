import type { FastifyInstance } from 'fastify';
import { needs } from './access';
import { commentSchema, noteSchema } from './schemas';

const newNoteBody = {
  type: 'object',
  required: ['title'],
  properties: {
    title: { type: 'string', maxLength: 120 },
    pinned: { type: 'boolean', default: false },
    colour: { type: 'string', enum: ['plain', 'yellow', 'blue'] },
    weight: { type: 'integer', minimum: 0, maximum: 10 },
  },
};

interface NoteParams {
  noteId: string;
}

export async function notesRoutes(fastify: FastifyInstance) {
  fastify.get('/', { preHandler: needs('notes.read') }, async () => []);
  fastify.post('/', { schema: { body: newNoteBody }, preHandler: [needs('notes.write')] }, async (request) => request.body);
  fastify.route({
    method: 'PUT',
    url: '/:noteId',
    preHandler: needs('notes.write'),
    handler: async (request) => noteSchema.validate(request.body),
  });
  fastify.post<{ Params: NoteParams; Body: { text: string } }>('/:noteId/comments', async (request) => {
    const { error } = commentSchema.validate(request.body);
    return { error, id: request.params.noteId };
  });
  fastify.route({
    method: 'PATCH',
    url: '/:noteId',
    schema: { body: { type: 'object', properties: { title: { type: 'string' } } } },
    handler: async () => ({}),
  });
}
