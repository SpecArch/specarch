import type { Metadata } from "next";
import type { ReactNode } from "react";
import { Notice } from "@/screens";
import "./globals.scss";

export const metadata: Metadata = {
  title: { default: "Library lending", template: "%s · Library lending" },
};

export default function RootLayout({ children }: { readonly children: ReactNode }) {
  return (
    <html lang="en">
      <body>
        <main className="screens-main">
          <Notice />
          {children}
        </main>
      </body>
    </html>
  );
}
