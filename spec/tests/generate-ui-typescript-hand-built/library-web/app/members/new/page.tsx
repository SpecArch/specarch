import type { Metadata } from "next";
import { Guard } from "@/screens";
import { chosenLanguage } from "@/language";
import { stringOf, texts } from "@/strings";
import { HookedForm } from "./page.client";
import { schema } from "./page.schema";

export async function generateMetadata(): Promise<Metadata> {
  return { title: stringOf("member-form.title", await chosenLanguage()) };
}

const routes = {
  "member-view": "/members/{memberId}",
};

export default async function Page() {
  const t = texts(schema, await chosenLanguage());
  return (
    <Guard permission={schema.permission} texts={t}>
      <HookedForm schema={schema} texts={t} routes={routes} />
    </Guard>
  );
}
