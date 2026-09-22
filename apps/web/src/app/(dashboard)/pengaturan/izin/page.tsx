"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { ApiError, request } from "@/lib/api";
import { useAuth } from "@/lib/useAuth";
import { hasPermission } from "@/lib/permissions";
import { useTranslation } from "@/i18n/context";
import type {
  PermissionCatalog,
  PermissionGroup,
} from "@/lib/operations-types";
import { PageHeader } from "@/components/ui/PageHeader";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Badge } from "@/components/ui/Badge";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { DataTable, type Column } from "@/components/ui/DataTable";
import { ErrorState } from "@/components/ui/States";

type Operation = "GRANT" | "REVOKE";

/**
 * Halaman pengelolaan grup & izin. Katalog hanya menampilkan keadaan yang
 * berlaku; setiap perubahan DIAJUKAN ke antrean maker-checker dan baru berlaku
 * setelah disetujui pemeriksa lain. Menyembunyikan form bukan batas keamanan:
 * API menolak tanpa izin permissions:manage (katalog: system:config:read).
 */
export default function IzinPage() {
  const { t } = useTranslation();
  const { user } = useAuth();

  const canReview =
    hasPermission(user, "system:config:read") ||
    hasPermission(user, "permissions:manage");
  const canRequest = hasPermission(user, "permissions:manage");

  const [catalog, setCatalog] = useState<PermissionCatalog | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);

  const [groupCode, setGroupCode] = useState("");
  const [permission, setPermission] = useState("");
  const [operation, setOperation] = useState<"" | Operation>("");
  const [notes, setNotes] = useState("");
  const [confirmAccessLoss, setConfirmAccessLoss] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await request<PermissionCatalog>("/permissions/catalog");
      setCatalog(response.data ?? null);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setError(t.permissionsPage.restricted);
      } else {
        setError(err instanceof ApiError ? err.message : t.permissionsPage.loadError);
      }
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    if (canReview) load();
    else setLoading(false);
  }, [load, reloadKey, canReview]);

  const groups = catalog?.groups ?? [];

  const groupOptions = useMemo(
    () => groups.map((group) => ({ value: group.code, label: `${group.code} - ${group.name}` })),
    [groups]
  );
  const permissionOptions = useMemo(
    () => (catalog?.available_permissions ?? []).map((p) => ({ value: p, label: p })),
    [catalog]
  );

  const onSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    setFormError(null);
    setFeedback(null);

    if (!groupCode || !permission || operation === "") {
      setFormError(t.permissionsPage.requiredFields);
      return;
    }

    setSubmitting(true);
    try {
      const response = await request<{ request_id: string }>("/permissions/requests", {
        method: "POST",
        body: {
          group_code: groupCode,
          permission,
          operation,
          confirm_access_loss: confirmAccessLoss,
          notes: notes.trim(),
        },
      });
      const requestId = response.data?.request_id ?? "";
      setFeedback(`${t.permissionsPage.submitted} (${requestId})`);
      setPermission("");
      setOperation("");
      setNotes("");
      setConfirmAccessLoss(false);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setFormError(t.permissionsPage.forbidden);
      } else {
        setFormError(
          err instanceof ApiError ? err.message : t.permissionsPage.requestError
        );
      }
    } finally {
      setSubmitting(false);
    }
  };

  const columns: Column<PermissionGroup>[] = [
    { header: t.permissionsPage.colCode, accessorKey: "code", isMono: true },
    { header: t.permissionsPage.colName, accessorKey: "name" },
    {
      header: t.permissionsPage.colKind,
      cell: (row) =>
        row.is_system ? (
          <Badge variant="outline">{t.permissionsPage.systemBadge}</Badge>
        ) : (
          <span className="text-ink-600">-</span>
        ),
    },
    {
      header: t.permissionsPage.colApprovalRole,
      cell: (row) => (
        <span className="font-mono text-meta">
          {row.approval_limit_role || t.permissionsPage.ownApprovalRole}
        </span>
      ),
    },
    {
      header: t.permissionsPage.colMembers,
      cell: (row) =>
        row.members.length === 0 ? (
          <span className="text-ink-600">{t.permissionsPage.noMembers}</span>
        ) : (
          <span className="text-meta">
            {row.members.map((member) => member.username).join(", ")}
          </span>
        ),
    },
    {
      header: t.permissionsPage.colPermissions,
      cell: (row) =>
        row.permissions.length === 0 ? (
          <span className="text-ink-600">{t.permissionsPage.noPermissions}</span>
        ) : (
          <div className="flex flex-wrap gap-1">
            {row.permissions.map((p) => (
              <Badge key={p} variant="neutral">
                {p}
              </Badge>
            ))}
          </div>
        ),
    },
  ];

  if (!canReview) {
    return (
      <>
        <PageHeader
          title={t.permissionsPage.title}
          description={t.permissionsPage.description}
        />
        <ErrorState title={t.permissionsPage.restricted} />
      </>
    );
  }

  return (
    <>
      <PageHeader
        title={t.permissionsPage.title}
        description={t.permissionsPage.description}
      />

      {error && !loading ? (
        <ErrorState
          title={t.permissionsPage.loadError}
          description={error}
          action={
            <Button variant="secondary" onClick={() => setReloadKey((key) => key + 1)}>
              {t.common.retry}
            </Button>
          }
        />
      ) : (
        <Card className="mb-4">
          <CardHeader>
            <CardTitle>{t.permissionsPage.listTitle}</CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            <DataTable
              columns={columns}
              data={groups}
              keyExtractor={(row) => row.code}
              loading={loading}
              emptyMessage={t.permissionsPage.empty}
              zebra
            />
          </CardContent>
        </Card>
      )}

      {canRequest && (
        <Card>
          <CardHeader>
            <CardTitle>{t.permissionsPage.requestTitle}</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="mb-3 text-meta text-ink-600">
              {t.permissionsPage.requestDescription}
            </p>
            <form onSubmit={onSubmit} className="space-y-3">
              <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
                <Select
                  label={t.permissionsPage.group}
                  value={groupCode}
                  onChange={(event) => setGroupCode(event.target.value)}
                  placeholder={t.permissionsPage.groupPlaceholder}
                  options={groupOptions}
                />
                <Select
                  label={t.permissionsPage.permission}
                  value={permission}
                  onChange={(event) => setPermission(event.target.value)}
                  placeholder={t.permissionsPage.permissionPlaceholder}
                  options={permissionOptions}
                />
                <Select
                  label={t.permissionsPage.operation}
                  value={operation}
                  onChange={(event) => setOperation(event.target.value as "" | Operation)}
                  placeholder={t.permissionsPage.operationPlaceholder}
                  options={[
                    { value: "GRANT", label: t.permissionsPage.operationGrant },
                    { value: "REVOKE", label: t.permissionsPage.operationRevoke },
                  ]}
                />
              </div>
              <Input
                label={t.permissionsPage.notes}
                value={notes}
                onChange={(event) => setNotes(event.target.value)}
                placeholder={t.permissionsPage.notesPlaceholder}
              />
              <label className="flex items-center gap-2 text-body text-ink-900">
                <input
                  type="checkbox"
                  className="h-4 w-4 rounded-sm border border-border-strong"
                  checked={confirmAccessLoss}
                  onChange={(event) => setConfirmAccessLoss(event.target.checked)}
                />
                {t.permissionsPage.confirmAccessLoss}
              </label>
              <div className="flex items-center gap-3">
                <Button type="submit" loading={submitting}>
                  {submitting ? t.permissionsPage.submitting : t.permissionsPage.submit}
                </Button>
                {formError && (
                  <span className="text-meta text-debit-700" role="alert">
                    {formError}
                  </span>
                )}
                {feedback && <span className="text-meta text-credit-700">{feedback}</span>}
              </div>
            </form>
          </CardContent>
        </Card>
      )}
    </>
  );
}
