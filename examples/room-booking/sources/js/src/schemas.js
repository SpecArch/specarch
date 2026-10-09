import { z } from 'zod';

// What a person sends to book a room.
export const newBookingSchema = z.object({
  roomId: z.string().uuid(),
  startsAt: z.string().datetime(),
  minutes: z.number().int().min(15).max(240),
  purpose: z.string().max(200).optional(),
});
