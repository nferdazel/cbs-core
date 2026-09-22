"use client";

import { useState } from "react";
import { ApiError, request } from "@/lib/api";
import type { AccountRecord } from "@/lib/types";
import { useAuth } from "@/lib/useAuth";
import { useTranslation } from "@/i18n/context";
import { Button } from "@/components/ui/Button";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { Input } from "@/components/ui/Input";

/**
 * Peran berwenang memegang accounts:freeze menurut domain/staff.go
 * (RolePermissions): SUPERADMIN, ADMIN, SUPERVISOR. Backend menegakkannya lewat
 * middleware RequirePermission, jadi tombol disembunyikan untuk peran lain agar
 * tidak ada kontrol yang pasti gagal.
 */
const REACTIVATION_ROLES = new Set(["SUPERADMIN", "ADMIN", "SUPERVISOR"]);

export function canReactivateAccount(role: string | null | undefined): boolean {
  return role ? REACTIVATION_ROLES.has(role) : false;
}

export interface AccountReactivationProps {
  account: AccountRecord;
  /** Dipanggil dengan data rekening terbaru dari server setelah reaktivasi sukses. */
  onReactivated: (account: AccountRecord) => void;
}

/**
 * Aksi reaktivasi rekening DORMANT. Hanya dirender untuk rekening DORMANT dan
 * peran berwenang. Selalu lewat konfirmasi, dan pesan galat server (403 izin /
 * lintas cabang, 422 bukan dormant) ditampilkan apa adanya di dalam dialog.
 */
export function AccountReactivation({
  account,
  onReactivated,
}: AccountReactivationProps) {
  const { user } = useAuth();
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [notes, setNotes] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (account.status !== "DORMANT" || !canReactivateAccount(user?.role)) {
    return null;
  }

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
        `/accounts/${encodeURIComponent(account.account_number)}/reactivate`,
        { method: "POST", body: { notes: notes.trim() || undefined } }
      );
      setOpen(false);
      setNotes("");
      onReactivated(response.data ?? { ...account, status: "ACTIVE" });
    } catch (err) {
      setError(
        err instanceof ApiError ? err.message : t.reactivation.error
      );
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <>
      <Button size="sm" variant="secondary" onClick={openDialog}>
        {t.reactivation.button}
      </Button>
      <ConfirmDialog
        open={open}
        title={t.reactivation.title}
        confirmLabel={t.reactivation.confirm}
        loading={submitting}
        onConfirm={submit}
        onCancel={() => setOpen(false)}
        description={
          <>
            <p>
              {t.reactivation.descAccount}{" "}
              <span className="font-mono">{account.account_number}</span>{" "}
              {t.reactivation.descRestoredFrom} <strong>DORMANT</strong>{" "}
              {t.reactivation.descTo} <strong>ACTIVE</strong>
              {t.reactivation.descSuffix}
            </p>
            <Input
              label={t.reactivation.notesLabel}
              value={notes}
              onChange={(event) => setNotes(event.target.value)}
              placeholder={t.reactivation.notesPlaceholder}
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
