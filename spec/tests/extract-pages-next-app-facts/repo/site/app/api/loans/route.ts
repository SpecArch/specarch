import { withPermission } from '../../../lib/auth';
import { newLoanSchema } from '../../../lib/schemas';

export async function GET() {
  return Response.json([]);
}

export const POST = withPermission('loans.write', async (request: Request) => {
  const loan = newLoanSchema.parse(await request.json());
  return Response.json(loan, { status: 201 });
});

async function remove() {
  return new Response(null, { status: 204 });
}

export { remove as DELETE };

export function HEAD() {
  return new Response(null);
}
