// The events the screens raise, each name declared once with its payload.
// A component subscribes where it is created; no name is a wildcard or
// computed. Set DEBUG to true to log every event.

export const DEBUG = false;

/** The filters of the fee waivers to approve list were applied: {} */
export const FEE_WAIVERS_INBOX_FILTERED = "FEE_WAIVERS_INBOX_FILTERED";
/** Another page of the fee waivers to approve list was asked for: { page } */
export const FEE_WAIVERS_INBOX_PAGED = "FEE_WAIVERS_INBOX_PAGED";
/** A page of fee waivers to approve arrived: { items, totalItems, totalPages, page, filtered } */
export const FEE_WAIVERS_INBOX_LOADED = "FEE_WAIVERS_INBOX_LOADED";
/** The fee waivers to approve could not be read: { status } */
export const FEE_WAIVERS_INBOX_FAILED = "FEE_WAIVERS_INBOX_FAILED";
/** approveFeeWaiver succeeded: { id } */
export const APPROVE_FEE_WAIVER_SUCCEEDED = "APPROVE_FEE_WAIVER_SUCCEEDED";
/** approveFeeWaiver was refused or failed: { id, status } */
export const APPROVE_FEE_WAIVER_FAILED = "APPROVE_FEE_WAIVER_FAILED";
/** The filters of the loans list were applied: { status, memberId } */
export const LOANS_LIST_FILTERED = "LOANS_LIST_FILTERED";
/** Another page of the loans list was asked for: { page } */
export const LOANS_LIST_PAGED = "LOANS_LIST_PAGED";
/** A page of loans arrived: { items, totalItems, totalPages, page, filtered } */
export const LOANS_LIST_LOADED = "LOANS_LIST_LOADED";
/** The loans could not be read: { status } */
export const LOANS_LIST_FAILED = "LOANS_LIST_FAILED";
/** returnLoan succeeded: { id } */
export const RETURN_LOAN_SUCCEEDED = "RETURN_LOAN_SUCCEEDED";
/** returnLoan was refused or failed: { id, status } */
export const RETURN_LOAN_FAILED = "RETURN_LOAN_FAILED";
/** reportLost succeeded: { id } */
export const REPORT_LOST_SUCCEEDED = "REPORT_LOST_SUCCEEDED";
/** reportLost was refused or failed: { id, status } */
export const REPORT_LOST_FAILED = "REPORT_LOST_FAILED";
/** The filters of the members list were applied: { tier } */
export const MEMBERS_LIST_FILTERED = "MEMBERS_LIST_FILTERED";
/** Another page of the members list was asked for: { page } */
export const MEMBERS_LIST_PAGED = "MEMBERS_LIST_PAGED";
/** A page of members arrived: { items, totalItems, totalPages, page, filtered } */
export const MEMBERS_LIST_LOADED = "MEMBERS_LIST_LOADED";
/** The members could not be read: { status } */
export const MEMBERS_LIST_FAILED = "MEMBERS_LIST_FAILED";

/** A member arrived: { record } */
export const MEMBER_VIEW_LOADED = "MEMBER_VIEW_LOADED";
/** The member could not be read: { status } */
export const MEMBER_VIEW_FAILED = "MEMBER_VIEW_FAILED";
/** Register member was sent and succeeded: { status, answer } */
export const MEMBER_FORM_SUBMITTED = "MEMBER_FORM_SUBMITTED";
/** Register member was refused or failed: { status } */
export const MEMBER_FORM_REFUSED = "MEMBER_FORM_REFUSED";

const handlers = new Map();

/** Calls handler with the payload of every event of that name. */
export function on(name, handler) {
  if (!handlers.has(name)) {
    handlers.set(name, []);
  }
  handlers.get(name).push(handler);
}

/** Raises an event. */
export function emit(name, payload) {
  if (DEBUG) {
    console.log(name, payload);
  }
  for (const handler of handlers.get(name) ?? []) {
    handler(payload);
  }
}
