"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, request } from "@/lib/api";
import type { HapusBukuItem, HapusBukuItemsData } from "@/lib/operations-types";
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

interface HapusBukuForm {
  id: string;
  jenis_aset_code: string;
  pihak_lawan_id: string;
  nomor_rekening: string;
  jenis_debitur_code: string;
  hubungan_bank_code: string;
  tanggal_hapus_buku: string;
  saldo_pokok_saat_hapus: string;
  saldo_pokok_akum_tertagih: string;
  saldo_pokok_per_posisi: string;
  bunga_saat_hapus: string;
  bunga_akum_tertagih: string;
  bunga_akum_tambahan: string;
  bunga_per_posisi: string;
  agunan_jenis_code: string;
  agunan_alamat: string;
  agunan_nilai: string;
  note: string;
}

/** Sandi baku Form 15.00 kolom IV (domain/form15_00_hapus_buku.go). */
const JENIS_ASET_CODES: string[] = ["10", "20"];
/** Sandi baku Form 15.00 kolom VI (domain/form15_00_hapus_buku.go). */
const HUBUNGAN_CODES: string[] = ["11", "12", "20"];

function amountToString(value: string | number | null | undefined): string {
  return value != null ? String(value) : "";
}

function formFrom(row: HapusBukuItem): HapusBukuForm {
  return {
    id: row.id,
    jenis_aset_code: row.jenis_aset_code ?? "",
    pihak_lawan_id: row.pihak_lawan_id ?? "",
    nomor_rekening: row.nomor_rekening ?? "",
    jenis_debitur_code: row.jenis_debitur_code ?? "",
    hubungan_bank_code: row.hubungan_bank_code ?? "",
    tanggal_hapus_buku: formatDateISO(row.tanggal_hapus_buku),
    saldo_pokok_saat_hapus: amountToString(row.saldo_pokok_saat_hapus),
    saldo_pokok_akum_tertagih: amountToString(row.saldo_pokok_akum_tertagih),
    saldo_pokok_per_posisi: amountToString(row.saldo_pokok_per_posisi),
    bunga_saat_hapus: amountToString(row.bunga_saat_hapus),
    bunga_akum_tertagih: amountToString(row.bunga_akum_tertagih),
    bunga_akum_tambahan: amountToString(row.bunga_akum_tambahan),
    bunga_per_posisi: amountToString(row.bunga_per_posisi),
    agunan_jenis_code: row.agunan_jenis_code ?? "",
    agunan_alamat: row.agunan_alamat ?? "",
    agunan_nilai: amountToString(row.agunan_nilai),
    note: row.note ?? "",
  };
}

function emptyForm(): HapusBukuForm {
  return {
    id: "",
    jenis_aset_code: "",
    pihak_lawan_id: "",
    nomor_rekening: "",
    jenis_debitur_code: "",
    hubungan_bank_code: "",
    tanggal_hapus_buku: "",
    saldo_pokok_saat_hapus: "",
    saldo_pokok_akum_tertagih: "",
    saldo_pokok_per_posisi: "",
    bunga_saat_hapus: "",
    bunga_akum_tertagih: "",
    bunga_akum_tambahan: "",
    bunga_per_posisi: "",
    agunan_jenis_code: "",
    agunan_alamat: "",
    agunan_nilai: "",
    note: "",
  };
}

const AMOUNT_FIELDS: (keyof HapusBukuForm)[] = [
  "saldo_pokok_saat_hapus",
  "saldo_pokok_akum_tertagih",
  "saldo_pokok_per_posisi",
  "bunga_saat_hapus",
  "bunga_akum_tertagih",
  "bunga_akum_tambahan",
  "bunga_per_posisi",
  "agunan_nilai",
];

/**
 * Kartu pengisian register Form 15.00 "Daftar Aset Produktif yang Dihapus Buku". Nominal
 * adalah isian bank (rupiah penuh); field kosong dianggap 0. Penjagaan izin sebenarnya di
 * API (system:config).
 */
export function OJKHapusBukuCard() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const canEdit = hasPermission(user, "system:config");

  const [items, setItems] = useState<HapusBukuItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState<HapusBukuForm | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<HapusBukuItem | null>(null);
  const [deleting, setDeleting] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    setForbidden(false);
    try {
      const response = await request<HapusBukuItemsData>(
        "/reports/ojk/hapus-buku/items",
      );
      setItems(response.data?.items ?? []);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setForbidden(true);
      } else {
        setLoadError(
          err instanceof ApiError ? err.message : t.ojkData.hapusBuku.loadError,
        );
      }
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    load();
  }, [load]);

  const jenisAsetOptions: Option[] = [
    { value: "10", label: t.ojkData.hapusBuku.jenisKredit },
    { value: "20", label: t.ojkData.hapusBuku.jenisPenempatan },
  ];
  const hubunganOptions: Option[] = [
    { value: "11", label: t.ojkData.hapusBuku.hubunganKesejahteraan },
    { value: "12", label: t.ojkData.hapusBuku.hubunganTerkaitLain },
    { value: "20", label: t.ojkData.hapusBuku.hubunganTidakTerkait },
  ];

  const updateForm = (patch: Partial<HapusBukuForm>) =>
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

  const openEdit = (row: HapusBukuItem) => {
    resetMessages();
    setForm(formFrom(row));
  };

  const validate = (value: HapusBukuForm): string | null => {
    if (!JENIS_ASET_CODES.includes(value.jenis_aset_code)) {
      return t.ojkData.hapusBuku.validationJenisAset;
    }
    if (!HUBUNGAN_CODES.includes(value.hubungan_bank_code)) {
      return t.ojkData.hapusBuku.validationHubungan;
    }
    if (!value.tanggal_hapus_buku) {
      return t.ojkData.hapusBuku.validationTanggal;
    }
    for (const field of AMOUNT_FIELDS) {
      const raw = String(value[field] ?? "").trim();
      if (raw === "") continue;
      if (!/^\d+(\.\d+)?$/.test(raw)) {
        return t.ojkData.hapusBuku.validationNominal;
      }
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
      await request("/reports/ojk/hapus-buku/items", {
        method: "PUT",
        body: {
          id: form.id || undefined,
          jenis_aset_code: form.jenis_aset_code,
          pihak_lawan_id: form.pihak_lawan_id,
          nomor_rekening: form.nomor_rekening,
          jenis_debitur_code: form.jenis_debitur_code,
          hubungan_bank_code: form.hubungan_bank_code,
          tanggal_hapus_buku: form.tanggal_hapus_buku,
          saldo_pokok_saat_hapus: form.saldo_pokok_saat_hapus.trim(),
          saldo_pokok_akum_tertagih: form.saldo_pokok_akum_tertagih.trim(),
          saldo_pokok_per_posisi: form.saldo_pokok_per_posisi.trim(),
          bunga_saat_hapus: form.bunga_saat_hapus.trim(),
          bunga_akum_tertagih: form.bunga_akum_tertagih.trim(),
          bunga_akum_tambahan: form.bunga_akum_tambahan.trim(),
          bunga_per_posisi: form.bunga_per_posisi.trim(),
          agunan_jenis_code: form.agunan_jenis_code,
          agunan_alamat: form.agunan_alamat,
          agunan_nilai: form.agunan_nilai.trim(),
          note: form.note,
        },
      });
      setFeedback(t.ojkData.hapusBuku.saved);
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
      await request(`/reports/ojk/hapus-buku/items/${deleteTarget.id}`, {
        method: "DELETE",
      });
      setFeedback(t.ojkData.hapusBuku.deleted);
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

  const jenisAsetSelectOptions = form
    ? withCurrent(jenisAsetOptions, form.jenis_aset_code)
    : jenisAsetOptions;
  const hubunganSelectOptions = form
    ? withCurrent(hubunganOptions, form.hubungan_bank_code)
    : hubunganOptions;

  const columns: Column<HapusBukuItem>[] = [
    {
      header: t.ojkData.hapusBuku.colTanggal,
      cell: (row) => formatDateISO(row.tanggal_hapus_buku) || "-",
      isMono: true,
    },
    {
      header: t.ojkData.hapusBuku.colJenisAset,
      cell: (row) => labelOf(jenisAsetOptions, row.jenis_aset_code),
    },
    {
      header: t.ojkData.hapusBuku.colRekening,
      cell: (row) => row.nomor_rekening || "-",
      isMono: true,
    },
    {
      header: t.ojkData.hapusBuku.colPokokPosisi,
      type: "money",
      accessorKey: "saldo_pokok_per_posisi",
    },
    {
      header: t.ojkData.hapusBuku.colBungaPosisi,
      type: "money",
      accessorKey: "bunga_per_posisi",
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

  /** Field nominal Form 15.00 (rupiah penuh, teks desimal). */
  const amountField = (
    key: keyof HapusBukuForm,
    label: string,
  ): React.ReactNode => (
    <Input
      key={key}
      inputMode="decimal"
      label={label}
      helperText={t.ojkData.hapusBuku.nominalHint}
      value={form ? String(form[key] ?? "") : ""}
      onChange={(event) => updateForm({ [key]: event.target.value })}
    />
  );

  return (
    <Card className="mb-4">
      <CardHeader>
        <CardTitle>{t.ojkData.hapusBuku.title}</CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <LoadingState label={t.states.loading} />
        ) : forbidden ? (
          <ErrorState
            title={t.ojkData.hapusBuku.title}
            description={t.ojkData.hapusBuku.forbidden}
          />
        ) : loadError ? (
          <ErrorState
            title={t.ojkData.hapusBuku.title}
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
              {t.ojkData.hapusBuku.description}
            </p>
            {!canEdit && <Alert variant="info">{t.ojkData.readOnly}</Alert>}
            {feedback && <Alert variant="success">{feedback}</Alert>}
            {fieldError && <Alert variant="error">{fieldError}</Alert>}
            {saveError && <Alert variant="error">{saveError}</Alert>}

            <div className="flex items-center justify-between">
              <h4 className="text-title font-semibold text-ink-900">
                {t.ojkData.hapusBuku.itemsTitle}
              </h4>
              {canEdit && (
                <Button size="sm" onClick={openCreate}>
                  {t.ojkData.hapusBuku.addTitle}
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
                    ? t.ojkData.hapusBuku.editTitle
                    : t.ojkData.hapusBuku.addTitle}
                </h5>
                <div className="grid gap-4 md:grid-cols-2">
                  <Select
                    label={t.ojkData.hapusBuku.fieldJenisAset}
                    helperText={t.ojkData.hapusBuku.fieldJenisAsetHint}
                    placeholder={t.ojkData.hapusBuku.jenisPlaceholder}
                    value={form.jenis_aset_code}
                    options={jenisAsetSelectOptions}
                    onChange={(event) =>
                      updateForm({ jenis_aset_code: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.hapusBuku.fieldHubungan}
                    helperText={t.ojkData.hapusBuku.fieldHubunganHint}
                    placeholder={t.ojkData.hapusBuku.hubunganPlaceholder}
                    value={form.hubungan_bank_code}
                    options={hubunganSelectOptions}
                    onChange={(event) =>
                      updateForm({ hubungan_bank_code: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.hapusBuku.fieldPihakLawan}
                    helperText={t.ojkData.hapusBuku.fieldPihakLawanHint}
                    value={form.pihak_lawan_id}
                    onChange={(event) =>
                      updateForm({ pihak_lawan_id: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.hapusBuku.fieldNomorRekening}
                    helperText={t.ojkData.hapusBuku.fieldNomorRekeningHint}
                    value={form.nomor_rekening}
                    onChange={(event) =>
                      updateForm({ nomor_rekening: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.hapusBuku.fieldJenisDebitur}
                    helperText={t.ojkData.hapusBuku.fieldJenisDebiturHint}
                    value={form.jenis_debitur_code}
                    onChange={(event) =>
                      updateForm({ jenis_debitur_code: event.target.value })
                    }
                  />
                  <Input
                    type="date"
                    label={t.ojkData.hapusBuku.fieldTanggal}
                    helperText={t.ojkData.hapusBuku.fieldTanggalHint}
                    value={form.tanggal_hapus_buku}
                    onChange={(event) =>
                      updateForm({ tanggal_hapus_buku: event.target.value })
                    }
                  />
                </div>

                <h5 className="text-meta font-medium uppercase tracking-wide text-ink-600">
                  {t.ojkData.hapusBuku.sectionPokok}
                </h5>
                <div className="grid gap-4 md:grid-cols-3">
                  {amountField(
                    "saldo_pokok_saat_hapus",
                    t.ojkData.hapusBuku.fieldPokokSaatHapus,
                  )}
                  {amountField(
                    "saldo_pokok_akum_tertagih",
                    t.ojkData.hapusBuku.fieldPokokAkumTertagih,
                  )}
                  {amountField(
                    "saldo_pokok_per_posisi",
                    t.ojkData.hapusBuku.fieldPokokPerPosisi,
                  )}
                </div>

                <h5 className="text-meta font-medium uppercase tracking-wide text-ink-600">
                  {t.ojkData.hapusBuku.sectionBunga}
                </h5>
                <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
                  {amountField(
                    "bunga_saat_hapus",
                    t.ojkData.hapusBuku.fieldBungaSaatHapus,
                  )}
                  {amountField(
                    "bunga_akum_tertagih",
                    t.ojkData.hapusBuku.fieldBungaAkumTertagih,
                  )}
                  {amountField(
                    "bunga_akum_tambahan",
                    t.ojkData.hapusBuku.fieldBungaAkumTambahan,
                  )}
                  {amountField(
                    "bunga_per_posisi",
                    t.ojkData.hapusBuku.fieldBungaPerPosisi,
                  )}
                </div>

                <h5 className="text-meta font-medium uppercase tracking-wide text-ink-600">
                  {t.ojkData.hapusBuku.sectionAgunan}
                </h5>
                <div className="grid gap-4 md:grid-cols-3">
                  <Input
                    label={t.ojkData.hapusBuku.fieldAgunanJenis}
                    helperText={t.ojkData.hapusBuku.fieldAgunanJenisHint}
                    value={form.agunan_jenis_code}
                    onChange={(event) =>
                      updateForm({ agunan_jenis_code: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.hapusBuku.fieldAgunanAlamat}
                    helperText={t.ojkData.hapusBuku.fieldAgunanAlamatHint}
                    value={form.agunan_alamat}
                    onChange={(event) =>
                      updateForm({ agunan_alamat: event.target.value })
                    }
                  />
                  {amountField(
                    "agunan_nilai",
                    t.ojkData.hapusBuku.fieldAgunanNilai,
                  )}
                </div>

                <Textarea
                  label={t.ojkData.hapusBuku.fieldNote}
                  helperText={t.ojkData.hapusBuku.fieldNoteHint}
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
              emptyMessage={t.ojkData.hapusBuku.empty}
              zebra
            />
          </>
        )}
      </CardContent>

      <ConfirmDialog
        open={deleteTarget !== null}
        title={t.ojkData.hapusBuku.editTitle}
        description={t.ojkData.hapusBuku.deleteConfirm}
        confirmLabel={deleting ? t.ojkData.saving : t.ojkData.delete}
        destructive
        loading={deleting}
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </Card>
  );
}
