import { requirePermission } from '../utils/auth';

export default defineNuxtRouteMiddleware(() => {
  requirePermission('loans.read');
});
