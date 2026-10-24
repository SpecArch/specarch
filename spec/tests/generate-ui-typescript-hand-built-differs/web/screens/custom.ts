import type { CustomPageSchema } from "./schema";
import type { Texts } from "./texts";

/**
 * What the route of a custom page hands the screen the project writes: the
 * default export of ./custom/<page> beside the route's page.tsx takes these
 * props. routes are the routes of the pages its actions open, by page.
 */
export interface CustomPageProps {
  readonly schema: CustomPageSchema;
  readonly texts: Texts;
  readonly parameters: Readonly<Record<string, string>>;
  readonly routes: Readonly<Record<string, string>>;
}
