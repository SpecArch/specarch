import type { FieldSchema, Rule, TaskPageSchema } from "./schema";
import { say, type Texts } from "./texts";

/** The values a page holds, by field. */
export type Values = Readonly<Record<string, string>>;

const email = /^[^\s@]+@[^\s@]+$/;

function length(value: string): number {
  return Array.from(value).length;
}

function fieldProblem(field: FieldSchema, value: string, texts: Texts): string | undefined {
  if (value === "") {
    return field.required ? say(texts, "screens.required") : undefined;
  }
  if ("minLength" in field && field.minLength !== undefined && length(value) < field.minLength) {
    return say(texts, { text: "screens.minLength", count: field.minLength });
  }
  if (field.maxLength !== undefined && length(value) > field.maxLength) {
    return say(texts, { text: "screens.maxLength", count: field.maxLength });
  }
  if ("pattern" in field && field.pattern !== undefined && !new RegExp(field.pattern, "u").test(value)) {
    return say(texts, "screens.pattern");
  }
  if (field.type === "email" && !email.test(value)) {
    return say(texts, "screens.email");
  }
  return undefined;
}

/** The value of a rule over the page's values. */
export function evaluate(rule: Rule, values: Values): unknown {
  switch (rule[0]) {
    case "field":
      return values[rule[1]] ?? "";
    case "value":
      return rule[1];
    case "!":
      return !evaluate(rule[1], values);
    case "size": {
      const value = evaluate(rule[1], values);
      return typeof value === "string" ? length(value) : 0;
    }
    case "&&":
      return evaluate(rule[1], values) === true && evaluate(rule[2], values) === true;
    case "||":
      return evaluate(rule[1], values) === true || evaluate(rule[2], values) === true;
  }
  const left = evaluate(rule[1], values);
  const right = evaluate(rule[2], values);
  switch (rule[0]) {
    case "==":
      return left === right;
    case "!=":
      return left !== right;
  }
  if (typeof left !== typeof right || (typeof left !== "string" && typeof left !== "number")) {
    return false;
  }
  const l = left as string | number;
  const r = right as string | number;
  switch (rule[0]) {
    case "<":
      return l < r;
    case "<=":
      return l <= r;
    case ">":
      return l > r;
    case ">=":
      return l >= r;
  }
}

/**
 * What stops the page being sent: each field's problem by field, then
 * each rule across fields that does not hold, under its field or, with
 * none, under "".
 */
export function problemsOf(schema: TaskPageSchema, values: Values, texts: Texts): Record<string, string> {
  const problems: Record<string, string> = {};
  for (const field of schema.fields) {
    const problem = fieldProblem(field, values[field.name] ?? "", texts);
    if (problem !== undefined) {
      problems[field.name] = problem;
    }
  }
  for (const check of schema.checks) {
    const at = check.field ?? "";
    if (problems[at] === undefined && evaluate(check.rule, values) !== true) {
      problems[at] = say(texts, check.message);
    }
  }
  return problems;
}
