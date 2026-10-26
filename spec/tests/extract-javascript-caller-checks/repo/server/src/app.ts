import express from 'express';
import type { Request, Response } from 'express';
import { allow, audit, requireApplication, requireAudience, requireCaller, requireClient, requireClients } from './guard';

const app = express();
const admin = express.Router();
const callers = (process.env.VOID_CALLERS ?? '').split(',');

app.use(requireAudience(process.env.TOKEN_AUDIENCE ?? ''));
app.get('/invoices', requireClients(['reporting', 'archive']), allow('invoices.read'), listInvoices);
app.delete('/invoices/:invoiceId', requireCaller(['middleware']), requireApplication('admin-web'), audit, allow('invoices.delete'), deleteInvoice);
app.post('/invoices/:invoiceId/void', requireCaller(callers), allow('invoices.delete'), voidInvoice);
app.post('/invoices/:invoiceId/archive', allow('invoices.delete'), archiveInvoice);
app.post('/invoices/:invoiceId/send', requireClients(['reporting', 'mailer']), requireClient('mailer', 'only the mailer sends invoices'), allow('invoices.read'), sendInvoice);
admin.post('/invoices/:invoiceId/restore', allow('invoices.restore'), restoreInvoice);
app.use('/admin', requireClient('admin-console'), admin);

function listInvoices(_req: Request, res: Response) {
  res.json([]);
}

function deleteInvoice(_req: Request, res: Response) {
  res.sendStatus(204);
}

function voidInvoice(_req: Request, res: Response) {
  res.sendStatus(200);
}

function archiveInvoice(_req: Request, res: Response) {
  res.sendStatus(200);
}

function sendInvoice(_req: Request, res: Response) {
  res.sendStatus(200);
}

function restoreInvoice(_req: Request, res: Response) {
  res.sendStatus(200);
}

app.listen(3000);
