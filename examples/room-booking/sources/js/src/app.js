import express from 'express';
import { port } from './config.js';
import bookings from './routes/bookings.js';
import { rooms } from './routes/rooms.js';

const app = express();
app.use(express.json());
app.use('/api/bookings', bookings);
app.use('/api/rooms', rooms);

app.listen(port, () => {
  console.log(`listening on ${port}`);
});
