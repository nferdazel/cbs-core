"use client";

import { useState } from "react";
import { ApiError, request } from "@/lib/api";
import type { AccountRecord } from "@/lib/types";
import { useAuth } from "@/lib/useAuth";
import { hasPermission } from "@/lib/permissions";
import { useTranslation } from "@/i18n/context";
import { Button } from "@/components/ui/Button";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { Input } from "@/components/ui/Input";
import { StatusBadge } from "@/components/ui/StatusBadge";

export interface AccountFreezeProps {
  account: AccountRecord;
  /**
   * Dipanggil dengan data rekening terbaru dari server setelah aksi sukses,
   * beserta pesan umpan balik siap tampil (sudah lewat kamus).
   */
  onChanged: (account: AccountRecord, message: string) => void;
}

/**
 * Tombol Bekukan (ACTIVE -> FROZEN) dan Batalkan Pembekuan (FROZEN -> ACTIVE).
 * Hanya dirender untuk status yang relevan dan peran berwenang. Selalu lewat
 * konfirmasi, dan pesan galat server (mis. 422 "pembekuan tidak dapat dibatalkan
 * oleh pelaksana pembekuan yang sama", 403 izin/lintas cabang) ditampilkan apa
 * adanya di dalam dialog sehingga pengguna tahu sebab sebenarnya.
 */
export function AccountFreeze({ account, onChanged }: AccountFreezeProps) {
  const { user } = useAuth();
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [notes, setNotes] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Tombol hanya tampil bila pengguna memegang izin accounts:freeze dari
  // GET /auth/me (bukan salinan peran). API tetap penjaga sebenarnya.
  if (
    (account.status !== "ACTIVE" && account.status !== "FROZEN") ||
    !hasPermission(user, "accounts:freeze")
  ) {
    return null;
  }

  const freezing = account.status === "ACTIVE";
  const endpoint = freezing ? "freeze" : "unfreeze";
  const targetStatus = freezing ? "FROZEN" : "ACTIVE";

  const openDialog = () => {
    setError(null);
    setNotes("");
    setOpen(true);
  };

  const submit = async () => {
    setSubmitting(true);
    setError(null);
    try {
      const response = await request<AccountRecord>(
        `/accounts/${encodeURIComponent(account.account_number)}/${endpoint}`,
        { method: "POST", body: { notes: notes.trim() || undefined } },
      );
      const updated = response.data ?? { ...account, status: targetStatus };
      setOpen(false);
      setNotes("");
      onChanged(
        updated,
        freezing
          ? t.accountFreeze.frozenFeedback
          : t.accountFreeze.unfrozenFeedback,
      );
    } catch (err) {
      // Pesan pelanggaran (mis. membatalkan pembekuan sendiri) datang dari API;
      // tampilkan apa adanya, jangan diterjemahkan ulang atau ditelan.
      setError(
        err instanceof ApiError
          ? err.message
          : freezing
            ? t.accountFreeze.freezeError
            : t.accountFreeze.unfreezeError,
      );
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <>
      <Button
        size="sm"
        variant={freezing ? "danger" : "secondary"}
        onClick={openDialog}
      >
        {freezing
          ? t.accountFreeze.freezeButton
          : t.accountFreeze.unfreezeButton}
      </Button>
      <ConfirmDialog
        open={open}
        title={
          freezing ? t.accountFreeze.freezeTitle : t.accountFreeze.unfreezeTitle
        }
        confirmLabel={
          freezing
            ? t.accountFreeze.freezeConfirm
            : t.accountFreeze.unfreezeConfirm
        }
        destructive={freezing}
        loading={submitting}
        onConfirm={submit}
        onCancel={() => setOpen(false)}
        description={
          <>
            <p className="flex flex-wrap items-center gap-1">
              <span>{t.accountFreeze.descAccount}</span>
              <span className="font-mono">{account.account_number}</span>
              <span>
                {freezing
                  ? t.accountFreeze.descFreezeFrom
                  : t.accountFreeze.descUnfreezeFrom}
              </span>
              <StatusBadge status={account.status} domain="account" />
              <span>
                {freezing
                  ? t.accountFreeze.descFreezeTo
                  : t.accountFreeze.descUnfreezeTo}
              </span>
              <StatusBadge status={targetStatus} domain="account" />
            </p>
            <p>
              {freezing
                ? t.accountFreeze.descFreezeSuffix
                : t.accountFreeze.descUnfreezeSuffix}
            </p>
            <Input
              label={t.accountFreeze.notesLabel}
              value={notes}
              onChange={(event) => setNotes(event.target.value)}
              placeholder={t.accountFreeze.notesPlaceholder}
            />
            {error && (
              <p role="alert" className="text-meta text-debit-700">
                {error}
              </p>
            )}
          </>
        }
      />
    </>
  );
}
