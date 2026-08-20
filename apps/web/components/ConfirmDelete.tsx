"use client";

import { useTransition } from "react";

export function ConfirmDelete({
  label,
  message,
  onConfirm,
}: {
  label: string;
  message: string;
  onConfirm: () => Promise<void>;
}) {
  const [pending, start] = useTransition();
  return (
    <button
      type="button"
      className="btn btn-secondary"
      disabled={pending}
      onClick={() => {
        if (!window.confirm(message)) return;
        start(async () => {
          await onConfirm();
        });
      }}
    >
      {label}
    </button>
  );
}
