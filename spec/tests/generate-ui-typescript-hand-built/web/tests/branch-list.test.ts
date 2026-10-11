import assert from "node:assert/strict";
import { test } from "node:test";
import { decide } from "../screens/access.ts";
import { schema } from "../app/branches/page.schema.ts";

test("branch-list is refused to someone signed in without members.read", () => {
  assert.equal(decide(schema.permission, { signedIn: true, permissions: [] }), "refused");
});

test("branch-list sends someone signed out to sign in", () => {
  assert.equal(decide(schema.permission, { signedIn: false, permissions: [] }), "sign-in");
});

test("branch-list opens to someone who holds members.read", () => {
  assert.equal(decide(schema.permission, { signedIn: true, permissions: ["members.read"] }), "allowed");
});
