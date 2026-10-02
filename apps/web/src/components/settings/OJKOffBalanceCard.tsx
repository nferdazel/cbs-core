"use client";

import { useCallback, useEffect, useState } from "react";
import type { Customer } from "@cbs/shared-types";
import { ApiError, request } from "@/lib/api";
import type {
  OffBalanceItem,
  OffBalanceItemsData,
} from "@/lib/operations-types";
import { formatDateISO } from "@/lib/format";
import { useAuth } from "@/lib/useAuth";
import { hasPermission } from "@/lib/permissions";
import { useTranslation } from "@/i18n/context";
import { Alert } from "@/components/ui/Alert";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { CurrencyInput } from "@/components/ui/CurrencyInput";
import { DataTable, Column } from "@/components/ui/DataTable";
import { DateInput } from "@/components/ui/DateInput";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { ErrorState, LoadingState } from "@/components/ui/States";
import { Textarea } from "@/components/ui/Textarea";
import { CustomerPicker } from "@/components/settings/CustomerPicker";
import { withCurrent, labelOf, type Option } from "./register-options";

interface OffBalanceForm {
  id: string;
  position_code: string;
  category: string;
  description: string;
  amount: number;
  counterparty_customer_id: string;
  reference: string;
  as_of: string;
  status: string;
  note: string;
}

const EMPTY_ITEM: OffBalanceForm = {
  id: "",
  position_code: "",
  category: "",
  description: "",
  amount: 0,
  counterparty_customer_id: "",
  reference: "",
  as_of: "",
  status: "AKTIF",
  note: "",
};

function itemFormFrom(row: OffBalanceItem): OffBalanceForm {
  return {
    id: row.id,
    position_code: row.position_code ?? "",
    category: row.category ?? "",
    description: row.description ?? "",
    amount: Number(row.amount) || 0,
    counterparty_customer_id: row.counterparty_customer_id ?? "",
    reference: row.reference ?? "",
    as_of: formatDateISO(row.as_of),
    status: row.status ?? "",
    note: row.note ?? "",
  };
}

/**
 * Kartu pengisian register rekening administratif Form 01.01: pos komitmen dan
 * kontinjensi off-balance yang bank catat. Data sebelumnya hanya dapat diisi lewat
 * API/SQL. Penjagaan izin sebenarnya di API; kartu hanya menyembunyikan kontrol
 * tulis saat peran tidak memegang system:config.
 */
export function OJKOffBalanceCard() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const canEdit = hasPermission(user, "system:config");

  const [items, setItems] = useState<OffBalanceItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<OffBalanceItem | null>(null);
  const [form, setForm] = useState<OffBalanceForm | null>(null);
  const [counterparty, setCounterparty] = useState<Customer | null>(null);
  const [pickerKey, setPickerKey] = useState(0);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    setForbidden(false);
    try {
      const response = await request<OffBalanceItemsData>(
        "/reports/ojk/off-balance/items",
      );
      setItems(response.data?.items ?? []);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setForbidden(true);
      } else {
        setLoadError(
          err instanceof ApiError
            ? err.message
            : t.ojkData.offBalance.loadError,
        );
      }
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    load();
  }, [load]);

  const categoryOptions: Option[] = [
    { value: "KOMITMEN", label: t.ojkData.offBalance.categoryKomitmen },
    { value: "KONTINJENSI", label: t.ojkData.offBalance.categoryKontinjensi },
    { value: "LAINNYA", label: t.ojkData.offBalance.categoryLainnya },
  ];
  const statusOptions: Option[] = [
    { value: "AKTIF", label: t.ojkData.offBalance.statusAktif },
    { value: "NONAKTIF", label: t.ojkData.offBalance.statusNonaktif },
  ];

  const updateForm = (patch: Partial<OffBalanceForm>) =>
    setForm((prev) => (prev ? { ...prev, ...patch } : prev));

  const resetMessages = () => {
    setFeedback(null);
    setFieldError(null);
    setSaveError(null);
  };

  const openNew = () => {
    resetMessages();
    setCounterparty(null);
    setPickerKey((key) => key + 1);
    setForm({ ...EMPTY_ITEM });
  };

  const openEdit = (row: OffBalanceItem) => {
    resetMessages();
    setCounterparty(null);
    setPickerKey((key) => key + 1);
    setForm(itemFormFrom(row));
  };

  const onSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!form) return;
    resetMessages();
    setSaving(true);
    try {
      await request<OffBalanceItem>("/reports/ojk/off-balance/items", {
        method: "PUT",
        body: {
          id: form.id,
          position_code: form.position_code,
          category: form.category,
          description: form.description,
          amount: form.amount,
          counterparty_customer_id: form.counterparty_customer_id,
          reference: form.reference,
          as_of: form.as_of,
          status: form.status,
          note: form.note,
        },
      });
      setFeedback(t.ojkData.offBalance.saved);
      setForm(null);
      await load();
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setSaveError(t.ojkData.saveForbidden);
      } else if (err instanceof ApiError && err.status === 422) {
        setFieldError(err.message);
      } else {
        setSaveError(
          err instanceof ApiError ? err.message : t.ojkData.saveError,
        );
      }
    } finally {
      setSaving(false);
    }
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    resetMessages();
    setDeleting(true);
    try {
      await request(`/reports/ojk/off-balance/items/${deleteTarget.id}`, {
        method: "DELETE",
      });
      setFeedback(t.ojkData.offBalance.deleted);
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

  const categorySelectOptions = form
    ? withCurrent(categoryOptions, form.category)
    : categoryOptions;
  const statusSelectOptions = form
    ? withCurrent(statusOptions, form.status)
    : statusOptions;

  const columns: Column<OffBalanceItem>[] = [
    {
      header: t.ojkData.offBalance.colAsOf,
      cell: (row) => formatDateISO(row.as_of) || "-",
    },
    {
      header: t.ojkData.offBalance.colPositionCode,
      cell: (row) => {
        if (!row.position_code) return "-";
        return <span className="font-mono">{row.position_code}</span>;
      },
    },
    {
      header: t.ojkData.offBalance.colCategory,
      cell: (row) => labelOf(categoryOptions, row.category),
    },
    { header: t.ojkData.offBalance.colDescription, accessorKey: "description" },
    {
      header: t.ojkData.offBalance.colAmount,
      type: "money",
      accessorKey: "amount",
    },
    {
      header: t.ojkData.offBalance.colCounterparty,
      cell: (row) => {
        if (!row.counterparty_customer_id) return "-";
        return (
          <span className="font-mono">{row.counterparty_customer_id}</span>
        );
      },
    },
    { header: t.ojkData.offBalance.colReference, accessorKey: "reference" },
    {
      header: t.ojkData.offBalance.colStatus,
      cell: (row) => (
        <Badge variant={row.status === "AKTIF" ? "credit" : "outline"}>
          {labelOf(statusOptions, row.status)}
        </Badge>
      ),
    },
  ];
  if (canEdit) {
    columns.push({
      header: t.common.actions,
      align: "right",
      cell: (row) => (
        <div className="flex justify-end gap-2">
          <Button size="sm" variant="secondary" onClick={() => openEdit(row)}>
            {t.ojkData.edit}
          </Button>
          <Button
            size="sm"
            variant="danger"
            onClick={() => setDeleteTarget(row)}
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
        <CardTitle>{t.ojkData.offBalance.title}</CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <LoadingState label={t.states.loading} />
        ) : forbidden ? (
          <ErrorState
            title={t.ojkData.offBalance.title}
            description={t.ojkData.offBalance.forbidden}
          />
        ) : loadError ? (
          <ErrorState
            title={t.ojkData.offBalance.title}
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
              {t.ojkData.offBalance.description}
            </p>
            {!canEdit && <Alert variant="info">{t.ojkData.readOnly}</Alert>}
            {feedback && <Alert variant="success">{feedback}</Alert>}
            {fieldError && <Alert variant="error">{fieldError}</Alert>}
            {saveError && <Alert variant="error">{saveError}</Alert>}

            <div className="flex items-center justify-between gap-4">
              <h4 className="text-title font-semibold text-ink-900">
                {t.ojkData.offBalance.itemsTitle}
              </h4>
              {canEdit && !form && (
                <Button size="sm" onClick={openNew}>
                  {t.ojkData.offBalance.add}
                </Button>
              )}
            </div>

            {form && (
              <form
                onSubmit={onSubmit}
                className="space-y-4 rounded-md border border-border p-4"
              >
                <h5 className="text-meta font-medium uppercase tracking-wide text-ink-600">
                  {form.id
                    ? t.ojkData.offBalance.editTitle
                    : t.ojkData.offBalance.newTitle}
                </h5>
                <div className="grid gap-4 md:grid-cols-2">
                  <Input
                    label={t.ojkData.offBalance.fieldPositionCode}
                    helperText={t.ojkData.offBalance.fieldPositionCodeHint}
                    isMono
                    value={form.position_code}
                    onChange={(event) =>
                      updateForm({ position_code: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.offBalance.fieldCategory}
                    placeholder={t.ojkData.offBalance.categoryPlaceholder}
                    value={form.category}
                    options={categorySelectOptions}
                    onChange={(event) =>
                      updateForm({ category: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.offBalance.fieldDescription}
                    helperText={t.ojkData.offBalance.fieldDescriptionHint}
                    value={form.description}
                    onChange={(event) =>
                      updateForm({ description: event.target.value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.offBalance.fieldAmount}
                    value={form.amount}
                    onChange={(value) => updateForm({ amount: value })}
                  />
                  <Input
                    label={t.ojkData.offBalance.fieldReference}
                    value={form.reference}
                    onChange={(event) =>
                      updateForm({ reference: event.target.value })
                    }
                  />
                  <DateInput
                    label={t.ojkData.offBalance.fieldAsOf}
                    helperText={t.ojkData.offBalance.fieldAsOfHint}
                    value={form.as_of}
                    onChange={(event) =>
                      updateForm({ as_of: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.offBalance.fieldStatus}
                    value={form.status}
                    options={statusSelectOptions}
                    onChange={(event) =>
                      updateForm({ status: event.target.value })
                    }
                  />
                </div>

                <div className="rounded-md border border-border p-4">
                  <h6 className="mb-2 text-meta font-medium uppercase tracking-wide text-ink-600">
                    {t.ojkData.offBalance.fieldCounterparty}
                  </h6>
                  <p className="mb-2 text-meta text-ink-600">
                    {t.ojkData.offBalance.fieldCounterpartyHint}
                  </p>
                  {form.counterparty_customer_id && (
                    <p className="mb-2 text-meta text-ink-600">
                      {t.ojkData.customer.current}:{" "}
                      <span className="font-mono">
                        {form.counterparty_customer_id}
                      </span>
                    </p>
                  )}
                  <CustomerPicker
                    key={pickerKey}
                    selected={counterparty}
                    onSelect={(customer) => {
                      setCounterparty(customer);
                      updateForm({
                        counterparty_customer_id: customer?.id ?? "",
                      });
                    }}
                  />
                </div>

                <Textarea
                  label={t.ojkData.offBalance.fieldNote}
                  value={form.note}
                  onChange={(event) => updateForm({ note: event.target.value })}
                />

                <div className="flex justify-end gap-2">
                  <Button
                    type="button"
                    variant="secondary"
                    onClick={() => setForm(null)}
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
              columns={columns}
              data={items}
              keyExtractor={(row) => row.id}
              emptyMessage={t.ojkData.offBalance.empty}
              zebra
            />
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
