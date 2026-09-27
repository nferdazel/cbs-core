"use client";

import { useCallback, useEffect, useState } from "react";
import type { Customer } from "@cbs/shared-types";
import { ApiError, request } from "@/lib/api";
import type {
  BMPKLimit,
  BMPKMasterData,
  BMPKRelatedParty,
} from "@/lib/operations-types";
import { formatDateISO } from "@/lib/format";
import { useAuth } from "@/lib/useAuth";
import { hasPermission } from "@/lib/permissions";
import { useTranslation } from "@/i18n/context";
import { Alert } from "@/components/ui/Alert";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { CurrencyInput } from "@/components/ui/CurrencyInput";
import { DataTable, Column } from "@/components/ui/DataTable";
import { DateInput } from "@/components/ui/DateInput";
import { Input } from "@/components/ui/Input";
import { ErrorState, LoadingState } from "@/components/ui/States";
import { Textarea } from "@/components/ui/Textarea";
import { CustomerPicker } from "@/components/settings/CustomerPicker";

interface RelatedForm {
  /** customer_id baris yang sedang diubah; kosong berarti baris baru. */
  original: string;
  customerId: string;
  relationship_type: string;
  note: string;
}

interface LimitForm {
  original: string;
  customerId: string;
  max_amount: number;
  effective_date: string;
  note: string;
}

const EMPTY_RELATED: RelatedForm = {
  original: "",
  customerId: "",
  relationship_type: "",
  note: "",
};

const EMPTY_LIMIT: LimitForm = {
  original: "",
  customerId: "",
  max_amount: 0,
  effective_date: "",
  note: "",
};

interface DeleteTarget {
  kind: "related" | "limit";
  customerId: string;
}

/**
 * Kartu pengisian fondasi BMPK: penandaan pihak terkait dan batas maksimum
 * pemberian kredit per nasabah. Data sebelumnya hanya dapat diisi lewat API/SQL.
 * Penjagaan izin sebenarnya di API; kartu hanya menyembunyikan kontrol tulis saat
 * peran tidak memegang system:config.
 */
export function OJKBMPKCard() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const canEdit = hasPermission(user, "system:config");

  const [related, setRelated] = useState<BMPKRelatedParty[]>([]);
  const [limits, setLimits] = useState<BMPKLimit[]>([]);
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget | null>(null);
  const [relatedForm, setRelatedForm] = useState<RelatedForm | null>(null);
  const [limitForm, setLimitForm] = useState<LimitForm | null>(null);
  const [relatedCustomer, setRelatedCustomer] = useState<Customer | null>(null);
  const [limitCustomer, setLimitCustomer] = useState<Customer | null>(null);
  const [relatedPickerKey, setRelatedPickerKey] = useState(0);
  const [limitPickerKey, setLimitPickerKey] = useState(0);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    setForbidden(false);
    try {
      const response = await request<BMPKMasterData>(
        "/reports/ojk/bmpk/master",
      );
      setRelated(response.data?.related_parties ?? []);
      setLimits(response.data?.limits ?? []);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setForbidden(true);
      } else {
        setLoadError(
          err instanceof ApiError ? err.message : t.ojkData.bmpk.loadError,
        );
      }
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    load();
  }, [load]);

  const resetMessages = () => {
    setFeedback(null);
    setFieldError(null);
    setSaveError(null);
  };

  const applySaveError = (err: unknown) => {
    if (err instanceof ApiError && err.status === 403) {
      setSaveError(t.ojkData.saveForbidden);
    } else if (err instanceof ApiError && err.status === 422) {
      setFieldError(err.message);
    } else {
      setSaveError(err instanceof ApiError ? err.message : t.ojkData.saveError);
    }
  };

  const openNewRelated = () => {
    resetMessages();
    setRelatedCustomer(null);
    setRelatedPickerKey((key) => key + 1);
    setRelatedForm({ ...EMPTY_RELATED });
  };

  const openEditRelated = (row: BMPKRelatedParty) => {
    resetMessages();
    setRelatedCustomer(null);
    setRelatedPickerKey((key) => key + 1);
    setRelatedForm({
      original: row.customer_id,
      customerId: row.customer_id,
      relationship_type: row.relationship_type,
      note: row.note ?? "",
    });
  };

  const openNewLimit = () => {
    resetMessages();
    setLimitCustomer(null);
    setLimitPickerKey((key) => key + 1);
    setLimitForm({ ...EMPTY_LIMIT });
  };

  const openEditLimit = (row: BMPKLimit) => {
    resetMessages();
    setLimitCustomer(null);
    setLimitPickerKey((key) => key + 1);
    setLimitForm({
      original: row.customer_id,
      customerId: row.customer_id,
      max_amount: Number(row.max_amount) || 0,
      effective_date: formatDateISO(row.effective_date),
      note: row.note ?? "",
    });
  };

  const saveRelated = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!relatedForm) return;
    if (!relatedForm.customerId) {
      setFieldError(t.ojkData.customer.notChosen);
      return;
    }
    resetMessages();
    setSaving(true);
    try {
      await request("/reports/ojk/bmpk/related-parties", {
        method: "PUT",
        body: {
          customer_id: relatedForm.customerId,
          relationship_type: relatedForm.relationship_type,
          note: relatedForm.note,
        },
      });
      setFeedback(t.ojkData.bmpk.savedRelated);
      setRelatedForm(null);
      await load();
    } catch (err) {
      applySaveError(err);
    } finally {
      setSaving(false);
    }
  };

  const saveLimit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!limitForm) return;
    if (!limitForm.customerId) {
      setFieldError(t.ojkData.customer.notChosen);
      return;
    }
    resetMessages();
    setSaving(true);
    try {
      await request("/reports/ojk/bmpk/limits", {
        method: "PUT",
        body: {
          customer_id: limitForm.customerId,
          max_amount: limitForm.max_amount,
          effective_date: limitForm.effective_date,
          note: limitForm.note,
        },
      });
      setFeedback(t.ojkData.bmpk.savedLimit);
      setLimitForm(null);
      await load();
    } catch (err) {
      applySaveError(err);
    } finally {
      setSaving(false);
    }
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    resetMessages();
    setDeleting(true);
    const path =
      deleteTarget.kind === "related"
        ? `/reports/ojk/bmpk/related-parties/${deleteTarget.customerId}`
        : `/reports/ojk/bmpk/limits/${deleteTarget.customerId}`;
    try {
      await request(path, { method: "DELETE" });
      setFeedback(
        deleteTarget.kind === "related"
          ? t.ojkData.bmpk.deletedRelated
          : t.ojkData.bmpk.deletedLimit,
      );
      setDeleteTarget(null);
      await load();
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setSaveError(t.ojkData.deleteForbidden);
      } else {
        setSaveError(
          err instanceof ApiError ? err.message : t.ojkData.deleteError,
        );
      }
      setDeleteTarget(null);
    } finally {
      setDeleting(false);
    }
  };

  const customerCell = (customerId: string) => (
    <span className="font-mono">{customerId}</span>
  );

  const relatedColumns: Column<BMPKRelatedParty>[] = [
    {
      header: t.ojkData.bmpk.colCustomer,
      cell: (row) => customerCell(row.customer_id),
    },
    {
      header: t.ojkData.bmpk.colRelationshipType,
      accessorKey: "relationship_type",
    },
    { header: t.ojkData.bmpk.colNote, accessorKey: "note" },
  ];
  if (canEdit) {
    relatedColumns.push({
      header: t.common.actions,
      align: "right",
      cell: (row) => (
        <div className="flex justify-end gap-2">
          <Button
            size="sm"
            variant="secondary"
            onClick={() => openEditRelated(row)}
          >
            {t.ojkData.edit}
          </Button>
          <Button
            size="sm"
            variant="danger"
            onClick={() =>
              setDeleteTarget({
                kind: "related",
                customerId: row.customer_id,
              })
            }
          >
            {t.ojkData.delete}
          </Button>
        </div>
      ),
    });
  }

  const limitColumns: Column<BMPKLimit>[] = [
    {
      header: t.ojkData.bmpk.colCustomer,
      cell: (row) => customerCell(row.customer_id),
    },
    {
      header: t.ojkData.bmpk.colMaxAmount,
      type: "money",
      accessorKey: "max_amount",
    },
    {
      header: t.ojkData.bmpk.colEffectiveDate,
      cell: (row) => formatDateISO(row.effective_date) || "-",
    },
    { header: t.ojkData.bmpk.colNote, accessorKey: "note" },
  ];
  if (canEdit) {
    limitColumns.push({
      header: t.common.actions,
      align: "right",
      cell: (row) => (
        <div className="flex justify-end gap-2">
          <Button
            size="sm"
            variant="secondary"
            onClick={() => openEditLimit(row)}
          >
            {t.ojkData.edit}
          </Button>
          <Button
            size="sm"
            variant="danger"
            onClick={() =>
              setDeleteTarget({ kind: "limit", customerId: row.customer_id })
            }
          >
            {t.ojkData.delete}
          </Button>
        </div>
      ),
    });
  }

  return (
    <Card className="mb-4">
      <CardHeader>
        <CardTitle>{t.ojkData.bmpk.title}</CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <LoadingState label={t.states.loading} />
        ) : forbidden ? (
          <ErrorState
            title={t.ojkData.bmpk.title}
            description={t.ojkData.bmpk.forbidden}
          />
        ) : loadError ? (
          <ErrorState
            title={t.ojkData.bmpk.title}
            description={loadError}
            action={
              <Button variant="secondary" onClick={load}>
                {t.common.retry}
              </Button>
            }
          />
        ) : (
          <>
            <p className="text-body text-ink-600">
              {t.ojkData.bmpk.description}
            </p>
            {!canEdit && <Alert variant="info">{t.ojkData.readOnly}</Alert>}
            {feedback && <Alert variant="success">{feedback}</Alert>}
            {fieldError && <Alert variant="error">{fieldError}</Alert>}
            {saveError && <Alert variant="error">{saveError}</Alert>}

            <section className="space-y-3">
              <div className="flex items-center justify-between gap-4">
                <h4 className="text-title font-semibold text-ink-900">
                  {t.ojkData.bmpk.relatedTitle}
                </h4>
                {canEdit && !relatedForm && (
                  <Button size="sm" onClick={openNewRelated}>
                    {t.ojkData.bmpk.addRelated}
                  </Button>
                )}
              </div>

              {relatedForm && (
                <form
                  onSubmit={saveRelated}
                  className="space-y-4 rounded-md border border-border p-4"
                >
                  <h5 className="text-meta font-medium uppercase tracking-wide text-ink-600">
                    {relatedForm.original
                      ? t.ojkData.bmpk.editRelatedTitle
                      : t.ojkData.bmpk.newRelatedTitle}
                  </h5>
                  {relatedForm.original && (
                    <p className="text-meta text-ink-600">
                      {t.ojkData.customer.current}:{" "}
                      <span className="font-mono">{relatedForm.original}</span>
                    </p>
                  )}
                  <CustomerPicker
                    key={relatedPickerKey}
                    selected={relatedCustomer}
                    onSelect={(customer) => {
                      setRelatedCustomer(customer);
                      setRelatedForm((prev) =>
                        prev
                          ? { ...prev, customerId: customer?.id ?? "" }
                          : prev,
                      );
                    }}
                  />
                  <div className="grid gap-4 md:grid-cols-2">
                    <Input
                      label={t.ojkData.bmpk.fieldRelationshipType}
                      helperText={t.ojkData.bmpk.fieldRelationshipTypeHint}
                      value={relatedForm.relationship_type}
                      onChange={(event) =>
                        setRelatedForm((prev) =>
                          prev
                            ? {
                                ...prev,
                                relationship_type: event.target.value,
                              }
                            : prev,
                        )
                      }
                    />
                    <Textarea
                      label={t.ojkData.bmpk.fieldNote}
                      value={relatedForm.note}
                      onChange={(event) =>
                        setRelatedForm((prev) =>
                          prev ? { ...prev, note: event.target.value } : prev,
                        )
                      }
                    />
                  </div>
                  <div className="flex justify-end gap-2">
                    <Button
                      type="button"
                      variant="secondary"
                      onClick={() => setRelatedForm(null)}
                    >
                      {t.common.cancel}
                    </Button>
                    <Button type="submit" loading={saving}>
                      {saving ? t.ojkData.saving : t.ojkData.save}
                    </Button>
                  </div>
                </form>
              )}

              <DataTable
                columns={relatedColumns}
                data={related}
                keyExtractor={(row) => row.customer_id}
                emptyMessage={t.ojkData.bmpk.relatedEmpty}
                zebra
              />
            </section>

            <section className="space-y-3">
              <div className="flex items-center justify-between gap-4">
                <h4 className="text-title font-semibold text-ink-900">
                  {t.ojkData.bmpk.limitsTitle}
                </h4>
                {canEdit && !limitForm && (
                  <Button size="sm" onClick={openNewLimit}>
                    {t.ojkData.bmpk.addLimit}
                  </Button>
                )}
              </div>

              {limitForm && (
                <form
                  onSubmit={saveLimit}
                  className="space-y-4 rounded-md border border-border p-4"
                >
                  <h5 className="text-meta font-medium uppercase tracking-wide text-ink-600">
                    {limitForm.original
                      ? t.ojkData.bmpk.editLimitTitle
                      : t.ojkData.bmpk.newLimitTitle}
                  </h5>
                  {limitForm.original && (
                    <p className="text-meta text-ink-600">
                      {t.ojkData.customer.current}:{" "}
                      <span className="font-mono">{limitForm.original}</span>
                    </p>
                  )}
                  <CustomerPicker
                    key={limitPickerKey}
                    selected={limitCustomer}
                    onSelect={(customer) => {
                      setLimitCustomer(customer);
                      setLimitForm((prev) =>
                        prev
                          ? { ...prev, customerId: customer?.id ?? "" }
                          : prev,
                      );
                    }}
                  />
                  <div className="grid gap-4 md:grid-cols-2">
                    <CurrencyInput
                      label={t.ojkData.bmpk.fieldMaxAmount}
                      helperText={t.ojkData.bmpk.fieldMaxAmountHint}
                      value={limitForm.max_amount}
                      onChange={(value) =>
                        setLimitForm((prev) =>
                          prev ? { ...prev, max_amount: value } : prev,
                        )
                      }
                    />
                    <DateInput
                      label={t.ojkData.bmpk.fieldEffectiveDate}
                      helperText={t.ojkData.bmpk.fieldEffectiveDateHint}
                      value={limitForm.effective_date}
                      onChange={(event) =>
                        setLimitForm((prev) =>
                          prev
                            ? { ...prev, effective_date: event.target.value }
                            : prev,
                        )
                      }
                    />
                  </div>
                  <Textarea
                    label={t.ojkData.bmpk.fieldNote}
                    value={limitForm.note}
                    onChange={(event) =>
                      setLimitForm((prev) =>
                        prev ? { ...prev, note: event.target.value } : prev,
                      )
                    }
                  />
                  <div className="flex justify-end gap-2">
                    <Button
                      type="button"
                      variant="secondary"
                      onClick={() => setLimitForm(null)}
                    >
                      {t.common.cancel}
                    </Button>
                    <Button type="submit" loading={saving}>
                      {saving ? t.ojkData.saving : t.ojkData.save}
                    </Button>
                  </div>
                </form>
              )}

              <DataTable
                columns={limitColumns}
                data={limits}
                keyExtractor={(row) => row.customer_id}
                emptyMessage={t.ojkData.bmpk.limitsEmpty}
                zebra
              />
            </section>
          </>
        )}
      </CardContent>

      <ConfirmDialog
        open={deleteTarget !== null}
        title={t.ojkData.confirmDeleteTitle}
        description={t.ojkData.confirmDeleteBody}
        confirmLabel={deleting ? t.ojkData.deleting : t.ojkData.delete}
        destructive
        loading={deleting}
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </Card>
  );
}
