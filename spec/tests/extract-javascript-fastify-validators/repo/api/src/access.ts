import type { FastifyReply, FastifyRequest } from 'fastify';

export function needs(permission: string) {
  return async (request: FastifyRequest, reply: FastifyReply) => {
    if (!(request as any).user?.can(permission)) {
      await reply.code(403).send();
    }
  };
}
