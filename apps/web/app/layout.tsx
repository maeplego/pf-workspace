import type { Metadata } from "next";

import "./globals.css";

export const metadata: Metadata = {
  title: "Workspace",
  description: "P04 team workspace — kanban MVP",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="ja">
      <body>{children}</body>
    </html>
  );
}
