import type { Metadata } from "next";
import { TaskPage } from "@/screens";
import { chosenLanguage } from "@/language";
import { stringOf, texts } from "@/strings";
import { schema } from "./page.schema";

export async function generateMetadata(): Promise<Metadata> {
  return { title: stringOf("sign-in.title", await chosenLanguage()) };
}

const routes = {
  "member-view": "/members/{memberId}",
};

export default async function Page({ searchParams }: { readonly searchParams: Promise<Record<string, string | string[] | undefined>> }) {
  const { returnTo } = await searchParams;
  const t = texts(schema, await chosenLanguage());
  return <TaskPage schema={schema} texts={t} routes={routes} returnTo={typeof returnTo === "string" ? returnTo : undefined} />;
}
