import type { Metadata } from "next";

import "./globals.css";

export const metadata: Metadata = {
  title: "Workspace",
  description: "P04 team workspace — kanban MVP",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="ja">
      <body>
        <div className="site-shell">
          <header className="site-header">
            <div className="site-brand">
              <strong>Workspace</strong>
              <span className="muted">P04 team workspace — kanban MVP</span>
            </div>
            <nav className="site-nav">
              <a href="/">ホーム</a>
            </nav>
          </header>
          <main className="site-main">{children}</main>
        </div>
      </body>
    </html>
  );
}
