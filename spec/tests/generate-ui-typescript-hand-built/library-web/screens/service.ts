import { application } from "@/application";
import { createTransport, fill, segment, type Ask, type Target } from "./transport";

export { fill, segment };

/** Where the screens call the service, empty when it answers on the page's own origin. */
export const service = application.service;

const transport = createTransport({ service, dialect: application.dialect ?? "standard", handshakes: application.handshakes ?? {} });

/**
 * Calls an operation as the service's dialect serves it, establishing the
 * handshake its envelope needs on the first call and again when the service
 * asks for a new one, and answers the Response: for an operation in an
 * envelope, the status and body inside it.
 */
export function call(target: Target, ask?: Ask): Promise<Response> {
  return transport.call(target, ask);
}

/** The JSON body of an answer, or undefined when it has none or cannot be read. */
export async function body(response: Response): Promise<unknown> {
  try {
    return await response.json();
  } catch {
    return undefined;
  }
}

/** A property of an answer's body; a name with dots reads into nested objects, as the dxlib dialect's list.rows. */
export function property(value: unknown, name: string): unknown {
  let at = value;
  for (const part of name.split(".")) {
    at = typeof at === "object" && at !== null ? (at as Record<string, unknown>)[part] : undefined;
  }
  return at;
}
