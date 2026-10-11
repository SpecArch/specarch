import type { Metadata } from "next";
import { Guard, ListPage } from "@/screens";
import { chosenLanguage } from "@/language";
import { stringOf, texts } from "@/strings";
import { schema } from "./page.schema";

export async function generateMetadata(): Promise<Metadata> {
  return { title: stringOf("branch-list.title", await chosenLanguage()) };
}

const routes = {
};

export default async function Page() {
  const t = texts(schema, await chosenLanguage());
  return (
    <Guard permission={schema.permission} texts={t}>
      <ListPage schema={schema} texts={t} routes={routes} />
    </Guard>
  );
}
