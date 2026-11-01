import assert from "node:assert/strict";
import { test } from "node:test";
import { decide } from "../screens/access.js";
import { page } from "../pages/member-form.js";

test("member-form is refused to someone signed in without members.write", () => {
  assert.equal(decide(page.permission, { signedIn: true, permissions: [] }), "refused");
});

test("member-form sends someone signed out to sign in", () => {
  assert.equal(decide(page.permission, { signedIn: false, permissions: [] }), "sign-in");
});

test("member-form opens to someone who holds members.write", () => {
  assert.equal(decide(page.permission, { signedIn: true, permissions: ["members.write"] }), "allowed");
});
