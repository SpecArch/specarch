// Runs a form page from what its module says of it. The fields are in the
// page's HTML, checked by the browser from the JSON Schema keywords; this
// module adds what HTML cannot say: a pattern as JSON Schema reads it,
// checks across fields, a field entered twice, fields read-only or hidden
// by the record's values, pickers that read their records, the parts of a
// value and rows added and removed. It loads the record a form changes,
// sends the fields its operation takes, and leads on once it succeeds.
// Every form and task page uses it.

import { on, emit } from "./events.js";
import { arrived, go, parameters } from "./pages.js";

// What holds fields: the form, a value with parts, a list of rows and a row.
const HOLDERS = "form, [data-value], [data-list], [data-row]";

/** The elements under a holder that are its own, not a holder's within it. */
function own(holder, selector) {
  return [...holder.querySelectorAll(selector)].filter((element) => element.parentElement.closest(HOLDERS) === holder);
}

/** A control's value: null when empty, a number or a boolean by its type. */
function valueOf(control) {
  if (control.type === "checkbox") {
    return control.checked;
  }
  if (control.value === "") {
    return null;
  }
  return control.type === "number" ? Number(control.value) : control.value;
}

/** The record a holder holds, an empty field as null, a field hidden
    left out. */
function read(holder) {
  const record = {};
  for (const control of own(holder, "[name]")) {
    if (!control.closest("[hidden]")) {
      record[control.name] = valueOf(control);
    }
  }
  for (const value of own(holder, "[data-value]")) {
    record[value.dataset.value] = read(value);
  }
  for (const list of own(holder, "[data-list]")) {
    record[list.dataset.list] = own(list, "[data-row]").filter((row) => !row.dataset.loaded).map(read);
  }
  return record;
}

/** A value as it is sent: nulls, empty values and empty lists left out. */
function sent(value) {
  if (Array.isArray(value)) {
    const items = value.map(sent).filter((item) => item !== undefined);
    return items.length === 0 ? undefined : items;
  }
  if (value !== null && typeof value === "object") {
    const entries = Object.entries(value).map(([name, part]) => [name, sent(part)]).filter(([, part]) => part !== undefined);
    return entries.length === 0 ? undefined : Object.fromEntries(entries);
  }
  return value ?? undefined;
}

/** Puts a record's values into a holder's fields. */
function fill(holder, record) {
  for (const control of own(holder, "[name]")) {
    const value = record?.[control.name];
    if (control.type === "checkbox") {
      control.checked = value === true;
    } else {
      control.value = value ?? "";
    }
  }
  for (const value of own(holder, "[data-value]")) {
    fill(value, record?.[value.dataset.value]);
  }
}

/** A list of rows: added and removed by the person up to its maximum. */
function rows(list) {
  const holder = own(list, "[data-rows]")[0];
  const template = list.querySelector("template");
  const add = list.querySelector("[data-add]");
  const maximum = list.dataset.maximum === undefined ? Infinity : Number(list.dataset.maximum);
  const legend = list.querySelector("legend").textContent;
  function renumber() {
    own(list, "[data-row]").forEach((row, i) => {
      row.querySelector("legend").textContent = `${legend} ${i + 1}`;
      const remove = row.querySelector("[data-remove]");
      if (remove) {
        remove.setAttribute("aria-label", `Remove ${legend} ${i + 1}`);
      }
    });
    add.disabled = own(list, "[data-row]").length >= maximum;
  }
  function append(record, loaded) {
    const row = template.content.firstElementChild.cloneNode(true);
    holder.append(row);
    fill(row, record);
    if (loaded) {
      row.dataset.loaded = "true";
      row.disabled = true;
      row.querySelector("[data-remove]")?.remove();
    } else {
      row.querySelector("[data-remove]").addEventListener("click", () => {
        row.remove();
        renumber();
        add.focus();
      });
    }
    renumber();
    return row;
  }
  add.addEventListener("click", () => append(undefined, false).querySelector("[name]")?.focus());
  renumber();
  return append;
}

/** Runs the form: its rules, its pickers, the record it loads and what it
    sends. */
export function form(page) {
  const given = parameters();
  const element = document.getElementById("record");
  const button = element.querySelector('button[type="submit"]');
  const line = document.getElementById("status");
  const field = (name) => own(element, `[name="${name}"]`)[0];
  const lists = new Map(own(element, "[data-list]").map((list) => [list.dataset.list, rows(list)]));
  let key = crypto.randomUUID();
  let record = {};
  let loaded = {};

  function show(text, failed) {
    line.textContent = text;
    line.classList.toggle("failed", failed);
  }

  // A part of a value is required once any part of it is given.
  function required() {
    for (const value of element.querySelectorAll("[data-value]")) {
      const given = [...value.querySelectorAll("[name]")].some((control) => valueOf(control) !== null && valueOf(control) !== false);
      for (const control of own(value, "[data-required]")) {
        control.required = given;
      }
    }
  }

  // A field drawn read-only stays so whatever its conditions say.
  const fixed = new Map(Object.keys(page.conditions).map((name) => [name, field(name).readOnly || field(name).disabled]));

  // The rules HTML cannot say, as each field's own message.
  function check() {
    // The conditions read the record as entered; the checks and what is
    // sent read it again once a field they hide is left out.
    record = read(element);
    for (const [name, rule] of Object.entries(page.conditions)) {
      const control = field(name);
      const hidden = rule.hiddenWhen?.(record) ?? false;
      const locked = fixed.get(name) || (rule.readOnlyWhen?.(record) ?? false);
      // A hidden field is disabled too, so the browser does not ask for it.
      // A list or a box has no read-only state of its own; it is disabled.
      const lockable = "readOnly" in control && control.type !== "checkbox";
      control.closest("label").hidden = hidden;
      control.disabled = hidden || (locked && !lockable);
      if (lockable) {
        control.readOnly = locked;
      }
      // A second entry has a label of its own, and is not asked for while
      // its field is hidden or locked.
      for (const again of element.querySelectorAll(`[data-again="${name}"]`)) {
        again.closest("label").hidden = hidden;
        again.disabled = hidden || locked;
      }
    }
    record = read(element);
    required();
    // Each control's message is its first rule broken, set once.
    const messages = new Map();
    const fail = (control, message) => {
      if (!messages.get(control)) {
        messages.set(control, message);
      }
    };
    for (const control of element.querySelectorAll("[data-pattern]")) {
      const wrong = control.value !== "" && !new RegExp(control.dataset.pattern, "u").test(control.value);
      fail(control, wrong ? page.messages.pattern : "");
    }
    for (const again of element.querySelectorAll("[data-again]")) {
      fail(again, again.value !== field(again.dataset.again).value ? page.messages.twice : "");
    }
    for (const rule of page.checks) {
      fail(field(rule.field), rule.rule(record) ? "" : rule.message);
    }
    for (const [control, message] of messages) {
      control.setCustomValidity(message);
    }
  }

  // Chooses a value in a picker, kept as an option of its own while its
  // list is not read or does not hold it.
  function choose(control, value, shown) {
    if (value !== "" && ![...control.options].some((option) => option.value === value)) {
      const option = document.createElement("option");
      option.value = value;
      option.textContent = shown || value;
      control.append(option);
    }
    control.value = value;
  }

  async function pick(name, picker, search) {
    const control = field(name);
    const query = new URLSearchParams({ [picker.pageSize[0]]: String(picker.pageSize[1]) });
    if (search) {
      query.set(picker.search, search);
    }
    try {
      const response = await fetch(`${picker.path}?${query}`, { headers: { Accept: "application/json" } });
      if (!response.ok) {
        show(page.messages.failed, true);
        return;
      }
      const items = (await response.json())[picker.items];
      const chosen = control.value;
      const shown = control.selectedOptions[0]?.textContent;
      control.replaceChildren(control.options[0]);
      for (const item of items) {
        const option = document.createElement("option");
        option.value = String(item[picker.value]);
        option.textContent = picker.shows.map((shown) => item[shown] ?? "").join(" · ");
        control.append(option);
      }
      choose(control, chosen, shown);
      control.picked = new Map(items.map((item) => [String(item[picker.value]), item]));
    } catch {
      show(page.messages.failed, true);
    }
  }

  for (const [name, picker] of Object.entries(page.pickers)) {
    const control = field(name);
    control.addEventListener("change", () => {
      const item = control.picked?.get(control.value);
      for (const [into, from] of Object.entries(picker.fills)) {
        field(into).value = item?.[from] ?? "";
      }
      check();
    });
    element.querySelector(`[data-search="${name}"]`)?.addEventListener("change", (event) => pick(name, picker, event.target.value.trim()));
    pick(name, picker, "");
  }

  element.addEventListener("input", (event) => {
    event.target.setCustomValidity?.("");
    check();
  });
  element.addEventListener("submit", async (event) => {
    event.preventDefault();
    record = read(element);
    const body = {};
    for (const name of page.submit.sent) {
      const value = sent(record[name]);
      if (value !== undefined) {
        body[name] = value;
      }
    }
    const headers = { Accept: "application/json", "Content-Type": "application/json" };
    if (page.submit.idempotencyKey) {
      headers[page.submit.idempotencyKey] = key;
    }
    button.disabled = true;
    try {
      const response = await fetch(page.submit.path(given, record), { method: page.submit.method, headers, body: JSON.stringify(body) });
      button.disabled = false;
      if (response.ok) {
        const answer = response.status === 204 ? {} : await response.json().catch(() => ({}));
        emit(page.submitted, { status: response.status, answer });
      } else {
        emit(page.refused, { status: response.status });
      }
    } catch {
      button.disabled = false;
      emit(page.refused, { status: 0 });
    }
  });

  on(page.submitted, ({ status, answer }) => {
    key = crypto.randomUUID();
    const next = page.next[String(status)] ?? page.next.success;
    if (next?.page) {
      // A parameter comes from the answer, the fields sent, the record
      // loaded, then this page's own address.
      const filled = Object.fromEntries(Object.entries(next.with).map(([name, from]) => [name, answer[from] ?? record[from] ?? loaded[from] ?? given[from]]));
      go(next.page, filled, next.message);
      return;
    }
    show(next?.message ?? "", false);
  });
  on(page.refused, ({ status }) => {
    const refusal = page.refusals.submit[String(status)];
    show(refusal?.message ?? page.messages.failed, true);
    if (refusal?.field) {
      field(refusal.field).setCustomValidity(refusal.message);
      field(refusal.field).reportValidity();
    }
  });

  show(arrived(), false);
  if (page.source) {
    button.disabled = true;
    fetch(page.source.path(given), { headers: { Accept: "application/json" } })
      .then(async (response) => {
        if (!response.ok) {
          show(page.refusals.source[String(response.status)] ?? page.messages.failed, true);
          return;
        }
        loaded = await response.json();
        fill(element, loaded);
        // A picker's list may come after the record; its value is kept.
        for (const name of Object.keys(page.pickers)) {
          choose(field(name), String(loaded[name] ?? ""));
        }
        for (const [name, from] of Object.entries(page.source.rows ?? {})) {
          for (const row of loaded[from] ?? []) {
            lists.get(name)(row, true);
          }
        }
        button.disabled = false;
        check();
      })
      .catch(() => show(page.messages.failed, true));
  } else {
    check();
  }
}
