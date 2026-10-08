// The page loans-list: Loans. It lists loans through listLoans, a page
// at a time, and runs returnLoan and reportLost on a row.

import {
  on, emit,
  LOANS_LIST_FILTERED, LOANS_LIST_PAGED, LOANS_LIST_LOADED, LOANS_LIST_FAILED,
  RETURN_LOAN_SUCCEEDED, RETURN_LOAN_FAILED,
  REPORT_LOST_SUCCEEDED, REPORT_LOST_FAILED,
} from "./events.js";

const columns = ["memberId", "bookId", "loanedAt", "dueOn", "status", "lateFee"];
const pageSize = 20;

const messages = {
  empty: "No loans yet. A loan is made from a member's record.",
  filteredEmpty: "No loan matches these filters.",
  failed: "The loans cannot be shown or changed right now. Try again in a moment.",
};

// What each operation's refusals say, by status: the page's failed states.
const refusals = {
  listLoans: {},
  returnLoan: {
    "409": "This loan was already closed, so nothing changed.",
  },
  reportLost: {
    "409": "This loan was already closed, so nothing changed.",
  },
};

const actions = [
  {
    label: "Return",
    operation: "returnLoan",
    method: "POST",
    path: (row) => `/loans/${encodeURIComponent(row.id)}/return`,
    confirm: "Record this copy as returned?",
    message: "The copy is recorded as returned.",
    succeeded: RETURN_LOAN_SUCCEEDED,
    failed: RETURN_LOAN_FAILED,
  },
  {
    label: "Lost",
    operation: "reportLost",
    method: "POST",
    path: (row) => `/loans/${encodeURIComponent(row.id)}/lost`,
    confirm: "Charge the replacement cost and close this loan?",
    message: "The loan is closed and the replacement cost charged.",
    succeeded: REPORT_LOST_SUCCEEDED,
    failed: REPORT_LOST_FAILED,
  },
];

/** The message of a refusal of an operation, or the page's default. */
function refusal(operation, status) {
  return refusals[operation][String(status)] ?? messages.failed;
}

/** Reads the list: the filters given, then the page. */
function loader() {
  let filters = {};
  let page = 1;
  async function load() {
    const query = new URLSearchParams();
    for (const [name, value] of Object.entries(filters)) {
      if (value !== "") {
        query.set(name, value);
      }
    }
    const filtered = [...query.keys()].length > 0;
    query.set("page", String(page));
    query.set("pageSize", String(pageSize));
    try {
      const response = await fetch(`/loans?${query}`, { headers: { Accept: "application/json" } });
      if (!response.ok) {
        emit(LOANS_LIST_FAILED, { status: response.status });
        return;
      }
      const body = await response.json();
      emit(LOANS_LIST_LOADED, { items: body.items, totalItems: body.totalItems, totalPages: body.totalPages, page, filtered });
    } catch {
      emit(LOANS_LIST_FAILED, { status: 0 });
    }
  }
  on(LOANS_LIST_FILTERED, (payload) => {
    filters = payload;
    page = 1;
    load();
  });
  on(LOANS_LIST_PAGED, (payload) => {
    page = payload.page;
    load();
  });
  for (const action of actions) {
    on(action.succeeded, () => load());
  }
  load();
}

/** The filters form. */
function filterForm() {
  const form = document.getElementById("filters");
  form.addEventListener("submit", (event) => {
    event.preventDefault();
    emit(LOANS_LIST_FILTERED, Object.fromEntries(new FormData(form)));
  });
}

/** Runs an action on a row, once the person confirms it. */
async function run(action, row) {
  if (!window.confirm(action.confirm)) {
    return;
  }
  try {
    const response = await fetch(action.path(row), { method: action.method, headers: { Accept: "application/json" } });
    if (response.ok) {
      emit(action.succeeded, { id: row.id });
    } else {
      emit(action.failed, { id: row.id, status: response.status });
    }
  } catch {
    emit(action.failed, { id: row.id, status: 0 });
  }
}

/** The table of loans. */
function table() {
  const body = document.querySelector("#rows tbody");
  const head = document.querySelectorAll("#rows thead th");
  on(LOANS_LIST_LOADED, ({ items }) => {
    body.replaceChildren();
    for (const row of items) {
      const tr = document.createElement("tr");
      columns.forEach((name, i) => {
        const td = document.createElement("td");
        td.className = head[i].className;
        td.textContent = row[name] ?? "";
        tr.append(td);
      });
      const cell = document.createElement("td");
      for (const action of actions) {
        const button = document.createElement("button");
        button.type = "button";
        button.textContent = action.label;
        button.addEventListener("click", () => run(action, row));
        cell.append(button);
      }
      tr.append(cell);
      body.append(tr);
    }
  });
}

/** The status line: states and the messages of events, announced without moving focus. */
function statusLine() {
  const line = document.getElementById("status");
  // An event's message stays until the person filters or pages; a state
  // gives way to the list it describes.
  let kept = false;
  function show(text, failed, keep) {
    line.textContent = text;
    line.classList.toggle("failed", failed);
    kept = keep;
  }
  on(LOANS_LIST_FILTERED, () => show("", false, false));
  on(LOANS_LIST_PAGED, () => show("", false, false));
  on(LOANS_LIST_LOADED, ({ items, filtered }) => {
    if (items.length === 0) {
      show(filtered ? messages.filteredEmpty : messages.empty, false, false);
    } else if (!kept) {
      show("", false, false);
    }
  });
  on(LOANS_LIST_FAILED, ({ status }) => show(refusal("listLoans", status), true, false));
  for (const action of actions) {
    on(action.succeeded, () => show(action.message, false, true));
    on(action.failed, ({ status }) => show(refusal(action.operation, status), true, true));
  }
}

/** The pager. */
function pager() {
  const nav = document.getElementById("pager");
  const where = nav.querySelector("span");
  const [previous, next] = nav.querySelectorAll("button");
  let page = 1;
  on(LOANS_LIST_LOADED, (payload) => {
    page = payload.page;
    where.textContent = `Page ${payload.page} of ${Math.max(payload.totalPages, 1)}`;
    previous.disabled = payload.page <= 1;
    next.disabled = payload.page >= payload.totalPages;
  });
  for (const button of [previous, next]) {
    button.addEventListener("click", () => emit(LOANS_LIST_PAGED, { page: page + Number(button.dataset.step) }));
  }
}

filterForm();
table();
statusLine();
pager();
loader();
