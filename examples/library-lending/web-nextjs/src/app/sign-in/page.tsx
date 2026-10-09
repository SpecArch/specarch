import type { Metadata } from "next";
import { TaskPage } from "@/screens";
import { strings, texts } from "@/strings";
import { schema } from "./page.schema";

export const metadata: Metadata = { title: strings["sign-in.title"] };

const routes = {
  "members-list": "/members",
  "second-factor": "/sign-in/second-factor",
};

export default async function Page({ searchParams }: { readonly searchParams: Promise<Record<string, string | string[] | undefined>> }) {
  const { returnTo } = await searchParams;
  return <TaskPage schema={schema} texts={texts(schema)} routes={routes} returnTo={typeof returnTo === "string" ? returnTo : undefined} />;
}
