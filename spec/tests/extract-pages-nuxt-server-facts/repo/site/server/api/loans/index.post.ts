import { requirePermission } from '../../../utils/auth';
import { newLoanSchema } from '../../../utils/schemas';

export default defineEventHandler(async (event) => {
  requirePermission('loans.write');
  const loan = await readValidatedBody(event, newLoanSchema.parse);
  return loan;
});
