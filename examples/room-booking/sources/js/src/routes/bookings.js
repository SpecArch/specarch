import { Router } from 'express';
import { requirePermission } from '../auth.js';
import { publish } from '../calendar.js';
import { newBookingSchema } from '../schemas.js';

const router = Router();
const bookings = new Map();

router.get('/', requirePermission('bookings.read'), (_req, res) => {
  res.json([...bookings.values()]);
});

router.post('/', requirePermission('bookings.write'), async (req, res) => {
  const input = newBookingSchema.parse(req.body);
  const booking = { id: crypto.randomUUID(), status: 'held', ...input };
  bookings.set(booking.id, booking);
  res.status(201).json(booking);
});

/**
 * @param {import('express').Request<{ bookingId: string }, import('../types.js').Booking>} req
 * @param {import('express').Response} res
 */
function show(req, res) {
  const booking = bookings.get(req.params.bookingId);
  if (!booking) {
    res.sendStatus(404);
    return;
  }
  res.json(booking);
}

router.get('/:bookingId', requirePermission('bookings.read'), show);

router.post('/:bookingId/confirm', requirePermission('bookings.write'), async (req, res) => {
  const booking = bookings.get(req.params.bookingId);
  if (!booking) {
    res.sendStatus(404);
    return;
  }
  booking.status = 'confirmed';
  await publish(booking);
  res.json(booking);
});

router.post('/:bookingId/cancel', requirePermission('bookings.write'), (req, res) => {
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
