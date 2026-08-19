"use client";

import { useRouter } from "next/navigation";
import { useMemo, useState, useTransition } from "react";

import { createSprint } from "../app/actions";

export type SprintView = {
  id: string;
  name: string;
  startAt: string;
  endAt: string;
};

export type BurndownView = {
  sprintId: string;
  unit: string;
  points: { date: string; remaining: number }[];
};

type Props = {
  boardId: string;
  sprints: SprintView[];
  burndowns: Record<string, BurndownView>;
  readOnly: boolean;
  devUser?: string;
};

function Chart({ points }: { points: { date: string; remaining: number }[] }) {
  const w = 420;
  const h = 160;
  const pad = 28;
  const max = Math.max(1, ...points.map((p) => p.remaining));
  const coords = points.map((p, i) => {
    const x = pad + (i / Math.max(1, points.length - 1)) * (w - pad * 2);
    const y = h - pad - (p.remaining / max) * (h - pad * 2);
    return `${x},${y}`;
  });
  return (
    <svg className="burndown-chart" viewBox={`0 0 ${w} ${h}`} role="img" aria-label="burndown">
      <polyline fill="none" stroke="#0052cc" strokeWidth="2" points={coords.join(" ")} />
      {points.map((p, i) => {
        const x = pad + (i / Math.max(1, points.length - 1)) * (w - pad * 2);
        const y = h - pad - (p.remaining / max) * (h - pad * 2);
        return (
          <g key={p.date}>
            <circle cx={x} cy={y} r="3" fill="#0052cc" />
            {i === 0 || i === points.length - 1 ? (
              <text x={x} y={h - 8} textAnchor="middle" fontSize="10" fill="#6b778c">
                {p.date.slice(5)}
              </text>
            ) : null}
          </g>
        );
      })}
      <text x={8} y={16} fontSize="11" fill="#6b778c">
        {max} cards
      </text>
    </svg>
  );
}

export function SprintPanel({ boardId, sprints, burndowns, readOnly, devUser }: Props) {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [error, setError] = useState("");
  const [selected, setSelected] = useState(sprints[0]?.id || "");
  const current = useMemo(() => sprints.find((s) => s.id === selected) || sprints[0], [sprints, selected]);
  const chart = current ? burndowns[current.id] : undefined;

  function onCreate(formData: FormData) {
    startTransition(async () => {
      const msg = await createSprint(boardId, formData, devUser);
      if (typeof msg === "string") {
        setError(msg);
        return;
      }
      setError("");
      router.refresh();
    });
  }

  return (
    <div>
      {!readOnly ? (
        <form action={onCreate} className="card-surface" style={{ marginBottom: "1rem", display: "grid", gap: "0.5rem" }}>
          <h2 style={{ margin: 0, fontSize: "1rem" }}>スプリントを作成</h2>
          {error ? <p style={{ color: "#bf2600", margin: 0 }}>{error}</p> : null}
          <input name="name" placeholder="Sprint name" required style={{ padding: "0.4rem" }} />
          <label className="muted">
            開始（UTC に変換）
            <input type="datetime-local" name="startAt" required style={{ display: "block", marginTop: "0.25rem" }} />
          </label>
          <label className="muted">
            終了
            <input type="datetime-local" name="endAt" required style={{ display: "block", marginTop: "0.25rem" }} />
          </label>
          <button type="submit" disabled={pending}>
            作成
          </button>
        </form>
      ) : (
        <p className="muted">閲覧のみ</p>
      )}

      {sprints.length === 0 ? (
        <p className="muted">スプリントがありません。</p>
      ) : (
        <div className="card-surface">
          <label className="muted">
            表示するスプリント
            <select
              value={current?.id || ""}
              onChange={(e) => setSelected(e.target.value)}
              style={{ display: "block", marginTop: "0.35rem", padding: "0.35rem" }}
            >
              {sprints.map((sp) => (
                <option key={sp.id} value={sp.id}>
                  {sp.name}
                </option>
              ))}
            </select>
          </label>
          {current ? (
            <p className="muted">
              {new Date(current.startAt).toISOString()} → {new Date(current.endAt).toISOString()}
            </p>
          ) : null}
          {chart && chart.points.length > 0 ? (
            <>
              <Chart points={chart.points} />
              <table className="burndown-table">
                <thead>
                  <tr>
                    <th>日付 (UTC)</th>
                    <th>未完了カード</th>
                  </tr>
                </thead>
                <tbody>
                  {chart.points.map((p) => (
                    <tr key={p.date}>
                      <td>{p.date}</td>
                      <td>{p.remaining}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </>
          ) : (
            <p className="muted">バーンダウン点がありません。</p>
          )}
        </div>
      )}
    </div>
  );
}
