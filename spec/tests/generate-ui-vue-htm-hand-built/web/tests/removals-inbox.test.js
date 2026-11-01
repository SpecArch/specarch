import assert from "node:assert/strict";
import { test } from "node:test";
import { decide } from "../screens/access.js";
import { page } from "../pages/removals-inbox.js";

test("removals-inbox is refused to someone signed in without members.approve", () => {
  assert.equal(decide(page.permission, { signedIn: true, permissions: [] }), "refused");
});

test("removals-inbox sends someone signed out to sign in", () => {
  assert.equal(decide(page.permission, { signedIn: false, permissions: [] }), "sign-in");
});

test("removals-inbox opens to someone who holds members.approve", () => {
  assert.equal(decide(page.permission, { signedIn: true, permissions: ["members.approve"] }), "allowed");
});
