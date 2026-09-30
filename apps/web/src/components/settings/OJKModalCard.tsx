"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, request } from "@/lib/api";
import type { ModalItem, ModalItemsData } from "@/lib/operations-types";
import { formatDateISO } from "@/lib/format";
import { useAuth } from "@/lib/useAuth";
import { hasPermission } from "@/lib/permissions";
import { useTranslation } from "@/i18n/context";
import { Alert } from "@/components/ui/Alert";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { DataTable, Column } from "@/components/ui/DataTable";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { ErrorState, LoadingState } from "@/components/ui/States";

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

interface ModalForm {
  id: string;
  jenis_code: string;
  tanggal_persetujuan: string;
  jenis_modal_code: string;
  jumlah: string;
  note: string;
}

/** Sandi baku Form 00.06 kolom I (domain/form00_06_modal.go). */
const JENIS_CODES: string[] = ["01", "02", "03"];
/** Sandi baku Form 00.06 kolom III (domain/form00_06_modal.go). */
const JENIS_MODAL_CODES: string[] = ["01", "02", "03"];

function formFrom(row: ModalItem): ModalForm {
  return {
    id: row.id,
    jenis_code: row.jenis_code ?? "",
    tanggal_persetujuan: formatDateISO(row.tanggal_persetujuan),
    jenis_modal_code: row.jenis_modal_code ?? "",
    jumlah: row.jumlah != null ? String(row.jumlah) : "",
    note: row.note ?? "",
  };
}

function emptyForm(): ModalForm {
  return {
    id: "",
    jenis_code: "",
    tanggal_persetujuan: "",
    jenis_modal_code: "",
    jumlah: "",
    note: "",
  };
}

/**
 * Kartu pengisian register Form 00.06 "Daftar Modal Disetor, Modal Sumbangan, dan Dana
 * Setoran Modal - Ekuitas". Nominal adalah isian bank (rupiah penuh), bukan turunan saldo
 * bagan akun. Penjagaan izin sebenarnya di API (system:config).
 */
export function OJKModalCard() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const canEdit = hasPermission(user, "system:config");

  const [items, setItems] = useState<ModalItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState<ModalForm | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<ModalItem | null>(null);
  const [deleting, setDeleting] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    setForbidden(false);
    try {
      const response = await request<ModalItemsData>(
        "/reports/ojk/modal/items",
      );
      setItems(response.data?.items ?? []);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setForbidden(true);
      } else {
        setLoadError(
          err instanceof ApiError ? err.message : t.ojkData.modal.loadError,
        );
      }
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    load();
  }, [load]);

  const jenisOptions: Option[] = [
    { value: "01", label: t.ojkData.modal.jenisDana },
    { value: "02", label: t.ojkData.modal.jenisTanahBangunanInti },
    { value: "03", label: t.ojkData.modal.jenisTanahBangunanNonInti },
  ];
  const jenisModalOptions: Option[] = [
    { value: "01", label: t.ojkData.modal.jenisModalDisetor },
    { value: "02", label: t.ojkData.modal.jenisModalSumbangan },
    { value: "03", label: t.ojkData.modal.jenisModalDanaSetoranEkuitas },
  ];

  const updateForm = (patch: Partial<ModalForm>) =>
    setForm((prev) => (prev ? { ...prev, ...patch } : prev));

  const resetMessages = () => {
    setFeedback(null);
    setFieldError(null);
    setSaveError(null);
  };

  const openCreate = () => {
    resetMessages();
    setForm(emptyForm());
  };

  const openEdit = (row: ModalItem) => {
    resetMessages();
    setForm(formFrom(row));
  };

  /** Batasan yang sama dengan domain.BuildModalItem, dicek sebelum menembak API. */
  const validate = (value: ModalForm): string | null => {
    if (!JENIS_CODES.includes(value.jenis_code)) {
      return t.ojkData.modal.validationJenis;
    }
    if (!JENIS_MODAL_CODES.includes(value.jenis_modal_code)) {
      return t.ojkData.modal.validationJenisModal;
    }
    const jumlah = value.jumlah.trim();
    if (!jumlah) return t.ojkData.modal.validationJumlah;
    if (!/^\d+(\.\d+)?$/.test(jumlah)) {
      return t.ojkData.modal.validationJumlah;
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
      await request("/reports/ojk/modal/items", {
        method: "PUT",
        body: {
          id: form.id || undefined,
          jenis_code: form.jenis_code,
          tanggal_persetujuan: form.tanggal_persetujuan,
          jenis_modal_code: form.jenis_modal_code,
          jumlah: form.jumlah.trim(),
          note: form.note,
        },
      });
      setFeedback(t.ojkData.modal.saved);
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
      await request(`/reports/ojk/modal/items/${deleteTarget.id}`, {
        method: "DELETE",
      });
      setFeedback(t.ojkData.modal.deleted);
      setDeleteTarget(null);
      await load();
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setSaveError(t.ojkData.saveForbidden);
      } else {
        setSaveError(
          err instanceof ApiError ? err.message : t.ojkData.saveError,
        );
      }
      setDeleteTarget(null);
    } finally {
      setDeleting(false);
    }
  };

  const jenisSelectOptions = form
    ? withCurrent(jenisOptions, form.jenis_code)
    : jenisOptions;
  const jenisModalSelectOptions = form
    ? withCurrent(jenisModalOptions, form.jenis_modal_code)
    : jenisModalOptions;

  const columns: Column<ModalItem>[] = [
    {
      header: t.ojkData.modal.colTanggal,
      cell: (row) => formatDateISO(row.tanggal_persetujuan) || "-",
      isMono: true,
    },
    {
      header: t.ojkData.modal.colJenisModal,
      cell: (row) => labelOf(jenisModalOptions, row.jenis_modal_code),
    },
    {
      header: t.ojkData.modal.colJumlah,
      type: "money",
      accessorKey: "jumlah",
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
        <CardTitle>{t.ojkData.modal.title}</CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <LoadingState label={t.states.loading} />
        ) : forbidden ? (
          <ErrorState
            title={t.ojkData.modal.title}
            description={t.ojkData.modal.forbidden}
          />
        ) : loadError ? (
          <ErrorState
            title={t.ojkData.modal.title}
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
              {t.ojkData.modal.description}
            </p>
            {!canEdit && <Alert variant="info">{t.ojkData.readOnly}</Alert>}
            {feedback && <Alert variant="success">{feedback}</Alert>}
            {fieldError && <Alert variant="error">{fieldError}</Alert>}
            {saveError && <Alert variant="error">{saveError}</Alert>}

            <div className="flex items-center justify-between">
              <h4 className="text-title font-semibold text-ink-900">
                {t.ojkData.modal.itemsTitle}
              </h4>
              {canEdit && (
                <Button size="sm" onClick={openCreate}>
                  {t.ojkData.modal.addTitle}
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
                    ? t.ojkData.modal.editTitle
                    : t.ojkData.modal.addTitle}
                </h5>
                <div className="grid gap-4 md:grid-cols-2">
                  <Select
                    label={t.ojkData.modal.fieldJenis}
                    helperText={t.ojkData.modal.fieldJenisHint}
                    placeholder={t.ojkData.modal.jenisPlaceholder}
                    value={form.jenis_code}
                    options={jenisSelectOptions}
                    onChange={(event) =>
                      updateForm({ jenis_code: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.modal.fieldJenisModal}
                    helperText={t.ojkData.modal.fieldJenisModalHint}
                    placeholder={t.ojkData.modal.jenisModalPlaceholder}
                    value={form.jenis_modal_code}
                    options={jenisModalSelectOptions}
                    onChange={(event) =>
                      updateForm({ jenis_modal_code: event.target.value })
                    }
                  />
                  <Input
                    type="date"
                    label={t.ojkData.modal.fieldTanggal}
                    helperText={t.ojkData.modal.fieldTanggalHint}
                    value={form.tanggal_persetujuan}
                    onChange={(event) =>
                      updateForm({ tanggal_persetujuan: event.target.value })
                    }
                  />
                  <Input
                    inputMode="decimal"
                    label={t.ojkData.modal.fieldJumlah}
                    helperText={t.ojkData.modal.fieldJumlahHint}
                    value={form.jumlah}
                    onChange={(event) =>
                      updateForm({ jumlah: event.target.value })
                    }
                  />
                </div>
                <Textarea
                  label={t.ojkData.modal.fieldNote}
                  helperText={t.ojkData.modal.fieldNoteHint}
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
              emptyMessage={t.ojkData.modal.empty}
              zebra
            />
          </>
        )}
      </CardContent>

      <ConfirmDialog
        open={deleteTarget !== null}
        title={t.ojkData.modal.editTitle}
        description={t.ojkData.modal.deleteConfirm}
        confirmLabel={deleting ? t.ojkData.saving : t.ojkData.delete}
        destructive
        loading={deleting}
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </Card>
  );
}
