import type { Metadata } from "next";
import { DataPage } from "@acme/screens/data";
import { Guard } from "@/screens";
import { chosenLanguage } from "@/language";
import { stringOf, texts } from "@/strings";
import { schema } from "./page.schema";

export async function generateMetadata(): Promise<Metadata> {
  return { title: stringOf("members-list.title", await chosenLanguage()) };
}

const routes = {
  invitation: "/invitations",
  "member-edit": "/members/{memberId}/edit",
  "member-view": "/members/{memberId}",
};

export default async function Page() {
  const t = texts(schema, await chosenLanguage());
  return (
    <Guard permission={schema.permission} texts={t}>
      <DataPage schema={schema} texts={t} routes={routes} />
    </Guard>
  );
}
