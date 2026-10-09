import { requirePermission } from '../../../../lib/auth';

export async function generateStaticParams() {
  return [{ loanId: 'l-1' }, { loanId: 'l-2' }];
}

export async function GET(_request: Request, { params }: { params: Promise<{ loanId: string }> }) {
  await requirePermission('loans.read');
  return Response.json({ id: (await params).loanId });
}

export async function PATCH() {
  await requirePermission(process.env.EDIT_PERMISSION ?? 'loans.edit');
  return Response.json({});
}
