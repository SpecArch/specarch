// The events the screens raise, each name declared once with its payload.
// A component subscribes where it is created; no name is a wildcard or
// computed. Set DEBUG to true to log every event.

export const DEBUG = false;

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
