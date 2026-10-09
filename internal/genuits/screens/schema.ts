import type { StringKey } from "@/strings";

/** A text to show: a string key, and the values its placeholders take. */
export interface Message {
  readonly text: StringKey;
  readonly count?: number;
  readonly pattern?: string;
}

/**
 * A rule over the values of a page, from an expression of the
 * specification: a field's value, a constant, or an operator and its
 * operands.
 */
export type Rule =
  | readonly ["field", string]
  | readonly ["value", string | number | boolean | null]
  | readonly ["!", Rule]
  | readonly ["size", Rule]
  | readonly ["==" | "!=" | "<" | "<=" | ">" | ">=" | "&&" | "||", Rule, Rule];

interface FieldBase {
  readonly name: string;
  readonly label: StringKey;
  readonly required: boolean;
}

/** A field of type string with no format the other field types take. */
export interface TextFieldSchema extends FieldBase {
  readonly type: "text";
  readonly minLength?: number;
  readonly maxLength?: number;
  readonly pattern?: string;
}

/** A field of format email. */
export interface EmailFieldSchema extends FieldBase {
  readonly type: "email";
  readonly maxLength?: number;
}

/** A field of format password, its rules shown under it. */
export interface PasswordFieldSchema extends FieldBase {
  readonly type: "password";
  readonly minLength?: number;
  readonly maxLength?: number;
  readonly pattern?: string;
  readonly rules: readonly Message[];
}

export type FieldSchema = TextFieldSchema | EmailFieldSchema | PasswordFieldSchema;

/** A rule across the fields, checked before the page is sent. */
export interface CheckSchema {
  readonly name: string;
  readonly rule: Rule;
  readonly message: StringKey;
  readonly field?: string;
}

/**
 * A problem the operation answers, and where the page shows it; with no
 * status, every problem the page names no message for.
 */
export interface FailureSchema {
  readonly status?: number;
  readonly problem: string;
  readonly message: StringKey;
  readonly field?: string;
}

/**
 * What a success the operation answers leads to: a page by its name, the
 * route parameters taken from the answer's body, and a message. A page
 * that keeps returnTo hands on the page sign-in was asked for from; any
 * other page gives way to it.
 */
export interface EventSchema {
  readonly status: number;
  readonly navigate?: string;
  readonly with?: Readonly<Record<string, string>>;
  readonly message?: StringKey;
  readonly keepsReturnTo?: boolean;
}

/** The operation a page sends its fields to. */
export interface SubmitSchema {
  readonly operation: string;
  readonly method: "POST" | "PUT" | "PATCH" | "DELETE";
  readonly path: string;
}

/** A form that submits to an operation without loading a record. */
export interface TaskPageSchema {
  readonly title: StringKey;
  readonly submit: SubmitSchema;
  readonly fields: readonly FieldSchema[];
  readonly checks: readonly CheckSchema[];
  readonly failed: readonly FailureSchema[];
  readonly events: readonly EventSchema[];
}
