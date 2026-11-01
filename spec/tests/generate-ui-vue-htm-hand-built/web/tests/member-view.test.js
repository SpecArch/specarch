import assert from "node:assert/strict";
import { test } from "node:test";
import { decide } from "../screens/access.js";
import { page } from "../pages/member-view.js";

test("member-view is refused to someone signed in without members.read", () => {
  assert.equal(decide(page.permission, { signedIn: true, permissions: [] }), "refused");
});

test("member-view sends someone signed out to sign in", () => {
  assert.equal(decide(page.permission, { signedIn: false, permissions: [] }), "sign-in");
});

test("member-view opens to someone who holds members.read", () => {
  assert.equal(decide(page.permission, { signedIn: true, permissions: ["members.read"] }), "allowed");
});
