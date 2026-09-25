"use client";

import React, { useCallback, useEffect, useMemo, useState } from "react";
import { ApiError, request } from "@/lib/api";
import { useAuth } from "@/lib/useAuth";
import { hasPermission } from "@/lib/permissions";
import { useTranslation } from "@/i18n/context";
import type { Dictionary } from "@/i18n/dictionaries/id";
import type {
  CreateStaffInput,
  StaffRole,
  StaffUser,
  UpdateStaffInput,
} from "@/lib/types";
import type { COABook } from "@/lib/operations-types";
import { Alert } from "@/components/ui/Alert";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardContent } from "@/components/ui/Card";
import { DataTable, type Column } from "@/components/ui/DataTable";
import { Input } from "@/components/ui/Input";
import { Pagination } from "@/components/ui/Pagination";
import { Select, type SelectOption } from "@/components/ui/Select";
import { ErrorState } from "@/components/ui/States";

const PAGE_SIZE = 20;

/**
 * Peran yang boleh diberikan lewat formulir. SUPERADMIN dan SYSTEM sengaja tidak
 * ada: API menolaknya (ErrStaffPrivilegedCreate / ErrStaffPrivilegedRole), jadi
 * menawarkannya hanya akan menghasilkan penolakan yang membingungkan.
 */
const ASSIGNABLE_ROLES: StaffRole[] = [
  "ADMIN",
  "SUPERVISOR",
  "TELLER",
  "CS",
  "AO",
  "AUDITOR",
];

interface Meta {
  page: number;
  page_size: number;
  total_items: number;
  total_pages: number;
}

function roleLabel(t: Dictionary, value: string): string {
  switch (value) {
    case "ADMIN":
      return t.staffPage.roleAdmin;
    case "SUPERVISOR":
      return t.staffPage.roleSupervisor;
    case "TELLER":
      return t.staffPage.roleTeller;
    case "CS":
      return t.staffPage.roleCustomerService;
    case "AO":
      return t.staffPage.roleAccountOfficer;
    case "AUDITOR":
      return t.staffPage.roleAuditor;
    default:
      return value || t.staffPage.roleUnknown;
  }
}

function bookLabel(t: Dictionary, value?: string): string {
  if (value === "CONVENTIONAL") return t.staffPage.bookConventional;
  if (value === "SYARIAH") return t.staffPage.bookSyariah;
  return t.staffPage.bookUnset;
}

/**
 * Pemilih buku hanya menawarkan lini usaha yang aktif di instalasi. Cakupan
 * dibaca dari server (/auth/me -> active_books); web tidak menebak. Buku yang
 * sedang dipegang akun tetap disertakan agar data lama tidak terkunci di luar
 * daftar. Bila active_books belum tersedia, kedua buku ditawarkan apa adanya.
 */
function bookSelectOptions(
  t: Dictionary,
  activeBooks?: string[],
  include?: string,
): SelectOption[] {
  const all: COABook[] = ["CONVENTIONAL", "SYARIAH"];
  return all
    .filter(
      (value) =>
        !activeBooks || activeBooks.includes(value) || value === include,
    )
    .map((value) => ({ value, label: bookLabel(t, value) }));
}

function roleSelectOptions(t: Dictionary, include?: string): SelectOption[] {
  const options = ASSIGNABLE_ROLES.map((value) => ({
    value,
    label: roleLabel(t, value),
  }));
  if (include && !ASSIGNABLE_ROLES.includes(include as StaffRole)) {
    return [{ value: include, label: roleLabel(t, include) }, ...options];
  }
  return options;
}

/** Peran operasional wajib berunit kerja terdaftar; peran pengawas tidak. */
function requiresBranch(role: string): boolean {
  return role !== "AUDITOR" && role !== "SUPERADMIN" && role !== "SYSTEM";
}

function isValidEmail(value: string): boolean {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value);
}

function describeError(
  err: unknown,
  messages: { forbidden: string; fallback: string },
): string {
  if (err instanceof ApiError) {
    if (err.status === 403) return messages.forbidden;
    return err.message || messages.fallback;
  }
  return messages.fallback;
}

export function StaffManagement() {
  const { t } = useTranslation();
  const { user } = useAuth();

  const canRead = hasPermission(user, "users:read");
  const canCreate = hasPermission(user, "users:create");
  const canUpdate = hasPermission(user, "users:update");

  const [members, setMembers] = useState<StaffUser[]>([]);
  const [meta, setMeta] = useState<Meta | null>(null);
  const [page, setPage] = useState(1);
  const [reloadKey, setReloadKey] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const [createOpen, setCreateOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<StaffUser | null>(null);
  const [resetTarget, setResetTarget] = useState<StaffUser | null>(null);
  const [changeOwnOpen, setChangeOwnOpen] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams({
        page: String(page),
        page_size: String(PAGE_SIZE),
      });
      const response = await request<StaffUser[]>(
        `/staff?${params.toString()}`,
      );
      setMembers(response.data ?? []);
      setMeta((response.meta as Meta | undefined) ?? null);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setError(t.staffPage.restricted);
      } else {
        setError(err instanceof ApiError ? err.message : t.staffPage.loadError);
      }
    } finally {
      setLoading(false);
    }
  }, [page, t]);

  useEffect(() => {
    if (canRead) load();
    else setLoading(false);
  }, [load, reloadKey, canRead]);

  // Buku bawaan formulir tambah: ikuti buku pengelola bila buku itu aktif,
  // selain itu buku tunggal yang aktif di instalasi.
  const defaultBook = useMemo(() => {
    const active = user?.active_books;
    if (user?.book && (!active || active.includes(user.book))) return user.book;
    if (active && active.length === 1) return active[0];
    return active?.[0] ?? "";
  }, [user?.book, user?.active_books]);

  const afterMutation = (message: string) => {
    setNotice(message);
    setCreateOpen(false);
    setEditTarget(null);
    setResetTarget(null);
    setChangeOwnOpen(false);
    setReloadKey((key) => key + 1);
  };

  const columns: Column<StaffUser>[] = [
    {
      header: t.staffPage.colEmployeeId,
      accessorKey: "employee_id",
      isMono: true,
    },
    { header: t.staffPage.colUsername, accessorKey: "username", isMono: true },
    { header: t.staffPage.colName, accessorKey: "full_name" },
    { header: t.staffPage.colRole, cell: (row) => roleLabel(t, row.role) },
    {
      header: t.staffPage.colBranch,
      cell: (row) => row.branch_code || "-",
      isMono: true,
    },
    { header: t.staffPage.colBook, cell: (row) => bookLabel(t, row.book) },
    {
      header: t.staffPage.colActive,
      cell: (row) =>
        row.is_active ? (
          <Badge variant="credit">{t.staffPage.activeBadge}</Badge>
        ) : (
          <Badge variant="outline">{t.staffPage.inactiveBadge}</Badge>
        ),
    },
    {
      header: t.common.actions,
      cell: (row) => (
        <div className="flex items-center gap-2">
          {canUpdate && (
            <Button
              size="sm"
              variant="secondary"
              onClick={() => setEditTarget(row)}
            >
              {t.staffPage.editButton}
            </Button>
          )}
          {canUpdate && row.id !== user?.id && (
            <Button
              size="sm"
              variant="secondary"
              onClick={() => setResetTarget(row)}
            >
              {t.staffPage.resetButton}
            </Button>
          )}
        </div>
      ),
    },
  ];

  if (!canRead) {
    return (
      <ErrorState
        title={t.staffPage.restrictedTitle}
        description={t.staffPage.restricted}
      />
    );
  }

  return (
    <>
      <div className="mb-4 flex flex-wrap items-center justify-end gap-2">
        <Button variant="secondary" onClick={() => setChangeOwnOpen(true)}>
          {t.staffPage.changeOwnButton}
        </Button>
        {canCreate && (
          <Button onClick={() => setCreateOpen(true)}>
            {t.staffPage.createButton}
          </Button>
        )}
      </div>

      {notice && (
        <div className="mb-4">
          <Alert variant="success">{notice}</Alert>
        </div>
      )}

      {/* Keadaan muat/kosong/jumlah hasil diumumkan ke pembaca layar. */}
      <p className="sr-only" role="status" aria-live="polite">
        {loading
          ? t.staffPage.loading
          : error
            ? ""
            : members.length === 0
              ? t.staffPage.empty
              : `${meta?.total_items ?? members.length} ${t.staffPage.resultCount}`}
      </p>

      {error && !loading ? (
        <ErrorState
          title={t.staffPage.errorTitle}
          description={error}
          action={
            <Button
              variant="secondary"
              onClick={() => setReloadKey((key) => key + 1)}
            >
              {t.common.retry}
            </Button>
          }
        />
      ) : (
        <Card>
          <CardContent className="p-0">
            <DataTable
              columns={columns}
              data={members}
              keyExtractor={(row) => row.id}
              loading={loading}
              emptyMessage={t.staffPage.empty}
              zebra
            />
            {meta && (
              <Pagination
                page={meta.page}
                pageSize={meta.page_size}
                totalItems={meta.total_items}
                totalPages={meta.total_pages}
                onPageChange={(next) => setPage(next)}
              />
            )}
          </CardContent>
        </Card>
      )}

      {createOpen && (
        <StaffFormDialog
          mode="create"
          activeBooks={user?.active_books}
          defaultBook={defaultBook}
          onClose={() => setCreateOpen(false)}
          onSaved={afterMutation}
        />
      )}

      {editTarget && (
        <StaffFormDialog
          key={editTarget.id}
          mode="edit"
          member={editTarget}
          activeBooks={user?.active_books}
          defaultBook={defaultBook}
          isSelf={editTarget.id === user?.id}
          onClose={() => setEditTarget(null)}
          onSaved={afterMutation}
        />
      )}

      {resetTarget && (
        <ResetPasswordDialog
          key={resetTarget.id}
          member={resetTarget}
          onClose={() => setResetTarget(null)}
          onSaved={afterMutation}
        />
      )}

      {changeOwnOpen && (
        <ChangeOwnPasswordDialog
          onClose={() => setChangeOwnOpen(false)}
          onSaved={afterMutation}
        />
      )}
    </>
  );
}

interface StaffFormDialogProps {
  mode: "create" | "edit";
  member?: StaffUser;
  activeBooks?: string[];
  defaultBook: string;
  isSelf?: boolean;
  onClose: () => void;
  onSaved: (message: string) => void;
}

function StaffFormDialog({
  mode,
  member,
  activeBooks,
  defaultBook,
  isSelf = false,
  onClose,
  onSaved,
}: StaffFormDialogProps) {
  const { t } = useTranslation();
  const editing = mode === "edit" && member !== undefined;
  const initialRole = member?.role ?? "TELLER";
  const roleEditable =
    !editing || ASSIGNABLE_ROLES.includes(initialRole as StaffRole);

  const [username, setUsername] = useState(member?.username ?? "");
  const [fullName, setFullName] = useState(member?.full_name ?? "");
  const [email, setEmail] = useState(member?.email ?? "");
  const [role, setRole] = useState(initialRole);
  const [branchCode, setBranchCode] = useState(member?.branch_code ?? "");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [book, setBook] = useState(
    member?.book ?? (editing ? "" : defaultBook),
  );
  const [isActive, setIsActive] = useState(member?.is_active ?? true);
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  const bookOptions = bookSelectOptions(t, activeBooks, member?.book);

  const onSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    setFormError(null);

    const trimmedName = fullName.trim();
    const trimmedEmail = email.trim();
    const trimmedBranch = branchCode.trim().toUpperCase();

    if (editing) {
      if (!trimmedName || !trimmedEmail) {
        setFormError(t.staffPage.requiredFieldsEdit);
        return;
      }
    } else if (!username.trim() || !trimmedName || !trimmedEmail || !password) {
      setFormError(t.staffPage.requiredFields);
      return;
    }
    if (!isValidEmail(trimmedEmail)) {
      setFormError(t.staffPage.emailInvalid);
      return;
    }
    if (requiresBranch(role) && !trimmedBranch) {
      setFormError(t.staffPage.branchRequired);
      return;
    }
    if (!editing && password !== confirm) {
      setFormError(t.staffPage.passwordMismatch);
      return;
    }

    setSubmitting(true);
    try {
      if (editing && member) {
        const input: UpdateStaffInput = {
          full_name: trimmedName,
          email: trimmedEmail,
          branch_code: trimmedBranch,
          is_active: isActive,
        };
        if (roleEditable) input.role = role as StaffRole;
        if (book) input.book = book as COABook;
        await request(`/staff/${member.id}`, { method: "PUT", body: input });
        onSaved(t.staffPage.updateSuccess);
      } else {
        const input: CreateStaffInput = {
          username: username.trim().toLowerCase(),
          full_name: trimmedName,
          email: trimmedEmail,
          password,
          role: role as StaffRole,
          branch_code: trimmedBranch,
        };
        if (book) input.book = book as COABook;
        await request("/staff", { method: "POST", body: input });
        onSaved(t.staffPage.createSuccess);
      }
    } catch (err) {
      setFormError(
        describeError(
          err,
          editing
            ? {
                forbidden: t.staffPage.updateForbidden,
                fallback: t.staffPage.updateError,
              }
            : {
                forbidden: t.staffPage.createForbidden,
                fallback: t.staffPage.createError,
              },
        ),
      );
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <ModalShell
      title={editing ? t.staffPage.editTitle : t.staffPage.createTitle}
      onClose={onClose}
      busy={submitting}
    >
      <form onSubmit={onSubmit}>
        <div className="space-y-4 px-4 py-4">
          <Input
            label={t.staffPage.fieldUsername}
            value={username}
            onChange={(event) => setUsername(event.target.value)}
            helperText={t.staffPage.fieldUsernameHint}
            isMono
            autoComplete="off"
            disabled={editing || submitting}
            autoFocus={!editing}
          />
          <Input
            label={t.staffPage.fieldFullName}
            value={fullName}
            onChange={(event) => setFullName(event.target.value)}
            autoComplete="off"
            disabled={submitting}
            autoFocus={editing}
          />
          <Input
            label={t.staffPage.fieldEmail}
            type="email"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
            autoComplete="off"
            disabled={submitting}
          />
          <div className="grid grid-cols-2 gap-3">
            <Select
              label={t.staffPage.fieldRole}
              value={role}
              onChange={(event) => setRole(event.target.value)}
              placeholder={t.staffPage.fieldRolePlaceholder}
              disabled={!roleEditable || submitting}
              options={roleSelectOptions(t, initialRole)}
            />
            <Input
              label={t.staffPage.fieldBranch}
              value={branchCode}
              onChange={(event) => setBranchCode(event.target.value)}
              helperText={t.staffPage.fieldBranchHint}
              isMono
              autoComplete="off"
              disabled={submitting}
            />
          </div>
          <Select
            label={t.staffPage.fieldBook}
            value={book}
            onChange={(event) => setBook(event.target.value)}
            helperText={t.staffPage.fieldBookHint}
            placeholder={t.staffPage.fieldBookPlaceholder}
            disabled={submitting || bookOptions.length === 0}
            options={bookOptions}
          />
          {!roleEditable && (
            <p className="text-meta text-ink-600">
              {t.staffPage.fieldRoleLocked}
            </p>
          )}
          {!editing ? (
            <div className="grid grid-cols-2 gap-3">
              <Input
                label={t.staffPage.fieldPassword}
                type="password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                helperText={t.staffPage.fieldPasswordHint}
                autoComplete="new-password"
                disabled={submitting}
              />
              <Input
                label={t.staffPage.fieldPasswordConfirm}
                type="password"
                value={confirm}
                onChange={(event) => setConfirm(event.target.value)}
                autoComplete="new-password"
                disabled={submitting}
              />
            </div>
          ) : (
            <label className="flex items-center gap-2 text-body text-ink-900">
              <input
                type="checkbox"
                className="h-4 w-4 rounded-sm border border-border-strong"
                checked={isActive}
                disabled={isSelf || submitting}
                onChange={(event) => setIsActive(event.target.checked)}
              />
              <span>{t.staffPage.fieldActive}</span>
              {isSelf && (
                <span className="text-meta text-ink-600">
                  {t.staffPage.fieldActiveSelfHint}
                </span>
              )}
            </label>
          )}
          {formError && <Alert variant="error">{formError}</Alert>}
        </div>
        <div className="flex justify-end gap-2 border-t border-border px-4 py-3">
          <Button
            variant="secondary"
            type="button"
            onClick={onClose}
            disabled={submitting}
          >
            {t.common.cancel}
          </Button>
          <Button type="submit" loading={submitting}>
            {submitting ? t.staffPage.savingButton : t.staffPage.saveButton}
          </Button>
        </div>
      </form>
    </ModalShell>
  );
}

interface ResetPasswordDialogProps {
  member: StaffUser;
  onClose: () => void;
  onSaved: (message: string) => void;
}

function ResetPasswordDialog({
  member,
  onClose,
  onSaved,
}: ResetPasswordDialogProps) {
  const { t } = useTranslation();
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  const onSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    setFormError(null);

    if (!password) {
      setFormError(t.staffPage.requiredFields);
      return;
    }
    if (password !== confirm) {
      setFormError(t.staffPage.passwordMismatch);
      return;
    }

    setSubmitting(true);
    try {
      await request(`/staff/${member.id}/reset-password`, {
        method: "POST",
        body: { new_password: password },
      });
      onSaved(t.staffPage.resetSuccess);
    } catch (err) {
      setFormError(
        describeError(err, {
          forbidden: t.staffPage.resetForbidden,
          fallback: t.staffPage.resetError,
        }),
      );
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <ModalShell
      title={t.staffPage.resetTitle}
      onClose={onClose}
      busy={submitting}
    >
      <form onSubmit={onSubmit}>
        <div className="space-y-4 px-4 py-4">
          <Alert variant="warning" title={t.staffPage.resetTitle}>
            {t.staffPage.resetDescription}
          </Alert>
          <p className="text-body text-ink-600">
            {t.staffPage.accountLabel}:{" "}
            <span className="font-mono text-ink-900">{member.username}</span>
          </p>
          <Input
            label={t.staffPage.fieldNewPassword}
            type="password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            helperText={t.staffPage.fieldPasswordHint}
            autoComplete="new-password"
            disabled={submitting}
            autoFocus
          />
          <Input
            label={t.staffPage.fieldPasswordConfirm}
            type="password"
            value={confirm}
            onChange={(event) => setConfirm(event.target.value)}
            autoComplete="new-password"
            disabled={submitting}
          />
          {formError && <Alert variant="error">{formError}</Alert>}
        </div>
        <div className="flex justify-end gap-2 border-t border-border px-4 py-3">
          <Button
            variant="secondary"
            type="button"
            onClick={onClose}
            disabled={submitting}
          >
            {t.common.cancel}
          </Button>
          <Button type="submit" variant="danger" loading={submitting}>
            {submitting ? t.staffPage.resetting : t.staffPage.resetSubmit}
          </Button>
        </div>
      </form>
    </ModalShell>
  );
}

interface ChangeOwnPasswordDialogProps {
  onClose: () => void;
  onSaved: (message: string) => void;
}

function ChangeOwnPasswordDialog({
  onClose,
  onSaved,
}: ChangeOwnPasswordDialogProps) {
  const { t } = useTranslation();
  const [current, setCurrent] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  const onSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    setFormError(null);

    if (!current) {
      setFormError(t.staffPage.currentPasswordRequired);
      return;
    }
    if (!password) {
      setFormError(t.staffPage.requiredFields);
      return;
    }
    if (password !== confirm) {
      setFormError(t.staffPage.passwordMismatch);
      return;
    }

    setSubmitting(true);
    try {
      await request("/staff/me/change-password", {
        method: "POST",
        body: { current_password: current, new_password: password },
      });
      onSaved(t.staffPage.changeOwnSuccess);
    } catch (err) {
      setFormError(
        describeError(err, {
          forbidden: t.staffPage.changeOwnError,
          fallback: t.staffPage.changeOwnError,
        }),
      );
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <ModalShell
      title={t.staffPage.changeOwnTitle}
      onClose={onClose}
      busy={submitting}
    >
      <form onSubmit={onSubmit}>
        <div className="space-y-4 px-4 py-4">
          <p className="text-body text-ink-600">
            {t.staffPage.changeOwnDescription}
          </p>
          <Input
            label={t.staffPage.fieldCurrentPassword}
            type="password"
            value={current}
            onChange={(event) => setCurrent(event.target.value)}
            autoComplete="current-password"
            disabled={submitting}
            autoFocus
          />
          <Input
            label={t.staffPage.fieldNewPassword}
            type="password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            helperText={t.staffPage.fieldPasswordHint}
            autoComplete="new-password"
            disabled={submitting}
          />
          <Input
            label={t.staffPage.fieldPasswordConfirm}
            type="password"
            value={confirm}
            onChange={(event) => setConfirm(event.target.value)}
            autoComplete="new-password"
            disabled={submitting}
          />
          {formError && <Alert variant="error">{formError}</Alert>}
        </div>
        <div className="flex justify-end gap-2 border-t border-border px-4 py-3">
          <Button
            variant="secondary"
            type="button"
            onClick={onClose}
            disabled={submitting}
          >
            {t.common.cancel}
          </Button>
          <Button type="submit" loading={submitting}>
            {submitting
              ? t.staffPage.changeOwnSubmitting
              : t.staffPage.changeOwnSubmit}
          </Button>
        </div>
      </form>
    </ModalShell>
  );
}

interface ModalShellProps {
  title: string;
  busy?: boolean;
  onClose: () => void;
  children: React.ReactNode;
}

/**
 * Cangkang dialog halaman ini. Mengikuti pola dialog lain: dapat ditutup dengan
 * Escape atau klik latar, fokus awal diarahkan ke bidang pertama lewat autoFocus.
 */
function ModalShell({
  title,
  busy = false,
  onClose,
  children,
}: ModalShellProps) {
  const { t } = useTranslation();

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !busy) onClose();
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [busy, onClose]);

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-ink-900/40 p-6"
      onClick={() => {
        if (!busy) onClose();
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label={title}
        className="max-h-full w-full max-w-lg overflow-y-auto rounded-md border border-border bg-surface shadow-overlay"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex items-center justify-between gap-4 border-b border-border px-4 py-3">
          <h2 className="text-title font-semibold text-ink-900">{title}</h2>
          <Button
            variant="ghost"
            size="sm"
            type="button"
            onClick={onClose}
            disabled={busy}
          >
            {t.common.close}
          </Button>
        </div>
        {children}
      </div>
    </div>
  );
}
