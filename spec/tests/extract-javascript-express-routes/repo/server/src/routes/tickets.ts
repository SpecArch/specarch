import { Router, type Request, type Response } from 'express';
import { allow } from '../guard';
import type { NewReply, Ticket } from '../types';

export const tickets = Router();
tickets.use(allow('tickets.read'));

function list(_req: Request, res: Response<Ticket[]>) {
  res.json([]);
}

tickets.get('/', list);
tickets.get('/:ticketId', (req: Request<{ ticketId: string }, Ticket>, res: Response<Ticket>) => {
  res.json({ id: req.params.ticketId } as Ticket);
});
tickets.post('/:ticketId/replies', allow('tickets.answer'), (req: Request<{ ticketId: string }, Ticket, NewReply>, res: Response) => {
  res.status(201).json({ id: req.params.ticketNumber });
});
tickets.route('/:ticketId/state').put((req: Request<{ ticketId: string }, Ticket, { state: string }>, res: Response) => {
  res.json({});
});
tickets.delete('/:ticketId', allow(process.env.CLOSE_PERMISSION ?? 'tickets.close'), (_req: Request, res: Response) => {
  res.sendStatus(204);
});
