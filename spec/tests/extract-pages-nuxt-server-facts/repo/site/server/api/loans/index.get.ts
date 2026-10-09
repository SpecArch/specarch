import { requirePermission } from '../../../utils/auth';

export default defineEventHandler(() => {
  requirePermission('loans.read');
  return [];
});
