"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, request } from "@/lib/api";
import type { AYDAItem, AYDAItemsData } from "@/lib/operations-types";
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

type Option = { value: string; label: string };

function withCurrent(options: Option[], current: string): Option[] {
  if (!current || options.some((option) => option.value === current)) {
    return options;
  }
  return [...options, { value: current, label: current }];
}

function labelOf(options: Option[], value: string): string {
  return options.find((option) => option.value === value)?.label ?? value;
}

interface AYDAForm {
  id: string;
  collateral_type_code: string;
  collateral_address: string;
  acquisition_date: string;
  initial_recognition_value: number;
  accumulated_impairment: number;
  net_realizable_value: number;
  as_of: string;
  status: string;
  note: string;
}

const EMPTY_ITEM: AYDAForm = {
  id: "",
  collateral_type_code: "",
  collateral_address: "",
  acquisition_date: "",
  initial_recognition_value: 0,
  accumulated_impairment: 0,
  net_realizable_value: 0,
  as_of: "",
  status: "AKTIF",
  note: "",
};

/** Sandi jenis agunan baku Form 07.00 - 2 (domain/ayda.go). */
const COLLATERAL_TYPE_CODES: string[] = ["01", "02", "03", "04", "05", "99"];

function itemFormFrom(row: AYDAItem): AYDAForm {
  return {
    id: row.id,
    collateral_type_code: row.collateral_type_code ?? "",
    collateral_address: row.collateral_address ?? "",
    acquisition_date: formatDateISO(row.acquisition_date),
    initial_recognition_value: Number(row.initial_recognition_value) || 0,
    accumulated_impairment: Number(row.accumulated_impairment) || 0,
    net_realizable_value: Number(row.net_realizable_value) || 0,
    as_of: formatDateISO(row.as_of),
    status: row.status ?? "",
    note: row.note ?? "",
  };
}

/**
 * Kartu pengisian register AYDA Form 07.00: daftar agunan yang diambil alih per
 * kasus yang bank catat. Data sebelumnya hanya dapat diisi lewat API/SQL. Penjagaan
 * izin sebenarnya di API (system:config); kartu hanya menyembunyikan kontrol tulis
 * saat peran tidak memegang izin itu.
 */
export function OJKAYDACard() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const canEdit = hasPermission(user, "system:config");

  const [items, setItems] = useState<AYDAItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<AYDAItem | null>(null);
  const [form, setForm] = useState<AYDAForm | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    setForbidden(false);
    try {
      const response = await request<AYDAItemsData>("/reports/ojk/ayda/items");
      setItems(response.data?.items ?? []);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setForbidden(true);
      } else {
        setLoadError(
          err instanceof ApiError ? err.message : t.ojkData.ayda.loadError,
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
    { value: "01", label: t.ojkData.ayda.typeEmas },
    { value: "02", label: t.ojkData.ayda.typeTanahBangunan },
    { value: "03", label: t.ojkData.ayda.typeResiGudang },
    { value: "04", label: t.ojkData.ayda.typeTempatUsaha },
    { value: "05", label: t.ojkData.ayda.typeKendaraan },
    { value: "99", label: t.ojkData.ayda.typeLainnya },
  ];
  const statusOptions: Option[] = [
    { value: "AKTIF", label: t.ojkData.ayda.statusAktif },
    { value: "NONAKTIF", label: t.ojkData.ayda.statusNonaktif },
  ];

  const updateForm = (patch: Partial<AYDAForm>) =>
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

  const openEdit = (row: AYDAItem) => {
    resetMessages();
    setForm(itemFormFrom(row));
  };

  /** Batasan yang sama dengan domain.BuildAYDAItem, dicek sebelum menembak API. */
  const validate = (value: AYDAForm): string | null => {
    if (!COLLATERAL_TYPE_CODES.includes(value.collateral_type_code)) {
      return t.ojkData.ayda.validationType;
    }
    const address = value.collateral_address.trim();
    if (!address) return t.ojkData.ayda.validationAddress;
    if (address.length > 255) return t.ojkData.ayda.validationAddressLength;
    if (!value.acquisition_date)
      return t.ojkData.ayda.validationAcquisitionDate;
    if (!value.as_of) return t.ojkData.ayda.validationAsOf;
    if (
      value.initial_recognition_value < 0 ||
      value.accumulated_impairment < 0 ||
      value.net_realizable_value < 0
    ) {
      return t.ojkData.ayda.validationAmount;
    }
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
      await request<AYDAItem>("/reports/ojk/ayda/items", {
        method: "PUT",
        body: {
          id: form.id,
          collateral_type_code: form.collateral_type_code,
          collateral_address: form.collateral_address,
          acquisition_date: form.acquisition_date,
          initial_recognition_value: form.initial_recognition_value,
          accumulated_impairment: form.accumulated_impairment,
          net_realizable_value: form.net_realizable_value,
          as_of: form.as_of,
          status: form.status,
          note: form.note,
        },
      });
      setFeedback(t.ojkData.ayda.saved);
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
      await request(`/reports/ojk/ayda/items/${deleteTarget.id}`, {
        method: "DELETE",
      });
      setFeedback(t.ojkData.ayda.deleted);
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
    ? withCurrent(typeOptions, form.collateral_type_code)
    : typeOptions;
  const statusSelectOptions = form
    ? withCurrent(statusOptions, form.status)
    : statusOptions;

  const columns: Column<AYDAItem>[] = [
    {
      header: t.ojkData.ayda.colAsOf,
      cell: (row) => formatDateISO(row.as_of) || "-",
    },
    {
      header: t.ojkData.ayda.colCollateralType,
      cell: (row) => labelOf(typeOptions, row.collateral_type_code),
    },
    {
      header: t.ojkData.ayda.colAddress,
      accessorKey: "collateral_address",
    },
    {
      header: t.ojkData.ayda.colAcquisitionDate,
      cell: (row) => formatDateISO(row.acquisition_date) || "-",
    },
    {
      header: t.ojkData.ayda.colInitialRecognition,
      type: "money",
      accessorKey: "initial_recognition_value",
    },
    {
      header: t.ojkData.ayda.colAccumulatedImpairment,
      type: "money",
      accessorKey: "accumulated_impairment",
    },
    {
      header: t.ojkData.ayda.colNetRealizable,
      type: "money",
      accessorKey: "net_realizable_value",
    },
    {
      header: t.ojkData.ayda.colStatus,
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
        <CardTitle>{t.ojkData.ayda.title}</CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <LoadingState label={t.states.loading} />
        ) : forbidden ? (
          <ErrorState
            title={t.ojkData.ayda.title}
            description={t.ojkData.ayda.forbidden}
          />
        ) : loadError ? (
          <ErrorState
            title={t.ojkData.ayda.title}
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
              {t.ojkData.ayda.description}
            </p>
            {!canEdit && <Alert variant="info">{t.ojkData.readOnly}</Alert>}
            {feedback && <Alert variant="success">{feedback}</Alert>}
            {fieldError && <Alert variant="error">{fieldError}</Alert>}
            {saveError && <Alert variant="error">{saveError}</Alert>}

            <div className="flex items-center justify-between gap-4">
              <h4 className="text-title font-semibold text-ink-900">
                {t.ojkData.ayda.itemsTitle}
              </h4>
              {canEdit && !form && (
                <Button size="sm" onClick={openNew}>
                  {t.ojkData.ayda.add}
                </Button>
              )}
            </div>

            {form && (
              <form
                onSubmit={onSubmit}
                className="space-y-4 rounded-md border border-border p-4"
              >
                <h5 className="text-meta font-medium uppercase tracking-wide text-ink-600">
                  {form.id ? t.ojkData.ayda.editTitle : t.ojkData.ayda.newTitle}
                </h5>
                <div className="grid gap-4 md:grid-cols-2">
                  <Select
                    label={t.ojkData.ayda.fieldCollateralType}
                    helperText={t.ojkData.ayda.fieldCollateralTypeHint}
                    placeholder={t.ojkData.ayda.typePlaceholder}
                    value={form.collateral_type_code}
                    options={typeSelectOptions}
                    onChange={(event) =>
                      updateForm({ collateral_type_code: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.ayda.fieldAddress}
                    helperText={t.ojkData.ayda.fieldAddressHint}
                    value={form.collateral_address}
                    onChange={(event) =>
                      updateForm({ collateral_address: event.target.value })
                    }
                  />
                  <DateInput
                    label={t.ojkData.ayda.fieldAcquisitionDate}
                    helperText={t.ojkData.ayda.fieldAcquisitionDateHint}
                    value={form.acquisition_date}
                    onChange={(event) =>
                      updateForm({ acquisition_date: event.target.value })
                    }
                  />
                  <DateInput
                    label={t.ojkData.ayda.fieldAsOf}
                    helperText={t.ojkData.ayda.fieldAsOfHint}
                    value={form.as_of}
                    onChange={(event) =>
                      updateForm({ as_of: event.target.value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.ayda.fieldInitialRecognition}
                    helperText={t.ojkData.ayda.fieldInitialRecognitionHint}
                    value={form.initial_recognition_value}
                    onChange={(value) =>
                      updateForm({ initial_recognition_value: value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.ayda.fieldAccumulatedImpairment}
                    helperText={t.ojkData.ayda.fieldAccumulatedImpairmentHint}
                    value={form.accumulated_impairment}
                    onChange={(value) =>
                      updateForm({ accumulated_impairment: value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.ayda.fieldNetRealizable}
                    helperText={t.ojkData.ayda.fieldNetRealizableHint}
                    value={form.net_realizable_value}
                    onChange={(value) =>
                      updateForm({ net_realizable_value: value })
                    }
                  />
                  <Select
                    label={t.ojkData.ayda.fieldStatus}
                    value={form.status}
                    options={statusSelectOptions}
                    onChange={(event) =>
                      updateForm({ status: event.target.value })
                    }
                  />
                </div>

                <Textarea
                  label={t.ojkData.ayda.fieldNote}
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
              emptyMessage={t.ojkData.ayda.empty}
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
