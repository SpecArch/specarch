import type { Metadata } from "next";
import { FormPage, Guard } from "@/screens";
import { chosenLanguage } from "@/language";
import { stringOf, texts } from "@/strings";
import { schema } from "./page.schema";

export async function generateMetadata(): Promise<Metadata> {
  return { title: stringOf("removal-form.title", await chosenLanguage()) };
}

const routes = {
  "members-list": "/members",
};

export default async function Page({ params }: { readonly params: Promise<Record<string, string>> }) {
  const parameters = await params;
  const t = texts(schema, await chosenLanguage());
  return (
    <Guard permission={schema.permission} texts={t}>
      <FormPage schema={schema} texts={t} routes={routes} parameters={parameters} />
    </Guard>
  );
}
