import { calendarToken } from './config';
import type { Booking } from './types';

// publish puts a confirmed booking on the shared calendar.
export async function publish(booking: Booking): Promise<void> {
  await fetch('https://calendar.example.org/v1/events', {
    method: 'POST',
    headers: { authorization: `Bearer ${calendarToken}` },
    body: JSON.stringify({ id: booking.id, start: booking.startsAt, minutes: booking.minutes }),
  });
}
