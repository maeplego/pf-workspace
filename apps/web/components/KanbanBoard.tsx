"use client";

import {
  DndContext,
  DragOverlay,
  PointerSensor,
  closestCorners,
  useDroppable,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragStartEvent,
} from "@dnd-kit/core";
import { SortableContext, useSortable, verticalListSortingStrategy } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMemo, useState, useTransition } from "react";

import { createCard, createColumn, deleteColumn, moveCard, renameColumn, reorderColumns, updateCardDetails } from "../app/actions";

export type CardView = {
  id: string;
  columnId: string;
  title: string;
  description: string;
  version: number;
  sprintId?: string;
};

export type ColumnView = {
  id: string;
  name: string;
  cards: CardView[];
};

export type SprintOption = { id: string; name: string };

type Props = {
  boardId: string;
  boardName: string;
  workspaceName: string;
  columns: ColumnView[];
  sprints: SprintOption[];
  devUser?: string;
  readOnly: boolean;
};

function ColumnDrop({ id, children }: { id: string; children: React.ReactNode }) {
  const { setNodeRef, isOver } = useDroppable({ id: `col:${id}` });
  return (
    <div ref={setNodeRef} data-column-id={id} style={{ minHeight: 72, background: isOver ? "#dde1e6" : undefined, borderRadius: 4 }}>
      {children}
    </div>
  );
}

function SortableCard({
  card,
  readOnly,
  onSelect,
}: {
  card: CardView;
  readOnly: boolean;
  onSelect: (c: CardView) => void;
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: card.id,
    disabled: readOnly,
  });
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
    background: "#fff",
    border: "1px solid #dfe1e6",
    borderRadius: 6,
    padding: "0.6rem 0.75rem",
    marginBottom: "0.5rem",
    cursor: readOnly ? "pointer" : "grab",
  };
  return (
    <div ref={setNodeRef} style={style}>
      <div style={{ display: "flex", gap: "0.35rem", alignItems: "flex-start" }}>
        {!readOnly ? (
          <button type="button" aria-label="ドラッグして列を変更" {...attributes} {...listeners} style={{ cursor: "grab", border: "none", background: "transparent", padding: 0 }}>
            ⋮⋮
          </button>
        ) : null}
        <button type="button" onClick={() => onSelect(card)} style={{ flex: 1, textAlign: "left", border: "none", background: "transparent", padding: 0, cursor: "pointer" }}>
          {card.title}
        </button>
      </div>
    </div>
  );
}

export function KanbanBoard({ boardId, boardName, workspaceName, columns, sprints, devUser, readOnly }: Props) {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [activeId, setActiveId] = useState<string | null>(null);
  const [selected, setSelected] = useState<CardView | null>(null);
  const [newTitles, setNewTitles] = useState<Record<string, string>>({});
  const [columnNames, setColumnNames] = useState<Record<string, string>>({});
  const [newColumnName, setNewColumnName] = useState("");

  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 6 } }));

  const cardMap = useMemo(() => {
    const map = new Map<string, CardView>();
    for (const col of columns) {
      for (const card of col.cards) {
        map.set(card.id, card);
      }
    }
    return map;
  }, [columns]);

  const columnIds = columns.map((c) => c.id);

  function columnTitle(col: ColumnView) {
    return columnNames[col.id] ?? col.name;
  }

  function saveColumnName(columnId: string) {
    const name = (columnNames[columnId] ?? columns.find((c) => c.id === columnId)?.name ?? "").trim();
    if (!name) return;
    startTransition(async () => {
      await renameColumn(boardId, columnId, name, devUser);
      router.refresh();
    });
  }

  function removeColumn(columnId: string) {
    const col = columns.find((c) => c.id === columnId);
    if (!col) return;
    if (col.cards.length > 0) {
      alert("カードがある列は削除できません");
      return;
    }
    if (columns.length <= 1) {
      alert("最後の列は削除できません");
      return;
    }
    startTransition(async () => {
      try {
        await deleteColumn(boardId, columnId, devUser);
        router.refresh();
      } catch (e) {
        console.error(e);
        alert("列の削除に失敗しました");
        router.refresh();
      }
    });
  }

  function moveColumn(columnId: string, dir: -1 | 1) {
    const idx = columnIds.indexOf(columnId);
    const next = idx + dir;
    if (idx < 0 || next < 0 || next >= columnIds.length) return;
    const ids = [...columnIds];
    [ids[idx], ids[next]] = [ids[next], ids[idx]];
    startTransition(async () => {
      await reorderColumns(boardId, ids, devUser);
      router.refresh();
    });
  }

  function addColumn() {
    const name = newColumnName.trim();
    if (!name) return;
    startTransition(async () => {
      await createColumn(boardId, name, devUser);
      setNewColumnName("");
      router.refresh();
    });
  }

  function onDragStart(event: DragStartEvent) {
    setActiveId(String(event.active.id));
  }

  function onDragEnd(event: DragEndEvent) {
    setActiveId(null);
    if (readOnly) return;
    const { active, over } = event;
    if (!over) return;

    const cardId = String(active.id);
    const card = cardMap.get(cardId);
    if (!card) return;

    let targetColumnId = String(over.id);
    let position = 0;

    if (targetColumnId.startsWith("col:")) {
      targetColumnId = targetColumnId.slice(4);
    }

    if (columnIds.includes(targetColumnId)) {
      const col = columns.find((c) => c.id === targetColumnId);
      position = col?.cards.length ?? 0;
    } else {
      const overCard = cardMap.get(String(over.id));
      if (!overCard) return;
      targetColumnId = overCard.columnId;
      const col = columns.find((c) => c.id === targetColumnId);
      position = col?.cards.findIndex((c) => c.id === overCard.id) ?? 0;
    }

    if (card.columnId === targetColumnId) {
      const col = columns.find((c) => c.id === targetColumnId);
      const currentIndex = col?.cards.findIndex((c) => c.id === cardId) ?? -1;
      if (currentIndex === position || currentIndex === position - 1) return;
    }

    startTransition(async () => {
      try {
        await moveCard(boardId, cardId, targetColumnId, position, card.version, devUser);
        router.refresh();
      } catch (e) {
        console.error(e);
        alert("移動に失敗しました（バージョン競合の可能性）");
        router.refresh();
      }
    });
  }

  function addCard(columnId: string) {
    const title = (newTitles[columnId] || "").trim();
    if (!title) return;
    startTransition(async () => {
      await createCard(columnId, title, devUser);
      setNewTitles((prev) => ({ ...prev, [columnId]: "" }));
      router.refresh();
    });
  }

  function saveSelected() {
    if (!selected) return;
    const current = cardMap.get(selected.id);
    startTransition(async () => {
      try {
        if (current && current.columnId !== selected.columnId) {
          await moveCard(boardId, selected.id, selected.columnId, 0, selected.version, devUser);
        } else {
          await updateCardDetails(
            boardId,
            selected.id,
            selected.title,
            selected.description,
            selected.version,
            selected.sprintId || "",
            devUser,
          );
        }
        setSelected(null);
        router.refresh();
      } catch (e) {
        console.error(e);
        alert("保存に失敗しました（バージョン競合の可能性）");
        router.refresh();
      }
    });
  }

  const activeCard = activeId ? cardMap.get(activeId) : null;

  return (
    <div>
      <header style={{ marginBottom: "1rem" }}>
        <p className="muted" style={{ margin: 0 }}>
          <Link href={devUser ? `/?user=${devUser}` : "/"}>{workspaceName}</Link> / {boardName}
          {" · "}
          <Link href={devUser ? `/boards/${boardId}/sprints?user=${devUser}` : `/boards/${boardId}/sprints`}>スプリント</Link>
          {readOnly ? " · 閲覧のみ" : null}
          {pending ? " · 保存中…" : null}
        </p>
        <h1 style={{ margin: "0.25rem 0 0" }}>{boardName}</h1>
      </header>

      <DndContext sensors={sensors} collisionDetection={closestCorners} onDragStart={onDragStart} onDragEnd={onDragEnd}>
        <div style={{ display: "flex", gap: "1rem", alignItems: "flex-start", overflowX: "auto" }}>
          {columns.map((col) => (
            <ColumnDrop key={col.id} id={col.id}>
              <div
                style={{
                  minWidth: 260,
                  background: "#ebecf0",
                  borderRadius: 8,
                  padding: "0.75rem",
                }}
              >
                <div style={{ display: "flex", gap: "0.35rem", alignItems: "center", marginBottom: "0.75rem" }}>
                  {!readOnly ? (
                    <>
                      <input
                        value={columnTitle(col)}
                        onChange={(e) => setColumnNames((prev) => ({ ...prev, [col.id]: e.target.value }))}
                        onBlur={() => saveColumnName(col.id)}
                        onKeyDown={(e) => {
                          if (e.key === "Enter") {
                            e.currentTarget.blur();
                          }
                        }}
                        style={{ flex: 1, fontSize: "0.95rem", fontWeight: 600, padding: "0.2rem 0.35rem" }}
                        aria-label="列名"
                      />
                      <button type="button" className="btn btn-secondary" onClick={() => moveColumn(col.id, -1)} disabled={pending || columnIds[0] === col.id} title="左へ">
                        ←
                      </button>
                      <button type="button" className="btn btn-secondary" onClick={() => moveColumn(col.id, 1)} disabled={pending || columnIds[columnIds.length - 1] === col.id} title="右へ">
                        →
                      </button>
                      <button type="button" className="btn btn-secondary" onClick={() => removeColumn(col.id)} disabled={pending} title="列を削除">
                        ×
                      </button>
                    </>
                  ) : (
                    <h3 style={{ margin: 0, fontSize: "0.95rem" }}>{col.name}</h3>
                  )}
                </div>
                <SortableContext items={col.cards.map((c) => c.id)} strategy={verticalListSortingStrategy}>
                  {col.cards.map((card) => (
                    <SortableCard key={card.id} card={card} readOnly={readOnly} onSelect={setSelected} />
                  ))}
                </SortableContext>
                {!readOnly ? (
                  <div style={{ display: "flex", gap: "0.35rem", marginTop: "0.5rem" }}>
                    <input
                      value={newTitles[col.id] || ""}
                      onChange={(e) => setNewTitles((prev) => ({ ...prev, [col.id]: e.target.value }))}
                      placeholder="New card"
                      style={{ flex: 1, padding: "0.35rem 0.5rem", fontSize: "0.85rem" }}
                      onKeyDown={(e) => {
                        if (e.key === "Enter") addCard(col.id);
                      }}
                    />
                    <button type="button" onClick={() => addCard(col.id)} disabled={pending}>
                      +
                    </button>
                  </div>
                ) : null}
              </div>
            </ColumnDrop>
          ))}
          {!readOnly ? (
            <div style={{ minWidth: 220, background: "#ebecf0", borderRadius: 8, padding: "0.75rem" }}>
              <p style={{ margin: "0 0 0.5rem", fontSize: "0.9rem" }}>列を追加</p>
              <div style={{ display: "flex", gap: "0.35rem" }}>
                <input
                  value={newColumnName}
                  onChange={(e) => setNewColumnName(e.target.value)}
                  placeholder="列名"
                  style={{ flex: 1, padding: "0.35rem 0.5rem", fontSize: "0.85rem" }}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") addColumn();
                  }}
                />
                <button type="button" onClick={addColumn} disabled={pending}>
                  +
                </button>
              </div>
            </div>
          ) : null}
        </div>
        <DragOverlay>
          {activeCard ? (
            <div style={{ padding: "0.6rem", background: "#fff", border: "1px solid #ccc" }}>{activeCard.title}</div>
          ) : null}
        </DragOverlay>
      </DndContext>

      {selected && !readOnly ? (
        <div
          role="dialog"
          style={{
            position: "fixed",
            inset: 0,
            background: "rgba(0,0,0,0.35)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            padding: "1rem",
          }}
          onClick={() => setSelected(null)}
        >
          <div className="card-surface" style={{ width: "100%", maxWidth: 480 }} onClick={(e) => e.stopPropagation()}>
            <h2 style={{ marginTop: 0 }}>カード詳細</h2>
            <label style={{ display: "block", marginBottom: "0.75rem" }}>
              タイトル
              <input
                value={selected.title}
                onChange={(e) => setSelected({ ...selected, title: e.target.value })}
                style={{ display: "block", width: "100%", marginTop: "0.25rem", padding: "0.5rem" }}
              />
            </label>
            <label style={{ display: "block", marginBottom: "0.75rem" }}>
              説明
              <textarea
                value={selected.description}
                onChange={(e) => setSelected({ ...selected, description: e.target.value })}
                rows={4}
                style={{ display: "block", width: "100%", marginTop: "0.25rem", padding: "0.5rem" }}
              />
            </label>
            <label style={{ display: "block", marginBottom: "0.75rem" }}>
              列
              <select
                value={selected.columnId}
                onChange={(e) => setSelected({ ...selected, columnId: e.target.value })}
                style={{ display: "block", width: "100%", marginTop: "0.25rem", padding: "0.5rem" }}
              >
                {columns.map((col) => (
                  <option key={col.id} value={col.id}>
                    {col.name}
                  </option>
                ))}
              </select>
            </label>
            <label style={{ display: "block", marginBottom: "0.75rem" }}>
              スプリント
              <select
                value={selected.sprintId || ""}
                onChange={(e) => setSelected({ ...selected, sprintId: e.target.value })}
                style={{ display: "block", width: "100%", marginTop: "0.25rem", padding: "0.5rem" }}
              >
                <option value="">（なし）</option>
                {sprints.map((sp) => (
                  <option key={sp.id} value={sp.id}>
                    {sp.name}
                  </option>
                ))}
              </select>
            </label>
            <div style={{ display: "flex", gap: "0.5rem" }}>
              <button type="button" onClick={saveSelected} disabled={pending}>
                保存
              </button>
              <button type="button" onClick={() => setSelected(null)}>
                閉じる
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}
