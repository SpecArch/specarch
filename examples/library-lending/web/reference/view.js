// Draws a view page from what its module says of it: the record its
// operation reads, shown read-only by sections, and its actions, each a
// link to another page or an operation run on the record once the person
// confirms it. Every view page uses it.

import { on, emit } from "./events.js";
import { arrived, go, href, parameters } from "./pages.js";
import { ask } from "./ask.js";

/** A value as the page shows it: a value with parts as a list of them,
    a list as one entry per item. */
function shown(value, field) {
  if (field.list) {
    const items = document.createElement("ul");
    for (const item of value ?? []) {
      const entry = document.createElement("li");
      entry.append(field.parts ? parts(item, field.parts) : String(item ?? ""));
      items.append(entry);
    }
    return items;
  }
  if (field.parts) {
    return parts(value ?? {}, field.parts);
  }
  return document.createTextNode(String(value ?? ""));
}

/** The parts of a value, each with its label. */
function parts(value, fields) {
  const list = document.createElement("dl");
  for (const field of fields) {
    const term = document.createElement("dt");
    term.textContent = field.label;
    const detail = document.createElement("dd");
    detail.append(shown(value[field.name], field));
    list.append(term, detail);
  }
  return list;
}

/** Draws the view, reads its record and runs its actions. */
export function view(page) {
  const given = parameters();
  const record = document.getElementById("record");
  const bar = document.getElementById("actions");
  const line = document.getElementById("status");
  let current;

  function refusal(operation, status) {
    return page.refusals[operation][String(status)] ?? page.messages.failed;
  }

  function show(text, failed) {
    line.textContent = text;
    line.classList.toggle("failed", failed);
  }

  async function load() {
    try {
      const response = await fetch(page.source.path(given), { headers: { Accept: "application/json" } });
      if (!response.ok) {
        emit(page.failed, { status: response.status });
        return;
      }
      emit(page.loaded, { record: await response.json() });
    } catch {
      emit(page.failed, { status: 0 });
    }
  }

  async function run(action, button) {
    const reason = await ask(action.confirm, action.reason);
    if (reason === null) {
      return;
    }
    const init = { method: action.method, headers: { Accept: "application/json" } };
    if (action.reason) {
      init.headers["Content-Type"] = "application/json";
      init.body = JSON.stringify({ [action.reason.name]: reason });
    }
    button.disabled = true;
    try {
      const response = await fetch(action.path(current), init);
      button.disabled = false;
      if (response.ok) {
        emit(action.succeeded, { id: current[page.key] });
      } else {
        emit(action.failed, { id: current[page.key], status: response.status });
      }
    } catch {
      button.disabled = false;
      emit(action.failed, { id: current[page.key], status: 0 });
    }
  }

  on(page.loaded, (payload) => {
    current = payload.record;
    record.replaceChildren();
    page.sections.forEach((section, i) => {
      const part = document.createElement("section");
      if (section.title) {
        const heading = document.createElement("h2");
        heading.id = `section-${i}`;
        heading.textContent = section.title;
        part.setAttribute("aria-labelledby", heading.id);
        part.append(heading);
      }
      part.append(parts(current, section.fields.filter((field) => !field.hiddenWhen?.(current))));
      record.append(part);
    });
    bar.replaceChildren();
    for (const action of page.actions) {
      if (action.when && !action.when(current)) {
        continue;
      }
      if (action.page) {
        const link = document.createElement("a");
        link.href = href(action.page, action.with(current, given));
        link.textContent = action.label;
        bar.append(link);
        continue;
      }
      const button = document.createElement("button");
      button.type = "button";
      button.textContent = action.label;
      button.addEventListener("click", () => run(action, button));
      bar.append(button);
    }
  });
  on(page.failed, ({ status }) => show(refusal(page.source.operation, status), true));
  for (const action of page.actions.filter((action) => action.operation)) {
    on(action.succeeded, () => {
      if (action.navigate) {
        go(action.navigate.page, action.navigate.with(current, given), action.message);
        return;
      }
      show(action.message, false);
      load();
    });
    on(action.failed, ({ status }) => show(refusal(action.operation, status), true));
  }
  show(arrived(), false);
  load();
}
