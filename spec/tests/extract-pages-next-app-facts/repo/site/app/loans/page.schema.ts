import { LOANS_TITLE } from '../../lib/constants';

const COLUMNS = ['book', 'member', 'due'];

export default {
  kind: 'list',
  title: LOANS_TITLE,
  entity: 'Loan',
  columns: COLUMNS,
  source: 'getApiLoans',
} satisfies Record<string, unknown>;
