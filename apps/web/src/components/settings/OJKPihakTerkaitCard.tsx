"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, request } from "@/lib/api";
import type {
  PihakTerkaitItem,
  PihakTerkaitItemsData,
} from "@/lib/operations-types";
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
import { withCurrent, labelOf, type Option } from "./register-options";

interface PihakTerkaitForm {
  id: string;
  nama: string;
  alamat: string;
  jenis_code: string;
  hubungan_code: string;
  note: string;
}

const NAMA_MAX = 255;

/** Sandi baku Form 00.05 kolom IV (domain/form00_05_pihak_terkait.go). */
const JENIS_CODES: string[] = ["01", "02", "03"];
/** Sandi baku Form 00.05 kolom V (domain/form00_05_pihak_terkait.go). */
const HUBUNGAN_CODES: string[] = ["01", "02", "03", "04", "05", "06"];

function formFrom(row: PihakTerkaitItem): PihakTerkaitForm {
  return {
    id: row.id,
    nama: row.nama ?? "",
    alamat: row.alamat ?? "",
    jenis_code: row.jenis_code ?? "",
    hubungan_code: row.hubungan_code ?? "",
    note: row.note ?? "",
  };
}

function emptyForm(): PihakTerkaitForm {
  return {
    id: "",
    nama: "",
    alamat: "",
    jenis_code: "",
    hubungan_code: "",
    note: "",
  };
}

/**
 * Kartu pengisian register Form 00.05 "Data Pihak Terkait Lainnya". Ini register
 * pelaporan yang berdiri sendiri: pihak terkait di sini BUKAN penandaan BMPK dan tidak
 * harus nasabah. Nomor identitas (NIK/NPWP) sengaja tidak diminta karena tidak disimpan.
 * Penjagaan izin sebenarnya di API (system:config).
 */
export function OJKPihakTerkaitCard() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const canEdit = hasPermission(user, "system:config");

  const [items, setItems] = useState<PihakTerkaitItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState<PihakTerkaitForm | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<PihakTerkaitItem | null>(
    null,
  );
  const [deleting, setDeleting] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    setForbidden(false);
    try {
      const response = await request<PihakTerkaitItemsData>(
        "/reports/ojk/pihak-terkait/items",
      );
      setItems(response.data?.items ?? []);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setForbidden(true);
      } else {
        setLoadError(
          err instanceof ApiError
            ? err.message
            : t.ojkData.pihakTerkait.loadError,
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
    { value: "01", label: t.ojkData.pihakTerkait.jenisPerorangan },
    { value: "02", label: t.ojkData.pihakTerkait.jenisBadan },
    { value: "03", label: t.ojkData.pihakTerkait.jenisPemerintah },
  ];
  const hubunganOptions: Option[] = [
    {
      value: "01",
      label: t.ojkData.pihakTerkait.hubunganPengendaliKeluarga,
    },
    {
      value: "02",
      label: t.ojkData.pihakTerkait.hubunganPerusahaanBukanBank,
    },
    {
      value: "03",
      label: t.ojkData.pihakTerkait.hubunganBPRLainDimiliki,
    },
    {
      value: "04",
      label: t.ojkData.pihakTerkait.hubunganBPRRangkapKomisaris,
    },
    {
      value: "05",
      label: t.ojkData.pihakTerkait.hubunganPerusahaanRangkap,
    },
    {
      value: "06",
      label: t.ojkData.pihakTerkait.hubunganPeminjamDijamin,
    },
  ];

  const updateForm = (patch: Partial<PihakTerkaitForm>) =>
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

  const openEdit = (row: PihakTerkaitItem) => {
    resetMessages();
    setForm(formFrom(row));
  };

  /** Batasan yang sama dengan domain.BuildPihakTerkaitItem, dicek sebelum menembak API. */
  const validate = (value: PihakTerkaitForm): string | null => {
    const nama = value.nama.trim();
    if (!nama) return t.ojkData.pihakTerkait.validationNama;
    if (nama.length > NAMA_MAX) {
      return t.ojkData.pihakTerkait.validationNamaLength;
    }
    if (!JENIS_CODES.includes(value.jenis_code)) {
      return t.ojkData.pihakTerkait.validationJenis;
    }
    if (!HUBUNGAN_CODES.includes(value.hubungan_code)) {
      return t.ojkData.pihakTerkait.validationHubungan;
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
      await request("/reports/ojk/pihak-terkait/items", {
        method: "PUT",
        body: {
          id: form.id || undefined,
          nama: form.nama,
          alamat: form.alamat,
          jenis_code: form.jenis_code,
          hubungan_code: form.hubungan_code,
          note: form.note,
        },
      });
      setFeedback(t.ojkData.pihakTerkait.saved);
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
      await request(`/reports/ojk/pihak-terkait/items/${deleteTarget.id}`, {
        method: "DELETE",
      });
      setFeedback(t.ojkData.pihakTerkait.deleted);
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
  const hubunganSelectOptions = form
    ? withCurrent(hubunganOptions, form.hubungan_code)
    : hubunganOptions;

  const columns: Column<PihakTerkaitItem>[] = [
    {
      header: t.ojkData.pihakTerkait.colNama,
      cell: (row) => row.nama || "-",
    },
    {
      header: t.ojkData.pihakTerkait.colAlamat,
      cell: (row) => row.alamat || "-",
    },
    {
      header: t.ojkData.pihakTerkait.colJenis,
      cell: (row) => labelOf(jenisOptions, row.jenis_code),
    },
    {
      header: t.ojkData.pihakTerkait.colHubungan,
      cell: (row) => labelOf(hubunganOptions, row.hubungan_code),
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
        <CardTitle>{t.ojkData.pihakTerkait.title}</CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <LoadingState label={t.states.loading} />
        ) : forbidden ? (
          <ErrorState
            title={t.ojkData.pihakTerkait.title}
            description={t.ojkData.pihakTerkait.forbidden}
          />
        ) : loadError ? (
          <ErrorState
            title={t.ojkData.pihakTerkait.title}
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
              {t.ojkData.pihakTerkait.description}
            </p>
            {!canEdit && <Alert variant="info">{t.ojkData.readOnly}</Alert>}
            {feedback && <Alert variant="success">{feedback}</Alert>}
            {fieldError && <Alert variant="error">{fieldError}</Alert>}
            {saveError && <Alert variant="error">{saveError}</Alert>}

            <div className="flex items-center justify-between">
              <h4 className="text-title font-semibold text-ink-900">
                {t.ojkData.pihakTerkait.itemsTitle}
              </h4>
              {canEdit && (
                <Button size="sm" onClick={openCreate}>
                  {t.ojkData.pihakTerkait.addTitle}
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
                    ? t.ojkData.pihakTerkait.editTitle
                    : t.ojkData.pihakTerkait.addTitle}
                </h5>
                <div className="grid gap-4 md:grid-cols-2">
                  <Input
                    label={t.ojkData.pihakTerkait.fieldNama}
                    helperText={t.ojkData.pihakTerkait.fieldNamaHint}
                    value={form.nama}
                    onChange={(event) =>
                      updateForm({ nama: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.pihakTerkait.fieldAlamat}
                    helperText={t.ojkData.pihakTerkait.fieldAlamatHint}
                    value={form.alamat}
                    onChange={(event) =>
                      updateForm({ alamat: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.pihakTerkait.fieldJenis}
                    helperText={t.ojkData.pihakTerkait.fieldJenisHint}
                    placeholder={t.ojkData.pihakTerkait.jenisPlaceholder}
                    value={form.jenis_code}
                    options={jenisSelectOptions}
                    onChange={(event) =>
                      updateForm({ jenis_code: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.pihakTerkait.fieldHubungan}
                    helperText={t.ojkData.pihakTerkait.fieldHubunganHint}
                    placeholder={t.ojkData.pihakTerkait.hubunganPlaceholder}
                    value={form.hubungan_code}
                    options={hubunganSelectOptions}
                    onChange={(event) =>
                      updateForm({ hubungan_code: event.target.value })
                    }
                  />
                </div>
                <Textarea
                  label={t.ojkData.pihakTerkait.fieldNote}
                  helperText={t.ojkData.pihakTerkait.fieldNoteHint}
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
              emptyMessage={t.ojkData.pihakTerkait.empty}
              zebra
            />
          </>
        )}
      </CardContent>

      <ConfirmDialog
        open={deleteTarget !== null}
        title={t.ojkData.pihakTerkait.editTitle}
        description={t.ojkData.pihakTerkait.deleteConfirm}
        confirmLabel={deleting ? t.ojkData.saving : t.ojkData.delete}
        destructive
        loading={deleting}
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </Card>
  );
}
