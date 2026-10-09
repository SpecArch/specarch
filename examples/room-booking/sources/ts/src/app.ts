import express from 'express';
import { port } from './config';
import bookings from './routes/bookings';
import { rooms } from './routes/rooms';

const app = express();
app.use(express.json());
app.use('/api/bookings', bookings);
app.use('/api/rooms', rooms);

app.listen(port, () => {
  console.log(`listening on ${port}`);
});
