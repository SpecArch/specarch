"use client";

import type { ComponentProps } from "react";
import { FormPage } from "@/screens";
import { joinedOn } from "./page.hooks";

const hooks = { joinedOn };

/** The form with the hooks of its fields, which run in the browser. */
export function HookedForm(props: Omit<ComponentProps<typeof FormPage>, "hooks">) {
  return <FormPage {...props} hooks={hooks} />;
}
