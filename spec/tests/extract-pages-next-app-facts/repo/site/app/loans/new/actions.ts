'use server';

import { requirePermission } from '../../../lib/auth';

export async function createLoan(form: FormData): Promise<void> {
  await requirePermission('loans.write');
  console.log(form.get('bookId'));
}
