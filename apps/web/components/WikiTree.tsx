export type PageNode = {
  id: string;
  parentId?: string;
  title: string;
  status: string;
  position: number;
  children: PageNode[];
};

function href(workspaceId: string, pageId: string, devUser?: string) {
  const base = `/wiki/${workspaceId}/pages/${pageId}`;
  return devUser ? `${base}?user=${encodeURIComponent(devUser)}` : base;
}

function TreeItems({
  nodes,
  workspaceId,
  currentId,
  devUser,
}: {
  nodes: PageNode[];
  workspaceId: string;
  currentId?: string;
  devUser?: string;
}) {
  return (
    <ul style={{ listStyle: "none", paddingLeft: "0.9rem", margin: 0 }}>
      {nodes.map((n) => (
        <li key={n.id} style={{ margin: "0.2rem 0" }}>
          <a href={href(workspaceId, n.id, devUser)} style={{ fontWeight: n.id === currentId ? 700 : 400 }}>
            {n.title}
          </a>
          {n.status === "draft" ? <span className="muted"> draft</span> : null}
          {n.children?.length ? (
            <TreeItems nodes={n.children} workspaceId={workspaceId} currentId={currentId} devUser={devUser} />
          ) : null}
        </li>
      ))}
    </ul>
  );
}

export function WikiTree({
  nodes,
  workspaceId,
  currentId,
  devUser,
}: {
  nodes: PageNode[];
  workspaceId: string;
  currentId?: string;
  devUser?: string;
}) {
  if (!nodes.length) {
    return <p className="muted">ページがありません。</p>;
  }
  return <TreeItems nodes={nodes} workspaceId={workspaceId} currentId={currentId} devUser={devUser} />;
}
