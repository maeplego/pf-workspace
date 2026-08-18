import Markdown from "react-markdown";

export function MarkdownPreview({ source }: { source: string }) {
  return (
    <div className="wiki-preview">
      <Markdown
        skipHtml
        components={{
          a({ href, children }) {
            const ok = href && /^(https?:|mailto:|#)/i.test(href);
            return ok ? <a href={href}>{children}</a> : <span>{children}</span>;
          },
        }}
      >
        {source}
      </Markdown>
    </div>
  );
}
