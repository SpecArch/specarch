package genui

import (
	"html"
	"strings"
)

// head writes the start of a page's HTML up to its main element's title:
// the page's language and title, the theme, its module and the menu.
func (g *gen) head(name, title, language string) []string {
	out := []string{
		"<!doctype html>",
		"<html lang=" + quoted(language) + ">",
		"<head>",
		`<meta charset="utf-8">`,
		`<meta name="viewport" content="width=device-width, initial-scale=1">`,
		"<title>" + html.EscapeString(title) + "</title>",
		`<link rel="stylesheet" href="theme.css">`,
		`<script type="module" src="` + name + `.js"></script>`,
		"</head>",
		"<body>",
	}
	out = append(out, g.menuHTML(name)...)
	return append(out, "<main>", `<h1 id="title">`+html.EscapeString(title)+"</h1>")
}

// html writes the list page's HTML.
func (p *page) html(g *gen) string {
	var b strings.Builder
	w := func(s string) { b.WriteString(s + "\n") }
	for _, l := range g.head(p.name, p.title, p.language) {
		w(l)
	}
	if len(p.links) > 0 {
		w(`<p id="links">`)
		for _, l := range p.links {
			w("<a href=" + quoted(l.page+".html") + ">" + html.EscapeString(l.label) + "</a>")
		}
		w("</p>")
	}
	if len(p.filters) > 0 {
		w(`<form id="filters" aria-label="Filters">`)
		for _, l := range p.filterHTML {
			w(l)
		}
		w(`<button type="submit">Filter</button>`)
		w("</form>")
	}
	w(`<p id="status" role="status" aria-live="polite"></p>`)
	w(`<table id="rows" aria-labelledby="title" tabindex="-1">`)
	w("<thead>")
	w("<tr>")
	for i, c := range p.columns {
		class := ""
		if p.wide[c] {
			class = ` class="wide"`
		}
		w(`<th scope="col"` + class + `>` + html.EscapeString(p.titles[i]) + `</th>`)
	}
	if len(p.actions) > 0 {
		w(`<th scope="col">Actions</th>`)
	}
	w("</tr>")
	w("</thead>")
	w("<tbody></tbody>")
	w("</table>")
	w(`<nav id="pager" aria-label="Pages">`)
	w(`<button type="button" data-step="-1">Previous page</button>`)
	w("<span></span>")
	w(`<button type="button" data-step="1">Next page</button>`)
	w("</nav>")
	w("</main>")
	w("</body>")
	w("</html>")
	return b.String()
}

// operations are the actions that run an operation, without the links.
func operations(actions []action) []action {
	var out []action
	for _, a := range actions {
		if a.link == nil {
			out = append(out, a)
		}
	}
	return out
}

// js writes the list page's module.
func (p *page) js() string {
	var b strings.Builder
	w := func(s string) { b.WriteString(s + "\n") }
	ops := operations(p.actions)
	intro := "The page " + p.name + ": " + p.title + ". It lists " + p.lowerTitle() + " through " + p.source + ", a page at a time"
	if len(ops) > 0 {
		var names []string
		for _, a := range ops {
			names = append(names, a.operation)
		}
		intro += ", and runs " + joinAnd(names) + " on a row"
	}
	for _, l := range wrap(intro+".", 72-3) {
		w("// " + l)
	}
	w("")
	w("import {")
	w("  on, emit,")
	w("  " + p.upper + "_FILTERED, " + p.upper + "_PAGED, " + p.upper + "_LOADED, " + p.upper + "_FAILED,")
	for _, a := range ops {
		u := upperSnake(a.operation)
		w("  " + u + "_SUCCEEDED, " + u + "_FAILED,")
	}
	w(`} from "./events.js";`)
	switch {
	case len(p.actions) > 0:
		w(`import { arrived, go, href } from "./pages.js";`)
	case p.selected != nil:
		w(`import { arrived, href } from "./pages.js";`)
	default:
		w(`import { arrived } from "./pages.js";`)
	}
	if len(ops) > 0 {
		w(`import { ask } from "./ask.js";`)
	}
	w("")
	var cols []string
	for _, c := range p.columns {
		cols = append(cols, jsString(c))
	}
	w("const columns = [" + strings.Join(cols, ", ") + "];")
	w("const pageSize = " + p.pageSize + ";")
	w("")
	w("const messages = {")
	w("  empty: " + jsString(p.empty) + ",")
	w("  filteredEmpty: " + jsString(p.filteredEmpty) + ",")
	w("  failed: " + jsString(p.failed) + ",")
	w("};")
	w("")
	w("// What each operation's refusals say, by status: the page's failed states.")
	w("const refusals = {")
	writeRefusals(w, p.source, p.sourceRefusals)
	seen := map[string]bool{p.source: true}
	for _, a := range ops {
		if !seen[a.operation] {
			seen[a.operation] = true
			writeRefusals(w, a.operation, a.refusals)
		}
	}
	w("};")
	w("")
	w("const actions = [")
	for _, a := range p.actions {
		actionJS(w, a, "  ", "row")
	}
	w("];")
	if p.selected != nil {
		w("")
		w("// The page a row opens when it is selected, from its first column.")
		w("const select = { page: " + jsString(p.selected.page) + ", with: " + p.selected.with + " };")
	}
	if len(p.filterKeys) > 0 {
		w("")
		w("// The filters the list's operation takes by the paginated-list idiom's name.")
		w("const filterKeys = {")
		for _, f := range p.filters {
			if k := p.filterKeys[f]; k != "" {
				w("  " + f + ": " + jsString(k) + ",")
			}
		}
		w("};")
	}
	id := "undefined"
	if p.key != "" {
		id = "row." + p.key
	}
	b.WriteString(strings.NewReplacer(
		"{{UPPER}}", p.upper,
		"{{PATH}}", p.sourcePath,
		"{{PAGE}}", p.list["page"],
		"{{PAGESIZE}}", p.list["pageSize"],
		"{{ITEMS}}", p.list["items"],
		"{{TOTALITEMS}}", p.list["totalItems"],
		"{{TOTALPAGES}}", p.list["totalPages"],
		"{{SOURCE}}", p.source,
		"{{TITLE}}", p.lowerTitle(),
		"{{ID}}", id,
	).Replace(listBody(len(p.filters) > 0, len(p.filterKeys) > 0, len(p.actions) > 0, len(ops) != len(p.actions), len(ops) > 0, p.selected != nil)))
	return b.String()
}

// writeRefusals writes what an operation's refusals say, by status.
func writeRefusals(w func(string), op string, rs [][2]string) {
	if len(rs) == 0 {
		w("  " + op + ": {},")
		return
	}
	w("  " + op + ": {")
	for _, r := range rs {
		w("    " + jsString(r[0]) + ": " + jsString(r[1]) + ",")
	}
	w("  },")
}

// listBody is the part of a list's module that is the same for every list,
// with the parts it has: filters, filters by the idiom's name, actions,
// actions that open a page, actions that run an operation, and a row that
// opens a page when selected.
func listBody(filters, filterKeys, actions, links, running, selected bool) string {
	var b strings.Builder
	key := "name"
	if filterKeys {
		key = "filterKeys[name] ?? name"
	}
	ops := "actions"
	if links {
		ops = "actions.filter((action) => action.operation)"
	}
	b.WriteString(`
/** The message of a refusal of an operation, or the page's default. */
function refusal(operation, status) {
  return refusals[operation][String(status)] ?? messages.failed;
}

/** Reads the list: the filters given, then the page. Only the answer to
    the latest request counts, and a page becomes the list's page once it
    has arrived. */
function loader() {
  let filters = {};
  let page = 1;
  let latest = 0;
  async function load(wanted) {
    const asked = ++latest;
    const query = new URLSearchParams();
    for (const [name, value] of Object.entries(filters)) {
      if (value !== "") {
        query.set(` + key + `, value);
      }
    }
    const filtered = [...query.keys()].length > 0;
    query.set("{{PAGE}}", String(wanted));
    query.set("{{PAGESIZE}}", String(pageSize));
    try {
      const response = await fetch(` + "`{{PATH}}?${query}`" + `, { headers: { Accept: "application/json" } });
      if (asked !== latest) {
        return;
      }
      if (!response.ok) {
        emit({{UPPER}}_FAILED, { status: response.status });
        return;
      }
      const body = await response.json();
      if (asked !== latest) {
        return;
      }
      if (body.{{ITEMS}}.length === 0 && wanted > 1 && body.{{TOTALPAGES}} > 0 && wanted > body.{{TOTALPAGES}}) {
        load(body.{{TOTALPAGES}}); // the page asked for is past the last
        return;
      }
      page = wanted;
      emit({{UPPER}}_LOADED, { items: body.{{ITEMS}}, totalItems: body.{{TOTALITEMS}}, totalPages: body.{{TOTALPAGES}}, page, filtered });
    } catch {
      if (asked === latest) {
        emit({{UPPER}}_FAILED, { status: 0 });
      }
    }
  }
  on({{UPPER}}_FILTERED, (payload) => {
    filters = payload;
    load(1);
  });
  on({{UPPER}}_PAGED, (payload) => load(payload.page));
  for (const action of ` + ops + `) {
    on(action.succeeded, () => load(page));
  }
  load(1);
}
`)
	if filters {
		b.WriteString(`
/** The filters form. */
function filterForm() {
  const form = document.getElementById("filters");
  form.addEventListener("submit", (event) => {
    event.preventDefault();
    const given = [...new FormData(form)].map(([name, value]) => [name, String(value).trim()]);
    emit({{UPPER}}_FILTERED, Object.fromEntries(given));
  });
}
`)
	}
	if running {
		b.WriteString(`
/** Runs an action on a row once the person confirms it and gives the
    reason it asks for, its button disabled while it runs; an action that
    leads to another page opens it once it has succeeded. */
async function run(action, row, button) {
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
    const response = await fetch(action.path(row), init);
    if (response.ok) {
      emit(action.succeeded, { id: {{ID}} });
      if (action.navigate) {
        go(action.navigate.page, action.navigate.with(row), action.message);
      }
    } else {
      button.disabled = false;
      emit(action.failed, { id: {{ID}}, status: response.status });
    }
  } catch {
    button.disabled = false;
    emit(action.failed, { id: {{ID}}, status: 0 });
  }
}
`)
	}
	b.WriteString(`
/** The table of {{TITLE}}. Focus in it stays on the same control, or moves
    to the table when its row is gone. */
function table() {
  const body = document.querySelector("#rows tbody");
  const head = document.querySelectorAll("#rows thead th");
  on({{UPPER}}_LOADED, ({ items }) => {
    const focused = body.contains(document.activeElement) ? document.activeElement.dataset.key : undefined;
    body.replaceChildren();
    for (const row of items) {
      const tr = document.createElement("tr");
      columns.forEach((name, i) => {
        const td = document.createElement("td");
        td.className = head[i].className;
`)
	if selected {
		b.WriteString(`        if (i === 0) {
          const link = document.createElement("a");
          link.href = href(select.page, select.with(row));
          link.textContent = row[name] ?? "";
          link.dataset.key = ` + "`select ${{{ID}}}`" + `;
          td.append(link);
        } else {
          td.textContent = row[name] ?? "";
        }
`)
	} else {
		b.WriteString(`        td.textContent = row[name] ?? "";
`)
	}
	b.WriteString(`        tr.append(td);
      });
`)
	if actions {
		b.WriteString(`      const cell = document.createElement("td");
      for (const action of actions) {
        if (action.when && !action.when(row)) {
          continue;
        }
        const label = ` + "`${action.label} ${row[columns[0]] ?? \"\"}`" + `.trim();
        if (action.page) {
          const link = document.createElement("a");
          link.href = href(action.page, action.with(row));
          link.textContent = action.label;
          link.setAttribute("aria-label", label);
          link.dataset.key = ` + "`${action.page} ${{{ID}}}`" + `;
          cell.append(link);
          continue;
        }
        const button = document.createElement("button");
        button.type = "button";
        button.textContent = action.label;
        button.setAttribute("aria-label", label);
        button.dataset.key = ` + "`${action.operation} ${{{ID}}}`" + `;
        button.addEventListener("click", () => run(action, row, button));
        cell.append(button);
      }
      tr.append(cell);
`)
	}
	b.WriteString(`      body.append(tr);
    }
    if (focused !== undefined) {
      const again = [...body.querySelectorAll("a, button")].find((control) => control.dataset.key === focused);
      (again ?? document.getElementById("rows")).focus();
    }
  });
}

/** The status line: states and the messages of events, announced without
    moving focus, the message the page that opened this one left first. */
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
  on({{UPPER}}_FILTERED, () => show("", false, false));
  on({{UPPER}}_PAGED, () => show("", false, false));
  on({{UPPER}}_LOADED, ({ items, filtered }) => {
    if (kept) {
      return;
    }
    show(items.length === 0 ? (filtered ? messages.filteredEmpty : messages.empty) : "", false, false);
  });
  on({{UPPER}}_FAILED, ({ status }) => show(refusal("{{SOURCE}}", status), true, false));
  for (const action of ` + ops + `) {
    on(action.succeeded, () => show(action.message, false, true));
    on(action.failed, ({ status }) => show(refusal(action.operation, status), true, true));
  }
  const carried = arrived();
  show(carried, false, carried !== "");
}

/** The pager. A button that is disabled while it has focus hands focus on. */
function pager() {
  const nav = document.getElementById("pager");
  const where = nav.querySelector("span");
  const [previous, next] = nav.querySelectorAll("button");
  let page = 1;
  on({{UPPER}}_LOADED, (payload) => {
    page = payload.page;
    where.textContent = ` + "`Page ${payload.page} of ${Math.max(payload.totalPages, 1)}`" + `;
    const focused = document.activeElement;
    previous.disabled = payload.page <= 1;
    next.disabled = payload.page >= payload.totalPages;
    if (focused instanceof HTMLButtonElement && nav.contains(focused) && focused.disabled) {
      const other = focused === previous ? next : previous;
      (other.disabled ? document.getElementById("rows") : other).focus();
    }
  });
  for (const button of [previous, next]) {
    button.addEventListener("click", () => emit({{UPPER}}_PAGED, { page: page + Number(button.dataset.step) }));
  }
}

`)
	if filters {
		b.WriteString("filterForm();\n")
	}
	b.WriteString("table();\nstatusLine();\npager();\nloader();\n")
	return b.String()
}
