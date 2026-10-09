"use client";

/**
 * The list screen of @acme/screens, a fictional library: a stub that draws
 * a page's heading and its column headings, so a list generated through the
 * example's override is type-checked and built against the library's own
 * names. A real library reads the rest of the schema as well.
 */
export interface AcmeColumn {
  readonly field: string;
  readonly title: string;
  readonly [key: string]: unknown;
}

export interface AcmeListSchema {
  readonly heading: string;
  readonly permission: string;
  readonly columnDefs: readonly AcmeColumn[];
  readonly [key: string]: unknown;
}

export interface AcmeListScreenProps {
  readonly schema: AcmeListSchema;
  readonly texts: Readonly<Record<string, string>>;
  readonly routes: Readonly<Record<string, string>>;
}

export function AcmeListScreen({ schema, texts }: AcmeListScreenProps) {
  return (
    <section>
      <h1>{texts[schema.heading] ?? schema.heading}</h1>
      <table>
        <thead>
          <tr>
            {schema.columnDefs.map((column) => (
              <th key={column.field}>{texts[column.title] ?? column.title}</th>
            ))}
          </tr>
        </thead>
      </table>
    </section>
  );
}
