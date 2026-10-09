import type { Metadata } from "next";
import type { ReactNode } from "react";
import { application } from "@/application";
import { chosenLanguage } from "@/language";
import { LanguageChoice, Menu, Notice } from "@/screens";
import { languageCookie, languages, texts } from "@/strings";
import "./globals.scss";

export const metadata: Metadata = {
  title: { default: "Library lending", template: "%s · Library lending" },
};

export default async function RootLayout({ children }: { readonly children: ReactNode }) {
  const language = await chosenLanguage();
  const t = texts(application.menu, language);
  return (
    <html lang={language}>
      <body>
        <Menu texts={t} />
        <main className="screens-main">
          <div className="screens-language">
            <LanguageChoice languages={languages} chosen={language} cookie={languageCookie} texts={t} />
          </div>
          <Notice />
          {children}
        </main>
      </body>
    </html>
  );
}
