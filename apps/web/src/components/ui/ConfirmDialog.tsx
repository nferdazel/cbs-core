"use client";

import React, { useEffect, useState } from "react";
import { Button } from "./Button";

export interface ConfirmDialogProps {
  open: boolean;
  title: string;
  /** Ringkasan konsekuensi tindakan. */
  description?: React.ReactNode;
  confirmLabel?: string;
  cancelLabel?: string;
  destructive?: boolean;
  loading?: boolean;
  /**
   * Bila diisi, pengguna harus mengetik teks ini persis sebelum tombol konfirmasi
   * aktif. Untuk aksi yang tidak dapat dibatalkan (mis. tutup buku tahunan).
   */
  requireKeyword?: string;
  onConfirm: () => void;
  onCancel: () => void;
}

/**
 * Dialog konfirmasi untuk aksi yang tidak bisa dibatalkan (aksi finansial).
 * Bisa ditutup dengan Escape dan klik latar. Satu-satunya tempat yang boleh
 * memakai --shadow-overlay.
 */
export const ConfirmDialog: React.FC<ConfirmDialogProps> = ({
  open,
  title,
  description,
  confirmLabel = "Konfirmasi",
  cancelLabel = "Batal",
  destructive = false,
  loading = false,
  requireKeyword,
  onConfirm,
  onCancel,
}) => {
  const [keyword, setKeyword] = useState("");

  useEffect(() => {
    setKeyword("");
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !loading) onCancel();
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [open, loading, onCancel]);

  if (!open) return null;

  const keywordSatisfied = !requireKeyword || keyword === requireKeyword;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-ink-900/40 p-6"
      onClick={() => {
        if (!loading) onCancel();
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label={title}
        className="w-full max-w-md rounded-md border border-border bg-surface shadow-overlay"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="border-b border-border px-4 py-3">
          <h2 className="text-title font-semibold text-ink-900">{title}</h2>
        </div>
        {(description || requireKeyword) && (
          <div className="space-y-2 px-4 py-4 text-body text-ink-600">
            {description}
            {requireKeyword && (
              <div>
                <label
                  htmlFor="confirm-keyword"
                  className="text-meta font-medium text-ink-900"
                >
                  Ketik <span className="font-mono">{requireKeyword}</span> untuk
                  mengonfirmasi
                </label>
                <input
                  id="confirm-keyword"
                  type="text"
                  value={keyword}
                  onChange={(event) => setKeyword(event.target.value)}
                  autoFocus
                  autoComplete="off"
                  disabled={loading}
                  className="mt-1 h-9 w-full rounded-md border border-border-strong bg-surface px-2 font-mono text-body text-ink-900 focus:border-navy-600 disabled:opacity-50"
                />
              </div>
            )}
          </div>
        )}
        <div className="flex justify-end gap-2 border-t border-border px-4 py-3">
          <Button variant="secondary" onClick={onCancel} disabled={loading}>
            {cancelLabel}
          </Button>
          <Button
            autoFocus={!requireKeyword}
            variant={destructive ? "danger" : "primary"}
            onClick={onConfirm}
            loading={loading}
            disabled={!keywordSatisfied}
          >
            {confirmLabel}
          </Button>
        </div>
      </div>
    </div>
  );
};
