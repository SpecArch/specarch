import { withPermission } from '../../../src/auth';

async function member(req, res) {
  if (req.method === 'DELETE') {
    return res.status(204).end();
  }
  return res.json({ id: req.query.memberId });
}

export default withPermission('members.admin', member);
