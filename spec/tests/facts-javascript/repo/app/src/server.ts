import express from 'express';
import { allow } from '@/guard';

const app = express();
const port = Number(process.env.PORT ?? '8080');

app.get('/shelves', allow('shelves.read'), (req, res) => {
  res.json([]);
});

app.listen(port);
