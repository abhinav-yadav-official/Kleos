import "./globals.css";
import type { Metadata } from "next";
import { ReactNode } from "react";

export const metadata: Metadata = {
  title: "Kleos",
  description: "Automated job outreach — scrape, find, generate, send.",
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body className="min-h-full">
        <div className="mx-auto max-w-6xl px-6 py-6">{children}</div>
      </body>
    </html>
  );
}
