import { z } from 'zod';

export const newLoanSchema = z.object({
  bookId: z.string().uuid(),
  days: z.number().int().min(1).max(28),
});
