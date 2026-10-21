import express from 'express';
import type { Request, Response } from 'express';

const app = express();

app.get('/invoices', listInvoices);
app.post('/invoices/:invoiceId/void', voidInvoice);
app.post('/invoices/:invoiceId/send', (req: Request, res: Response) => {
  res.sendStatus(req.headers['Txn-Token'] ? 200 : 401);
});

function listInvoices(req: Request, res: Response) {
  res.json({ language: req.headers['accept-language'] });
}

function voidInvoice(req: Request, res: Response) {
  if (!req.headers['txn-token']) {
    res.sendStatus(401);
    return;
  }
  res.sendStatus(200);
}

app.listen(3000);
