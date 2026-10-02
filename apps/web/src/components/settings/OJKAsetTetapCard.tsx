"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, request } from "@/lib/api";
import type { AsetTetapItem, AsetTetapItemsData } from "@/lib/operations-types";
import { formatDateISO, formatMoney } from "@/lib/format";
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
import { Select } from "@/components/ui/Select";
import { ErrorState, LoadingState } from "@/components/ui/States";
import { Textarea } from "@/components/ui/Textarea";
import { withCurrent, labelOf, type Option } from "./register-options";

interface AsetTetapForm {
  id: string;
  jenis_aset_code: string;
  sumber_perolehan_code: string;
  status_aset_code: string;
  biaya_perolehan: number;
  akumulasi_penyusutan_amortisasi: number;
  akumulasi_kerugian_penurunan_nilai: number;
  metode_pengukuran_code: string;
  as_of: string;
  status: string;
  note: string;
}

const EMPTY_ITEM: AsetTetapForm = {
  id: "",
  jenis_aset_code: "",
  sumber_perolehan_code: "",
  status_aset_code: "",
  biaya_perolehan: 0,
  akumulasi_penyusutan_amortisasi: 0,
  akumulasi_kerugian_penurunan_nilai: 0,
  metode_pengukuran_code: "",
  as_of: "",
  status: "AKTIF",
  note: "",
};

/** Sandi baku Form 08.00 (domain/aset_tetap.go). */
const JENIS_CODES: string[] = [
  "101",
  "102",
  "103",
  "104",
  "199",
  "201",
  "202",
  "299",
];
const SUMBER_CODES: string[] = ["01", "02", "03", "99"];
const STATUS_ASET_CODES: string[] = ["1", "2"];
const METODE_CODES: string[] = ["1", "2"];

function itemFormFrom(row: AsetTetapItem): AsetTetapForm {
  return {
    id: row.id,
    jenis_aset_code: row.jenis_aset_code ?? "",
    sumber_perolehan_code: row.sumber_perolehan_code ?? "",
    status_aset_code: row.status_aset_code ?? "",
    biaya_perolehan: Number(row.biaya_perolehan) || 0,
    akumulasi_penyusutan_amortisasi:
      Number(row.akumulasi_penyusutan_amortisasi) || 0,
    akumulasi_kerugian_penurunan_nilai:
      Number(row.akumulasi_kerugian_penurunan_nilai) || 0,
    metode_pengukuran_code: row.metode_pengukuran_code ?? "",
    as_of: formatDateISO(row.as_of),
    status: row.status ?? "",
    note: row.note ?? "",
  };
}

/**
 * Kolom VIII Nilai Tercatat: biaya perolehan (V) dikurangi akumulasi penyusutan
 * atau amortisasi (VI) dan akumulasi kerugian penurunan nilai (VII). Domain
 * menghitungnya lewat AsetTetapItem.NilaiTercatat dan TIDAK menyerialkannya ke
 * klien (metode, bukan bidang), jadi rumus yang sama dihitung di sini. null bila
 * hasilnya negatif supaya ditulis "-", sama seperti keputusan domain.
 */
function nilaiTercatat(row: AsetTetapItem): number | null {
  const biaya = Number(row.biaya_perolehan);
  const penyusutan = Number(row.akumulasi_penyusutan_amortisasi);
  const kerugian = Number(row.akumulasi_kerugian_penurunan_nilai);
  if (
    !Number.isFinite(biaya) ||
    !Number.isFinite(penyusutan) ||
    !Number.isFinite(kerugian)
  ) {
    return null;
  }
  const net = biaya - penyusutan - kerugian;
  return Number.isFinite(net) && net >= 0 ? net : null;
}

/**
 * Kartu pengisian register aset tetap, inventaris, dan aset tidak berwujud Form
 * 08.00: satu baris per aset. Data sebelumnya hanya dapat diisi lewat API/SQL.
 * Penjagaan izin sebenarnya di API (system:config); kartu hanya menyembunyikan
 * kontrol tulis saat peran tidak memegang izin itu. Form 08.00 menggabungkan
 * baris per kombinasi jenis/sumber/status/metode saat laporan dibuat, bukan di
 * layar ini. Kolom VIII Nilai Tercatat turunan, jadi tidak diminta diisi. Status
 * Aset boleh kosong (aset tidak berwujud, sandi jenis 2xx). Penghapusan DELETE
 * fisik: form ini tidak mengatur no reuse/no recycle nomor register.
 */
export function OJKAsetTetapCard() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const canEdit = hasPermission(user, "system:config");

  const [items, setItems] = useState<AsetTetapItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<AsetTetapItem | null>(null);
  const [form, setForm] = useState<AsetTetapForm | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    setForbidden(false);
    try {
      const response = await request<AsetTetapItemsData>(
        "/reports/ojk/aset-tetap/items",
      );
      setItems(response.data?.items ?? []);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setForbidden(true);
      } else {
        setLoadError(
          err instanceof ApiError ? err.message : t.ojkData.asetTetap.loadError,
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
    { value: "101", label: t.ojkData.asetTetap.jenisTanah },
    { value: "102", label: t.ojkData.asetTetap.jenisBangunan },
    { value: "103", label: t.ojkData.asetTetap.jenisPeralatan },
    { value: "104", label: t.ojkData.asetTetap.jenisKendaraan },
    { value: "199", label: t.ojkData.asetTetap.jenisAsetTetapLainnya },
    { value: "201", label: t.ojkData.asetTetap.jenisSoftware },
    { value: "202", label: t.ojkData.asetTetap.jenisGoodwill },
    { value: "299", label: t.ojkData.asetTetap.jenisTidakBerwujudLainnya },
  ];
  const sumberOptions: Option[] = [
    { value: "01", label: t.ojkData.asetTetap.sumberSewaPembiayaan },
    { value: "02", label: t.ojkData.asetTetap.sumberModalDisetor },
    { value: "03", label: t.ojkData.asetTetap.sumberModalSumbangan },
    { value: "99", label: t.ojkData.asetTetap.sumberLainnya },
  ];
  const statusAsetOptions: Option[] = [
    { value: "1", label: t.ojkData.asetTetap.statusAsetDijaminkan },
    { value: "2", label: t.ojkData.asetTetap.statusAsetTidakDijaminkan },
  ];
  const metodeOptions: Option[] = [
    { value: "1", label: t.ojkData.asetTetap.metodeBiaya },
    { value: "2", label: t.ojkData.asetTetap.metodeRevaluasi },
  ];
  const statusOptions: Option[] = [
    { value: "AKTIF", label: t.ojkData.asetTetap.statusAktif },
    { value: "NONAKTIF", label: t.ojkData.asetTetap.statusNonaktif },
  ];

  const updateForm = (patch: Partial<AsetTetapForm>) =>
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

  const openEdit = (row: AsetTetapItem) => {
    resetMessages();
    setForm(itemFormFrom(row));
  };

  /** Batasan yang sama dengan domain.BuildAsetTetapItem, dicek sebelum menembak API. */
  const validate = (value: AsetTetapForm): string | null => {
    if (!JENIS_CODES.includes(value.jenis_aset_code)) {
      return t.ojkData.asetTetap.validationJenis;
    }
    if (!SUMBER_CODES.includes(value.sumber_perolehan_code)) {
      return t.ojkData.asetTetap.validationSumber;
    }
    if (
      value.status_aset_code &&
      !STATUS_ASET_CODES.includes(value.status_aset_code)
    ) {
      return t.ojkData.asetTetap.validationStatusAset;
    }
    if (value.biaya_perolehan < 0) {
      return t.ojkData.asetTetap.validationBiaya;
    }
    if (value.akumulasi_penyusutan_amortisasi < 0) {
      return t.ojkData.asetTetap.validationAkumulasiPenyusutan;
    }
    if (value.akumulasi_kerugian_penurunan_nilai < 0) {
      return t.ojkData.asetTetap.validationAkumulasiKerugian;
    }
    if (!METODE_CODES.includes(value.metode_pengukuran_code)) {
      return t.ojkData.asetTetap.validationMetode;
    }
    if (!value.as_of) return t.ojkData.asetTetap.validationAsOf;
    if (value.status !== "AKTIF" && value.status !== "NONAKTIF") {
      return t.ojkData.asetTetap.validationStatus;
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
      await request<AsetTetapItem>("/reports/ojk/aset-tetap/items", {
        method: "PUT",
        body: {
          id: form.id,
          jenis_aset_code: form.jenis_aset_code,
          sumber_perolehan_code: form.sumber_perolehan_code,
          status_aset_code: form.status_aset_code,
          biaya_perolehan: form.biaya_perolehan,
          akumulasi_penyusutan_amortisasi: form.akumulasi_penyusutan_amortisasi,
          akumulasi_kerugian_penurunan_nilai:
            form.akumulasi_kerugian_penurunan_nilai,
          metode_pengukuran_code: form.metode_pengukuran_code,
          as_of: form.as_of,
          status: form.status,
          note: form.note,
        },
      });
      setFeedback(t.ojkData.asetTetap.saved);
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
      await request(`/reports/ojk/aset-tetap/items/${deleteTarget.id}`, {
        method: "DELETE",
      });
      setFeedback(t.ojkData.asetTetap.deleted);
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

  const jenisSelectOptions = form
    ? withCurrent(jenisOptions, form.jenis_aset_code)
    : jenisOptions;
  const sumberSelectOptions = form
    ? withCurrent(sumberOptions, form.sumber_perolehan_code)
    : sumberOptions;
  const statusAsetSelectOptions = form
    ? withCurrent(statusAsetOptions, form.status_aset_code)
    : statusAsetOptions;
  const metodeSelectOptions = form
    ? withCurrent(metodeOptions, form.metode_pengukuran_code)
    : metodeOptions;
  const statusSelectOptions = form
    ? withCurrent(statusOptions, form.status)
    : statusOptions;

  const columns: Column<AsetTetapItem>[] = [
    {
      header: t.ojkData.asetTetap.colJenis,
      cell: (row) => labelOf(jenisOptions, row.jenis_aset_code),
    },
    {
      header: t.ojkData.asetTetap.colSumber,
      cell: (row) => labelOf(sumberOptions, row.sumber_perolehan_code),
    },
    {
      header: t.ojkData.asetTetap.colStatusAset,
      cell: (row) =>
        row.status_aset_code
          ? labelOf(statusAsetOptions, row.status_aset_code)
          : "-",
    },
    {
      header: t.ojkData.asetTetap.colBiayaPerolehan,
      type: "money",
      accessorKey: "biaya_perolehan",
    },
    {
      header: t.ojkData.asetTetap.colAkumulasiPenyusutan,
      type: "money",
      accessorKey: "akumulasi_penyusutan_amortisasi",
    },
    {
      header: t.ojkData.asetTetap.colAkumulasiKerugian,
      type: "money",
      accessorKey: "akumulasi_kerugian_penurunan_nilai",
    },
    {
      header: t.ojkData.asetTetap.colNilaiTercatat,
      align: "right",
      cell: (row) => {
        const value = nilaiTercatat(row);
        return value === null ? "-" : formatMoney(value);
      },
    },
    {
      header: t.ojkData.asetTetap.colMetode,
      cell: (row) => labelOf(metodeOptions, row.metode_pengukuran_code),
    },
    {
      header: t.ojkData.asetTetap.colAsOf,
      cell: (row) => formatDateISO(row.as_of) || "-",
    },
    {
      header: t.ojkData.asetTetap.colStatus,
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
        <CardTitle>{t.ojkData.asetTetap.title}</CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <LoadingState label={t.states.loading} />
        ) : forbidden ? (
          <ErrorState
            title={t.ojkData.asetTetap.title}
            description={t.ojkData.asetTetap.forbidden}
          />
        ) : loadError ? (
          <ErrorState
            title={t.ojkData.asetTetap.title}
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
              {t.ojkData.asetTetap.description}
            </p>
            {!canEdit && <Alert variant="info">{t.ojkData.readOnly}</Alert>}
            {feedback && <Alert variant="success">{feedback}</Alert>}
            {fieldError && <Alert variant="error">{fieldError}</Alert>}
            {saveError && <Alert variant="error">{saveError}</Alert>}

            <div className="flex items-center justify-between gap-4">
              <h4 className="text-title font-semibold text-ink-900">
                {t.ojkData.asetTetap.itemsTitle}
              </h4>
              {canEdit && !form && (
                <Button size="sm" onClick={openNew}>
                  {t.ojkData.asetTetap.add}
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
                    ? t.ojkData.asetTetap.editTitle
                    : t.ojkData.asetTetap.newTitle}
                </h5>
                <div className="grid gap-4 md:grid-cols-2">
                  <Select
                    label={t.ojkData.asetTetap.fieldJenis}
                    helperText={t.ojkData.asetTetap.fieldJenisHint}
                    placeholder={t.ojkData.asetTetap.jenisPlaceholder}
                    value={form.jenis_aset_code}
                    options={jenisSelectOptions}
                    onChange={(event) =>
                      updateForm({ jenis_aset_code: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.asetTetap.fieldSumber}
                    helperText={t.ojkData.asetTetap.fieldSumberHint}
                    placeholder={t.ojkData.asetTetap.sumberPlaceholder}
                    value={form.sumber_perolehan_code}
                    options={sumberSelectOptions}
                    onChange={(event) =>
                      updateForm({ sumber_perolehan_code: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.asetTetap.fieldStatusAset}
                    helperText={t.ojkData.asetTetap.fieldStatusAsetHint}
                    placeholder={t.ojkData.asetTetap.statusAsetPlaceholder}
                    value={form.status_aset_code}
                    options={statusAsetSelectOptions}
                    onChange={(event) =>
                      updateForm({ status_aset_code: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.asetTetap.fieldMetode}
                    helperText={t.ojkData.asetTetap.fieldMetodeHint}
                    placeholder={t.ojkData.asetTetap.metodePlaceholder}
                    value={form.metode_pengukuran_code}
                    options={metodeSelectOptions}
                    onChange={(event) =>
                      updateForm({ metode_pengukuran_code: event.target.value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.asetTetap.fieldBiayaPerolehan}
                    helperText={t.ojkData.asetTetap.fieldBiayaPerolehanHint}
                    value={form.biaya_perolehan}
                    onChange={(value) => updateForm({ biaya_perolehan: value })}
                  />
                  <CurrencyInput
                    label={t.ojkData.asetTetap.fieldAkumulasiPenyusutan}
                    helperText={
                      t.ojkData.asetTetap.fieldAkumulasiPenyusutanHint
                    }
                    value={form.akumulasi_penyusutan_amortisasi}
                    onChange={(value) =>
                      updateForm({ akumulasi_penyusutan_amortisasi: value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.asetTetap.fieldAkumulasiKerugian}
                    helperText={t.ojkData.asetTetap.fieldAkumulasiKerugianHint}
                    value={form.akumulasi_kerugian_penurunan_nilai}
                    onChange={(value) =>
                      updateForm({
                        akumulasi_kerugian_penurunan_nilai: value,
                      })
                    }
                  />
                  <DateInput
                    label={t.ojkData.asetTetap.fieldAsOf}
                    helperText={t.ojkData.asetTetap.fieldAsOfHint}
                    value={form.as_of}
                    onChange={(event) =>
                      updateForm({ as_of: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.asetTetap.fieldStatus}
                    value={form.status}
                    options={statusSelectOptions}
                    onChange={(event) =>
                      updateForm({ status: event.target.value })
                    }
                  />
                </div>

                <Textarea
                  label={t.ojkData.asetTetap.fieldNote}
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
              emptyMessage={t.ojkData.asetTetap.empty}
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
