import { requirePermission } from '../../../src/auth';

export default async function handler(req, res) {
  switch (req.method) {
    case 'GET':
      await requirePermission('members.read');
      return res.json([]);
    case 'POST':
      await requirePermission('members.write');
      return res.status(201).json(req.body);
    default:
      return res.status(405).end();
  }
}
