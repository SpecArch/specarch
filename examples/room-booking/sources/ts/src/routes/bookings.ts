import { Router, type Request, type Response } from 'express';
import { requirePermission } from '../auth';
import { publish } from '../calendar';
import { newBookingSchema } from '../schemas';
import type { Booking, CancelRequest } from '../types';

const router = Router();
const bookings = new Map<string, Booking>();

router.get('/', requirePermission('bookings.read'), (_req: Request, res: Response<Booking[]>) => {
  res.json([...bookings.values()]);
});

router.post('/', requirePermission('bookings.write'), async (req: Request, res: Response<Booking>) => {
  const input = newBookingSchema.parse(req.body);
  const booking: Booking = { id: crypto.randomUUID(), status: 'held', ...input };
  bookings.set(booking.id, booking);
  res.status(201).json(booking);
});

router.get('/:bookingId', requirePermission('bookings.read'), (req: Request<{ bookingId: string }, Booking>, res: Response<Booking>) => {
  const booking = bookings.get(req.params.bookingId);
  if (!booking) {
    res.sendStatus(404);
    return;
  }
  res.json(booking);
});

router.post('/:bookingId/confirm', requirePermission('bookings.write'), async (req: Request<{ bookingId: string }, Booking>, res: Response<Booking>) => {
  const booking = bookings.get(req.params.bookingId);
  if (!booking) {
    res.sendStatus(404);
    return;
  }
  booking.status = 'confirmed';
  await publish(booking);
  res.json(booking);
});

router.post('/:bookingId/cancel', requirePermission('bookings.write'), (req: Request<{ bookingId: string }, Booking, CancelRequest>, res: Response<Booking>) => {
  const booking = bookings.get(req.params.bookingId);
  if (!booking) {
    res.sendStatus(404);
    return;
  }
  booking.status = 'cancelled';
  console.log(`cancelled ${booking.id}: ${req.body.reason}`);
  res.json(booking);
});

export default router;
