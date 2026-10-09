import { Router } from 'express';

export const rooms = Router();

const all = [
  { id: '0b6f8e9e-1c1e-4d38-9a51-4c1f1e5f6a01', name: 'Harbour', seats: 8 },
  { id: '0b6f8e9e-1c1e-4d38-9a51-4c1f1e5f6a02', name: 'Garden', seats: 4 },
];

rooms.get('/', (_req, res) => {
  res.json(all);
});

rooms.get('/:roomId', (req, res) => {
  const room = all.find((r) => r.id === req.params.roomId);
  if (!room) {
    res.sendStatus(404);
    return;
  }
  res.json(room);
});
