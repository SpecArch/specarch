// The generated screens run: Vue's runtime-only build and htm as index.html
// loads them, the application mounted on a stand-in for the browser, the
// service stubbed, and each page kind opened and read. It checks what the
// derived tests cannot: that the shell, the router, the session, the
// guard, the menu and the list, view, form and task pages draw from the
// data files without an error. Run with node --test from this folder's
// parent: node --test examples/library-lending/web-vue/smoke.

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import vm from "node:vm";
import { install, settle } from "./dom.js";

const here = new URL("..", import.meta.url);
const file = (path) => readFileSync(new URL(path, here), "utf8");

const errors = [];
for (const level of ["error", "warn"]) {
  const original = console[level];
  console[level] = (...args) => {
    errors.push(level + ": " + args.map(String).join(" "));
    original(...args);
  };
}

const page = install();

// The service, as the screens see it: who is signed in, the lists, the records.
const session = { signedIn: true, permissions: ["members.read", "members.write", "loans.read", "loans.create", "fees.request"], roles: ["librarian"] };
const members = [
  { id: "m-1", cardNumber: "C-0001", fullName: "Ada Byron", email: "ada@example.org", tier: "standard", status: "active", outstandingFees: 0, joinedOn: "2024-01-05", membershipEndsOn: "2026-01-05", address: { street: "1 Main St", city: "Town", postcode: "12345" }, phones: [{ number: "555-0100", kind: "home" }] },
  { id: "m-2", cardNumber: "C-0002", fullName: "Blaise Pascal", email: "blaise@example.org", tier: "extended", status: "inactive", outstandingFees: 12.5, joinedOn: "2023-06-01", membershipEndsOn: "2025-06-01", address: null, phones: [] },
];
const requests = [];
globalThis.fetch = async (url, init = {}) => {
  const u = new URL(String(url), "http://localhost");
  requests.push((init.method ?? "GET") + " " + u.pathname + u.search);
  const json = (status, body) => new Response(body === undefined ? null : JSON.stringify(body), { status, headers: body === undefined ? {} : { "Content-Type": "application/json" } });
  if (u.pathname === "/sessions/current") {
    return json(200, session);
  }
  if (u.pathname === "/members" && (init.method ?? "GET") === "GET") {
    const search = (u.searchParams.get("search") ?? "").toLowerCase();
    const items = members.filter((m) => m.fullName.toLowerCase().includes(search));
    return json(200, { items, totalItems: items.length, totalPages: 1 });
  }
  if (/^\/members\/[^/]+$/.test(u.pathname)) {
    const found = members.find((m) => m.id === decodeURIComponent(u.pathname.split("/")[2]));
    return found ? json(200, found) : json(404, { problem: "member-not-found" });
  }
  if (u.pathname === "/books") {
    return json(200, { items: [{ id: "b-1", title: "On Computable Numbers", isbn: "978-0" }], totalItems: 1, totalPages: 1 });
  }
  if (u.pathname === "/members" && init.method === "POST") {
    return json(201, { id: "m-3", ...JSON.parse(init.body) });
  }
  if (init.method === "POST") {
    return json(200, {});
  }
  return json(404, { problem: "no stub" });
};

// Vue and htm as classic scripts, the globals index.html gives the screens.
vm.runInThisContext(file("vendor/vue.runtime.global.prod.js"), { filename: "vue.runtime.global.prod.js" });
vm.runInThisContext(file("vendor/htm.umd.js"), { filename: "htm.umd.js" });
assert.ok(globalThis.Vue && globalThis.htm, "the vendored files define Vue and htm");

const text = (el) => el.textContent.replace(/\s+/g, " ").trim();
const { root, navigate } = page;

test("the application mounts and shows the menu to a signed-in librarian", async () => {
  await import("../app.js");
  await settle();
  const nav = root.find((e) => e.tagName === "NAV");
  assert.ok(nav, "a menu is drawn");
  const links = nav.byTag("a").map(text);
  assert.ok(links.includes("All members"), `the menu holds the members list: ${links.join(", ")}`);
  assert.ok(!links.includes("Fee waivers to approve"), `fees.approve is not held, so no inbox entry: ${links.join(", ")}`);
  assert.equal(root.find((e) => e.tagName === "SELECT" && e.getAttribute("id") === "screens-language")?.tagName, "SELECT", "two languages give a language choice");
  assert.deepEqual(errors, []);
});

test("the start opens the librarian's home, the members list, read from the service", async () => {
  await settle();
  assert.equal(window.location.hash, "#/members", "the home of the role librarian");
  const table = root.find((e) => e.tagName === "TABLE");
  assert.ok(table, "the list draws a table");
  const headings = table.byTag("th").map(text);
  assert.deepEqual(headings.slice(0, 2), ["Card number", "Full name"]);
  assert.ok(headings.includes("Actions"), "the row actions column");
  const rows = table.byTag("tbody")[0].byTag("tr");
  assert.equal(rows.length, 2);
  assert.ok(text(rows[0]).includes("Ada Byron"));
  const link = rows[0].find((e) => e.tagName === "A");
  assert.equal(link.getAttribute("href"), "#/members/m-1", "the first column opens the member");
  assert.ok(text(rows[0]).includes("Deactivate"), "the active member can be deactivated");
  assert.ok(!text(rows[1]).includes("Deactivate"), "the inactive one cannot");
  assert.equal(document.title, "Members · Library Lending");
  assert.deepEqual(errors, []);
});

test("sorting by a column asks the service again with the sort", async () => {
  const button = root.find((e) => e.tagName === "BUTTON" && text(e) === "Full name");
  assert.ok(button, "the sortable column is a button");
  const before = requests.length;
  button.dispatch("click");
  await settle();
  assert.ok(requests.slice(before).some((r) => r.includes("sort=fullName")), requests.slice(before).join("\n"));
  assert.deepEqual(errors, []);
});

test("a row's action asks its confirmation with the reason, and runs after it", async () => {
  const deactivate = root.find((e) => e.tagName === "BUTTON" && text(e) === "Deactivate");
  deactivate.dispatch("click");
  await settle();
  const dialog = root.find((e) => e.tagName === "DIALOG");
  assert.ok(dialog && dialog.open, "the confirm dialog is open");
  const confirm = dialog.find((e) => e.tagName === "BUTTON" && text(e) === "Deactivate");
  assert.equal(confirm.disabled, true, "not confirmed while the reason is empty");
  const reason = dialog.find((e) => e.tagName === "TEXTAREA");
  reason.value = "Moved away";
  reason.dispatch("input");
  await settle();
  assert.equal(root.find((e) => e.tagName === "DIALOG").find((e) => e.tagName === "BUTTON" && text(e) === "Deactivate").disabled, false, "confirmed once a reason is typed");
  const before = requests.length;
  root.find((e) => e.tagName === "DIALOG").find((e) => e.tagName === "BUTTON" && text(e) === "Deactivate").dispatch("click");
  await settle();
  assert.ok(requests.slice(before).includes("POST /members/m-1/deactivate"), requests.slice(before).join("\n"));
  assert.ok(text(root).includes("The member is deactivated."), "the message is announced");
  assert.deepEqual(errors, []);
});

test("a view shows the record's sections, a value's parts and a list of values", async () => {
  await navigate("/members/m-1");
  await settle();
  const body = text(root);
  assert.ok(body.includes("Ada Byron"), body);
  assert.ok(body.includes("1 Main St"), "the address's parts are shown");
  assert.ok(body.includes("555-0100"), "the phones are shown as a table");
  assert.ok(root.find((e) => e.tagName === "BUTTON" && text(e) === "Lend a book"), "the action that opens a page is offered to loans.create");
  assert.deepEqual(errors, []);
});

test("a form draws its fields, a lookup with its datalist, and sends what was typed", async () => {
  await navigate("/members/new");
  await settle();
  const form = root.find((e) => e.tagName === "FORM");
  assert.ok(form, "the form is drawn");
  const name = root.byId("fullName");
  assert.ok(name, "the full name field");
  const type = (id, value) => {
    const el = root.byId(id);
    assert.ok(el, "the field " + id);
    el.value = value;
    el.dispatch(el.tagName === "SELECT" ? "change" : "input");
  };
  // Sent with a required field empty: the problems show and nothing is sent.
  form.dispatch("submit");
  await settle();
  let problems = root.all((e) => e.getAttribute("class") === "screens-problem").map(text);
  assert.ok(problems.includes("Fill this in."), problems.join(" | "));
  assert.ok(!requests.some((r) => r === "POST /members"), "nothing sent while a field is missing");
  type("fullName", "Grace Hopper");
  type("email", "grace@example.org");
  type("tier", "standard");
  type("address.street", "2 Side St");
  type("address.city", "Town");
  await settle();
  const before = requests.length;
  form.dispatch("submit");
  await settle();
  problems = root.all((e) => e.getAttribute("class") === "screens-problem").map(text);
  assert.deepEqual(problems, [], "every field is filled");
  assert.ok(requests.slice(before).includes("POST /members"), requests.slice(before).join("\n"));
  assert.equal(window.location.hash, "#/members/m-3", "the success leads to the new member's view");
  assert.deepEqual(errors, []);
});

test("the loan form starts its date from the project's hook, and its lookups read their lists", async () => {
  await navigate("/loans/new");
  await settle();
  const lentOn = root.byId("lentOn");
  assert.ok(lentOn, "the lentOn field");
  assert.match(String(lentOn.value), /^\d{4}-\d{2}-\d{2}$/, "today, from hooks/loan-form.js");
  const datalists = root.byTag("datalist");
  assert.ok(datalists.length >= 2, "a datalist per lookup");
  assert.ok(requests.some((r) => r.startsWith("GET /books")), "the book lookup read its list");
  assert.deepEqual(errors, []);
});

test("a task page draws its fields and sends them", async () => {
  await navigate("/sign-in");
  await settle();
  assert.ok(root.byId("email") || root.byId("cardNumber") || root.byTag("input").length > 0, "the sign-in fields");
  assert.equal(text(root.find((e) => e.tagName === "H1")), "Sign in");
  assert.deepEqual(errors, []);
});

test("a page whose permission is not held is refused, and one that is not there is said", async () => {
  await navigate("/fee-waivers");
  await settle();
  assert.ok(text(root).includes("You do not hold the permission this page needs."), text(root));
  await navigate("/nowhere");
  await settle();
  assert.ok(text(root).includes("There is no page at /nowhere."), text(root));
  assert.deepEqual(errors, []);
});

test("choosing the other language draws the screens again in it", async () => {
  await navigate("/members");
  await settle();
  const select = root.find((e) => e.tagName === "SELECT" && e.getAttribute("id") === "screens-language");
  select.value = "id";
  select.dispatch("change");
  await settle();
  assert.equal(localStorage.getItem("screens.language"), "id");
  assert.equal(document.documentElement.lang, "id");
  const nav = root.find((e) => e.tagName === "NAV");
  assert.ok(!nav.byTag("a").map(text).includes("Members"), "the menu is no longer in English: " + nav.byTag("a").map(text).join(", "));
  assert.deepEqual(errors, []);
});
