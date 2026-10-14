export type TicketState = 'open' | 'answered' | 'closed';

export interface Ticket {
  id: string;
  subject: string;
  state: TicketState;
  opened: Date;
  tags: string[];
  assignee: string | null;
  priority?: number;
  extra: Record<string, string>;
  meta: Record<string, unknown>;
}

export interface NewReply {
  text: string;
  internal?: boolean;
}
