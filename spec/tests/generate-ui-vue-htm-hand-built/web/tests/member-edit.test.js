import assert from "node:assert/strict";
import { test } from "node:test";
import { decide } from "../screens/access.js";
import { page } from "../pages/member-edit.js";

test("member-edit is refused to someone signed in without members.write", () => {
  assert.equal(decide(page.permission, { signedIn: true, permissions: [] }), "refused");
});

test("member-edit sends someone signed out to sign in", () => {
  assert.equal(decide(page.permission, { signedIn: false, permissions: [] }), "sign-in");
});

test("member-edit opens to someone who holds members.write", () => {
  assert.equal(decide(page.permission, { signedIn: true, permissions: ["members.write"] }), "allowed");
});
