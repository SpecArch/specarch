import type { Metadata } from "next";
import type { ReactNode } from "react";
import { application } from "@/application";
import { Menu, Notice } from "@/screens";
import { texts } from "@/strings";
import "./globals.scss";

export const metadata: Metadata = {
  title: { default: "Library lending", template: "%s · Library lending" },
};

export default function RootLayout({ children }: { readonly children: ReactNode }) {
  return (
    <html lang="en">
      <body>
        <Menu texts={texts(application.menu)} />
        <main className="screens-main">
          <Notice />
          {children}
        </main>
      </body>
    </html>
  );
}
