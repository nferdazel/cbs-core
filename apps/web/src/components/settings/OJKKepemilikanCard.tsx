"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, request } from "@/lib/api";
import type {
  KepemilikanItem,
  KepemilikanItemsData,
} from "@/lib/operations-types";
import { formatDateISO, formatRate } from "@/lib/format";
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
import { withCurrent, labelOf, type Option } from "./register-options";

interface KepemilikanForm {
  id: string;
  shareholder_name: string;
  shareholder_address: string;
  shareholder_type_code: string;
  shareholder_status_code: string;
  nominal_amount: number;
  ownership_percentage: number;
  change_status_code: string;
  as_of: string;
  status: string;
  note: string;
}

const EMPTY_ITEM: KepemilikanForm = {
  id: "",
  shareholder_name: "",
  shareholder_address: "",
  shareholder_type_code: "",
  shareholder_status_code: "",
  nominal_amount: 0,
  ownership_percentage: 0,
  change_status_code: "",
  as_of: "",
  status: "AKTIF",
  note: "",
};

/** Sandi baku Form 00.01 (domain/kepemilikan.go). */
const SHAREHOLDER_TYPE_CODES: string[] = ["01", "02", "03", "04"];
const SHAREHOLDER_STATUS_CODES: string[] = ["01", "02"];
const CHANGE_STATUS_CODES: string[] = ["1", "2", "3", "9"];

function itemFormFrom(row: KepemilikanItem): KepemilikanForm {
  return {
    id: row.id,
    shareholder_name: row.shareholder_name ?? "",
    shareholder_address: row.shareholder_address ?? "",
    shareholder_type_code: row.shareholder_type_code ?? "",
    shareholder_status_code: row.shareholder_status_code ?? "",
    nominal_amount: Number(row.nominal_amount) || 0,
    ownership_percentage: Number(row.ownership_percentage) || 0,
    change_status_code: row.change_status_code ?? "",
    as_of: formatDateISO(row.as_of),
    status: row.status ?? "",
    note: row.note ?? "",
  };
}

/**
 * Kartu pengisian register pemegang saham Form 00.01: data kepemilikan BPR per
 * baris. Data sebelumnya hanya dapat diisi lewat API/SQL. Penjagaan izin
 * sebenarnya di API (system:config); kartu hanya menyembunyikan kontrol tulis
 * saat peran tidak memegang izin itu. Nomor identitas (kolom IV) memang tidak
 * ada di kontrak karena tidak disimpan.
 */
export function OJKKepemilikanCard() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const canEdit = hasPermission(user, "system:config");

  const [items, setItems] = useState<KepemilikanItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<KepemilikanItem | null>(
    null,
  );
  const [form, setForm] = useState<KepemilikanForm | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    setForbidden(false);
    try {
      const response = await request<KepemilikanItemsData>(
        "/reports/ojk/kepemilikan/items",
      );
      setItems(response.data?.items ?? []);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setForbidden(true);
      } else {
        setLoadError(
          err instanceof ApiError
            ? err.message
            : t.ojkData.kepemilikan.loadError,
        );
      }
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    load();
  }, [load]);

  const typeOptions: Option[] = [
    { value: "01", label: t.ojkData.kepemilikan.typePerorangan },
    { value: "02", label: t.ojkData.kepemilikan.typeBadanHukum },
    { value: "03", label: t.ojkData.kepemilikan.typePemerintahDaerah },
    { value: "04", label: t.ojkData.kepemilikan.typePublik },
  ];
  const shareholderStatusOptions: Option[] = [
    { value: "01", label: t.ojkData.kepemilikan.shareholderStatusPSP },
    { value: "02", label: t.ojkData.kepemilikan.shareholderStatusNonPSP },
  ];
  const changeStatusOptions: Option[] = [
    { value: "1", label: t.ojkData.kepemilikan.changeStatusBaru },
    { value: "2", label: t.ojkData.kepemilikan.changeStatusUbahPSP },
    { value: "3", label: t.ojkData.kepemilikan.changeStatusUbahTanpaPSP },
    { value: "9", label: t.ojkData.kepemilikan.changeStatusTidakAda },
  ];
  const statusOptions: Option[] = [
    { value: "AKTIF", label: t.ojkData.kepemilikan.statusAktif },
    { value: "NONAKTIF", label: t.ojkData.kepemilikan.statusNonaktif },
  ];

  const updateForm = (patch: Partial<KepemilikanForm>) =>
    setForm((prev) => (prev ? { ...prev, ...patch } : prev));

  const resetMessages = () => {
    setFeedback(null);
    setFieldError(null);
    setSaveError(null);
  };

  const openNew = () => {
    resetMessages();
    setForm({ ...EMPTY_ITEM });
  };

  const openEdit = (row: KepemilikanItem) => {
    resetMessages();
    setForm(itemFormFrom(row));
  };

  /** Batasan yang sama dengan domain.BuildKepemilikanItem, dicek sebelum menembak API. */
  const validate = (value: KepemilikanForm): string | null => {
    const name = value.shareholder_name.trim();
    if (!name) return t.ojkData.kepemilikan.validationName;
    if (name.length > 255) return t.ojkData.kepemilikan.validationNameLength;
    if (value.shareholder_address.trim().length > 255) {
      return t.ojkData.kepemilikan.validationAddressLength;
    }
    if (!SHAREHOLDER_TYPE_CODES.includes(value.shareholder_type_code)) {
      return t.ojkData.kepemilikan.validationType;
    }
    if (!SHAREHOLDER_STATUS_CODES.includes(value.shareholder_status_code)) {
      return t.ojkData.kepemilikan.validationShareholderStatus;
    }
    if (value.nominal_amount < 0) {
      return t.ojkData.kepemilikan.validationNominal;
    }
    if (value.ownership_percentage < 0 || value.ownership_percentage > 100) {
      return t.ojkData.kepemilikan.validationPercentage;
    }
    if (!CHANGE_STATUS_CODES.includes(value.change_status_code)) {
      return t.ojkData.kepemilikan.validationChangeStatus;
    }
    if (!value.as_of) return t.ojkData.kepemilikan.validationAsOf;
    return null;
  };

  const onSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!form) return;
    resetMessages();
    const invalid = validate(form);
    if (invalid) {
      setFieldError(invalid);
      return;
    }
    setSaving(true);
    try {
      await request<KepemilikanItem>("/reports/ojk/kepemilikan/items", {
        method: "PUT",
        body: {
          id: form.id,
          shareholder_name: form.shareholder_name,
          shareholder_address: form.shareholder_address,
          shareholder_type_code: form.shareholder_type_code,
          shareholder_status_code: form.shareholder_status_code,
          nominal_amount: form.nominal_amount,
          ownership_percentage: form.ownership_percentage,
          change_status_code: form.change_status_code,
          as_of: form.as_of,
          status: form.status,
          note: form.note,
        },
      });
      setFeedback(t.ojkData.kepemilikan.saved);
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
      await request(`/reports/ojk/kepemilikan/items/${deleteTarget.id}`, {
        method: "DELETE",
      });
      setFeedback(t.ojkData.kepemilikan.deleted);
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

  const typeSelectOptions = form
    ? withCurrent(typeOptions, form.shareholder_type_code)
    : typeOptions;
  const shareholderStatusSelectOptions = form
    ? withCurrent(shareholderStatusOptions, form.shareholder_status_code)
    : shareholderStatusOptions;
  const changeStatusSelectOptions = form
    ? withCurrent(changeStatusOptions, form.change_status_code)
    : changeStatusOptions;
  const statusSelectOptions = form
    ? withCurrent(statusOptions, form.status)
    : statusOptions;

  const columns: Column<KepemilikanItem>[] = [
    { header: t.ojkData.kepemilikan.colName, accessorKey: "shareholder_name" },
    {
      header: t.ojkData.kepemilikan.colAddress,
      cell: (row) => row.shareholder_address || "-",
    },
    {
      header: t.ojkData.kepemilikan.colType,
      cell: (row) => labelOf(typeOptions, row.shareholder_type_code),
    },
    {
      header: t.ojkData.kepemilikan.colShareholderStatus,
      cell: (row) =>
        labelOf(shareholderStatusOptions, row.shareholder_status_code),
    },
    {
      header: t.ojkData.kepemilikan.colNominal,
      type: "money",
      accessorKey: "nominal_amount",
    },
    {
      header: t.ojkData.kepemilikan.colPercentage,
      align: "right",
      cell: (row) => formatRate(row.ownership_percentage),
    },
    {
      header: t.ojkData.kepemilikan.colChangeStatus,
      cell: (row) => labelOf(changeStatusOptions, row.change_status_code),
    },
    {
      header: t.ojkData.kepemilikan.colAsOf,
      cell: (row) => formatDateISO(row.as_of) || "-",
    },
    {
      header: t.ojkData.kepemilikan.colStatus,
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
        <CardTitle>{t.ojkData.kepemilikan.title}</CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <LoadingState label={t.states.loading} />
        ) : forbidden ? (
          <ErrorState
            title={t.ojkData.kepemilikan.title}
            description={t.ojkData.kepemilikan.forbidden}
          />
        ) : loadError ? (
          <ErrorState
            title={t.ojkData.kepemilikan.title}
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
              {t.ojkData.kepemilikan.description}
            </p>
            {!canEdit && <Alert variant="info">{t.ojkData.readOnly}</Alert>}
            {feedback && <Alert variant="success">{feedback}</Alert>}
            {fieldError && <Alert variant="error">{fieldError}</Alert>}
            {saveError && <Alert variant="error">{saveError}</Alert>}

            <div className="flex items-center justify-between gap-4">
              <h4 className="text-title font-semibold text-ink-900">
                {t.ojkData.kepemilikan.itemsTitle}
              </h4>
              {canEdit && !form && (
                <Button size="sm" onClick={openNew}>
                  {t.ojkData.kepemilikan.add}
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
                    ? t.ojkData.kepemilikan.editTitle
                    : t.ojkData.kepemilikan.newTitle}
                </h5>
                <div className="grid gap-4 md:grid-cols-2">
                  <Input
                    label={t.ojkData.kepemilikan.fieldName}
                    helperText={t.ojkData.kepemilikan.fieldNameHint}
                    value={form.shareholder_name}
                    onChange={(event) =>
                      updateForm({ shareholder_name: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.kepemilikan.fieldAddress}
                    helperText={t.ojkData.kepemilikan.fieldAddressHint}
                    value={form.shareholder_address}
                    onChange={(event) =>
                      updateForm({ shareholder_address: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.kepemilikan.fieldType}
                    helperText={t.ojkData.kepemilikan.fieldTypeHint}
                    placeholder={t.ojkData.kepemilikan.typePlaceholder}
                    value={form.shareholder_type_code}
                    options={typeSelectOptions}
                    onChange={(event) =>
                      updateForm({ shareholder_type_code: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.kepemilikan.fieldShareholderStatus}
                    helperText={
                      t.ojkData.kepemilikan.fieldShareholderStatusHint
                    }
                    placeholder={
                      t.ojkData.kepemilikan.shareholderStatusPlaceholder
                    }
                    value={form.shareholder_status_code}
                    options={shareholderStatusSelectOptions}
                    onChange={(event) =>
                      updateForm({
                        shareholder_status_code: event.target.value,
                      })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.kepemilikan.fieldNominal}
                    helperText={t.ojkData.kepemilikan.fieldNominalHint}
                    value={form.nominal_amount}
                    onChange={(value) => updateForm({ nominal_amount: value })}
                  />
                  <CurrencyInput
                    label={t.ojkData.kepemilikan.fieldPercentage}
                    helperText={t.ojkData.kepemilikan.fieldPercentageHint}
                    currencyPrefix="%"
                    allowDecimals
                    value={form.ownership_percentage}
                    onChange={(value) =>
                      updateForm({ ownership_percentage: value })
                    }
                  />
                  <Select
                    label={t.ojkData.kepemilikan.fieldChangeStatus}
                    helperText={t.ojkData.kepemilikan.fieldChangeStatusHint}
                    placeholder={t.ojkData.kepemilikan.changeStatusPlaceholder}
                    value={form.change_status_code}
                    options={changeStatusSelectOptions}
                    onChange={(event) =>
                      updateForm({ change_status_code: event.target.value })
                    }
                  />
                  <DateInput
                    label={t.ojkData.kepemilikan.fieldAsOf}
                    helperText={t.ojkData.kepemilikan.fieldAsOfHint}
                    value={form.as_of}
                    onChange={(event) =>
                      updateForm({ as_of: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.kepemilikan.fieldStatus}
                    value={form.status}
                    options={statusSelectOptions}
                    onChange={(event) =>
                      updateForm({ status: event.target.value })
                    }
                  />
                </div>

                <Textarea
                  label={t.ojkData.kepemilikan.fieldNote}
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
              emptyMessage={t.ojkData.kepemilikan.empty}
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
