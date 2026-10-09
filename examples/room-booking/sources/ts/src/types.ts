// The state of a booking.
export type BookingStatus = 'held' | 'confirmed' | 'cancelled';

// A booking as the service answers it.
export interface Booking {
  id: string;
  roomId: string;
  startsAt: string;
  minutes: number;
  status: BookingStatus;
  purpose?: string;
}

// A room people can book.
export interface Room {
  id: string;
  name: string;
  seats: number;
}

// Why a booking is cancelled.
export interface CancelRequest {
  reason: string;
}
