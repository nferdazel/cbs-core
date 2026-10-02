"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, request } from "@/lib/api";
import type {
  AsetKeuanganItem,
  AsetKeuanganItemsData,
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

interface AsetKeuanganForm {
  id: string;
  no_rekening: string;
  counterparty_id: string;
  jenis_code: string;
  tanggal_mulai: string;
  tanggal_jatuh_tempo: string;
  suku_bunga: number;
  nominal: number;
  nilai_agunan_diperhitungkan: number;
  cadangan_kerugian_penurunan_nilai: number;
  ckpn_aset_baik: number;
  ckpn_aset_kurang_baik: number;
  ckpn_aset_tidak_baik: number;
  klasifikasi_aset_keuangan_code: string;
  jenis_ckpn_code: string;
  as_of: string;
  status: string;
  note: string;
}

const EMPTY_ITEM: AsetKeuanganForm = {
  id: "",
  no_rekening: "",
  counterparty_id: "",
  jenis_code: "",
  tanggal_mulai: "",
  tanggal_jatuh_tempo: "",
  suku_bunga: 0,
  nominal: 0,
  nilai_agunan_diperhitungkan: 0,
  cadangan_kerugian_penurunan_nilai: 0,
  ckpn_aset_baik: 0,
  ckpn_aset_kurang_baik: 0,
  ckpn_aset_tidak_baik: 0,
  klasifikasi_aset_keuangan_code: "",
  jenis_ckpn_code: "",
  as_of: "",
  status: "AKTIF",
  note: "",
};

/** Sandi baku Form 18.00 (domain/aset_keuangan_lainnya.go), semuanya isian bank. */
const JENIS_CODES: string[] = ["10", "99"];
const KLASIFIKASI_ASET_CODES: string[] = ["1", "2", "3"];
const JENIS_CKPN_CODES: string[] = ["1", "2"];
const NO_REKENING_MAX = 64;
const COUNTERPARTY_MAX = 64;

function itemFormFrom(row: AsetKeuanganItem): AsetKeuanganForm {
  return {
    id: row.id,
    no_rekening: row.no_rekening ?? "",
    counterparty_id: row.counterparty_id ?? "",
    jenis_code: row.jenis_code ?? "",
    tanggal_mulai: formatDateISO(row.tanggal_mulai),
    tanggal_jatuh_tempo: formatDateISO(row.tanggal_jatuh_tempo),
    suku_bunga: Number(row.suku_bunga) || 0,
    nominal: Number(row.nominal) || 0,
    nilai_agunan_diperhitungkan: Number(row.nilai_agunan_diperhitungkan) || 0,
    cadangan_kerugian_penurunan_nilai:
      Number(row.cadangan_kerugian_penurunan_nilai) || 0,
    ckpn_aset_baik: Number(row.ckpn_aset_baik) || 0,
    ckpn_aset_kurang_baik: Number(row.ckpn_aset_kurang_baik) || 0,
    ckpn_aset_tidak_baik: Number(row.ckpn_aset_tidak_baik) || 0,
    klasifikasi_aset_keuangan_code: row.klasifikasi_aset_keuangan_code ?? "",
    jenis_ckpn_code: row.jenis_ckpn_code ?? "",
    as_of: formatDateISO(row.as_of),
    status: row.status ?? "",
    note: row.note ?? "",
  };
}

/**
 * 422 nomor rekening terpakai (katalog API aset_keuangan_no_rekening_used). API
 * mengirim pesan terterjemah, bukan kode mesin, jadi kecocokan lewat penanda pesan
 * katalog ID/EN. Pesan 422 lain tetap ditampilkan apa adanya.
 */
function isNoRekeningUsed(err: ApiError): boolean {
  const message = err.message.toLowerCase();
  return (
    message.includes("tidak boleh dipakai ulang") ||
    message.includes("cannot be reused")
  );
}

/**
 * Kartu pengisian register aset keuangan lainnya Form 18.00: satu baris per rekening
 * unik. Data sebelumnya hanya dapat diisi lewat API/SQL. Penjagaan izin sebenarnya di
 * API (system:config); kartu hanya menyembunyikan kontrol tulis saat peran tidak
 * memegang izin itu. Tidak ada kolom turunan: seluruh nilai adalah isian bank, jadi
 * semuanya diminta apa adanya. Kolom I Sandi Kantor tidak ada di payload klien
 * (diambil server dari kantor pelapor). Hapus adalah soft-delete: baris menjadi
 * NONAKTIF dan nomor rekening tetap terkunci ("tidak boleh sama" menurut form).
 */
export function OJKAsetKeuanganLainnyaCard() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const canEdit = hasPermission(user, "system:config");

  const [items, setItems] = useState<AsetKeuanganItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<AsetKeuanganItem | null>(
    null,
  );
  const [form, setForm] = useState<AsetKeuanganForm | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    setForbidden(false);
    try {
      const response = await request<AsetKeuanganItemsData>(
        "/reports/ojk/aset-keuangan/items",
      );
      setItems(response.data?.items ?? []);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setForbidden(true);
      } else {
        setLoadError(
          err instanceof ApiError
            ? err.message
            : t.ojkData.asetKeuangan.loadError,
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
    { value: "10", label: t.ojkData.asetKeuangan.jenisTagihanFraud },
    { value: "99", label: t.ojkData.asetKeuangan.jenisTagihanLainnya },
  ];
  const klasifikasiAsetOptions: Option[] = [
    { value: "1", label: t.ojkData.asetKeuangan.klasifikasiNilaiWajarLabaRugi },
    { value: "2", label: t.ojkData.asetKeuangan.klasifikasiNilaiWajarPKL },
    { value: "3", label: t.ojkData.asetKeuangan.klasifikasiBiayaPerolehan },
  ];
  const jenisCKPNOptions: Option[] = [
    { value: "1", label: t.ojkData.asetKeuangan.jenisCKPNIndividual },
    { value: "2", label: t.ojkData.asetKeuangan.jenisCKPNKolektif },
  ];
  const statusOptions: Option[] = [
    { value: "AKTIF", label: t.ojkData.asetKeuangan.statusAktif },
    { value: "NONAKTIF", label: t.ojkData.asetKeuangan.statusNonaktif },
  ];

  const updateForm = (patch: Partial<AsetKeuanganForm>) =>
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

  const openEdit = (row: AsetKeuanganItem) => {
    resetMessages();
    setForm(itemFormFrom(row));
  };

  /** Batasan yang sama dengan domain.BuildAsetKeuanganItem, dicek sebelum menembak API. */
  const validate = (value: AsetKeuanganForm): string | null => {
    const noRekening = value.no_rekening.trim();
    if (!noRekening) return t.ojkData.asetKeuangan.validationNoRekening;
    if (noRekening.length > NO_REKENING_MAX) {
      return t.ojkData.asetKeuangan.validationNoRekeningLength;
    }
    const counterparty = value.counterparty_id.trim();
    if (!counterparty) return t.ojkData.asetKeuangan.validationCounterparty;
    if (counterparty.length > COUNTERPARTY_MAX) {
      return t.ojkData.asetKeuangan.validationCounterpartyLength;
    }
    if (!JENIS_CODES.includes(value.jenis_code)) {
      return t.ojkData.asetKeuangan.validationJenis;
    }
    if (!value.tanggal_mulai) {
      return t.ojkData.asetKeuangan.validationTanggalMulai;
    }
    if (!value.tanggal_jatuh_tempo) {
      return t.ojkData.asetKeuangan.validationTanggalJatuhTempo;
    }
    if (value.suku_bunga < 0) {
      return t.ojkData.asetKeuangan.validationSukuBunga;
    }
    if (value.nominal < 0) return t.ojkData.asetKeuangan.validationNominal;
    if (value.nilai_agunan_diperhitungkan < 0) {
      return t.ojkData.asetKeuangan.validationNilaiAgunan;
    }
    if (value.cadangan_kerugian_penurunan_nilai < 0) {
      return t.ojkData.asetKeuangan.validationCKPN;
    }
    if (value.ckpn_aset_baik < 0) {
      return t.ojkData.asetKeuangan.validationCKPNAsetBaik;
    }
    if (value.ckpn_aset_kurang_baik < 0) {
      return t.ojkData.asetKeuangan.validationCKPNAsetKurangBaik;
    }
    if (value.ckpn_aset_tidak_baik < 0) {
      return t.ojkData.asetKeuangan.validationCKPNAsetTidakBaik;
    }
    if (
      !KLASIFIKASI_ASET_CODES.includes(value.klasifikasi_aset_keuangan_code)
    ) {
      return t.ojkData.asetKeuangan.validationKlasifikasiAset;
    }
    if (!JENIS_CKPN_CODES.includes(value.jenis_ckpn_code)) {
      return t.ojkData.asetKeuangan.validationJenisCKPN;
    }
    if (!value.as_of) return t.ojkData.asetKeuangan.validationAsOf;
    if (value.status !== "AKTIF" && value.status !== "NONAKTIF") {
      return t.ojkData.asetKeuangan.validationStatus;
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
      await request<AsetKeuanganItem>("/reports/ojk/aset-keuangan/items", {
        method: "PUT",
        body: {
          id: form.id,
          no_rekening: form.no_rekening,
          counterparty_id: form.counterparty_id,
          jenis_code: form.jenis_code,
          tanggal_mulai: form.tanggal_mulai,
          tanggal_jatuh_tempo: form.tanggal_jatuh_tempo,
          suku_bunga: form.suku_bunga,
          nominal: form.nominal,
          nilai_agunan_diperhitungkan: form.nilai_agunan_diperhitungkan,
          cadangan_kerugian_penurunan_nilai:
            form.cadangan_kerugian_penurunan_nilai,
          ckpn_aset_baik: form.ckpn_aset_baik,
          ckpn_aset_kurang_baik: form.ckpn_aset_kurang_baik,
          ckpn_aset_tidak_baik: form.ckpn_aset_tidak_baik,
          klasifikasi_aset_keuangan_code: form.klasifikasi_aset_keuangan_code,
          jenis_ckpn_code: form.jenis_ckpn_code,
          as_of: form.as_of,
          status: form.status,
          note: form.note,
        },
      });
      setFeedback(t.ojkData.asetKeuangan.saved);
      setForm(null);
      await load();
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setSaveError(t.ojkData.saveForbidden);
      } else if (err instanceof ApiError && err.status === 422) {
        setFieldError(
          isNoRekeningUsed(err)
            ? t.ojkData.asetKeuangan.noRekeningUsed
            : err.message,
        );
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
      await request(`/reports/ojk/aset-keuangan/items/${deleteTarget.id}`, {
        method: "DELETE",
      });
      setFeedback(t.ojkData.asetKeuangan.deleted);
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
    ? withCurrent(jenisOptions, form.jenis_code)
    : jenisOptions;
  const klasifikasiAsetSelectOptions = form
    ? withCurrent(klasifikasiAsetOptions, form.klasifikasi_aset_keuangan_code)
    : klasifikasiAsetOptions;
  const jenisCKPNSelectOptions = form
    ? withCurrent(jenisCKPNOptions, form.jenis_ckpn_code)
    : jenisCKPNOptions;
  const statusSelectOptions = form
    ? withCurrent(statusOptions, form.status)
    : statusOptions;

  const columns: Column<AsetKeuanganItem>[] = [
    {
      header: t.ojkData.asetKeuangan.colNoRekening,
      accessorKey: "no_rekening",
      isMono: true,
    },
    {
      header: t.ojkData.asetKeuangan.colCounterparty,
      accessorKey: "counterparty_id",
      isMono: true,
    },
    {
      header: t.ojkData.asetKeuangan.colJenis,
      cell: (row) => labelOf(jenisOptions, row.jenis_code),
    },
    {
      header: t.ojkData.asetKeuangan.colTanggalMulai,
      cell: (row) => formatDateISO(row.tanggal_mulai) || "-",
      isMono: true,
    },
    {
      header: t.ojkData.asetKeuangan.colTanggalJatuhTempo,
      cell: (row) => formatDateISO(row.tanggal_jatuh_tempo) || "-",
      isMono: true,
    },
    {
      header: t.ojkData.asetKeuangan.colSukuBunga,
      align: "right",
      cell: (row) => formatRate(row.suku_bunga),
    },
    {
      header: t.ojkData.asetKeuangan.colNominal,
      type: "money",
      accessorKey: "nominal",
    },
    {
      header: t.ojkData.asetKeuangan.colNilaiAgunan,
      type: "money",
      accessorKey: "nilai_agunan_diperhitungkan",
    },
    {
      header: t.ojkData.asetKeuangan.colCKPN,
      type: "money",
      accessorKey: "cadangan_kerugian_penurunan_nilai",
    },
    {
      header: t.ojkData.asetKeuangan.colCKPNAsetBaik,
      type: "money",
      accessorKey: "ckpn_aset_baik",
    },
    {
      header: t.ojkData.asetKeuangan.colCKPNAsetKurangBaik,
      type: "money",
      accessorKey: "ckpn_aset_kurang_baik",
    },
    {
      header: t.ojkData.asetKeuangan.colCKPNAsetTidakBaik,
      type: "money",
      accessorKey: "ckpn_aset_tidak_baik",
    },
    {
      header: t.ojkData.asetKeuangan.colKlasifikasiAset,
      cell: (row) =>
        labelOf(klasifikasiAsetOptions, row.klasifikasi_aset_keuangan_code),
    },
    {
      header: t.ojkData.asetKeuangan.colJenisCKPN,
      cell: (row) => labelOf(jenisCKPNOptions, row.jenis_ckpn_code),
    },
    {
      header: t.ojkData.asetKeuangan.colAsOf,
      cell: (row) => formatDateISO(row.as_of) || "-",
      isMono: true,
    },
    {
      header: t.ojkData.asetKeuangan.colStatus,
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
        <CardTitle>{t.ojkData.asetKeuangan.title}</CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <LoadingState label={t.states.loading} />
        ) : forbidden ? (
          <ErrorState
            title={t.ojkData.asetKeuangan.title}
            description={t.ojkData.asetKeuangan.forbidden}
          />
        ) : loadError ? (
          <ErrorState
            title={t.ojkData.asetKeuangan.title}
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
              {t.ojkData.asetKeuangan.description}
            </p>
            {!canEdit && <Alert variant="info">{t.ojkData.readOnly}</Alert>}
            {feedback && <Alert variant="success">{feedback}</Alert>}
            {fieldError && <Alert variant="error">{fieldError}</Alert>}
            {saveError && <Alert variant="error">{saveError}</Alert>}

            <div className="flex items-center justify-between gap-4">
              <h4 className="text-title font-semibold text-ink-900">
                {t.ojkData.asetKeuangan.itemsTitle}
              </h4>
              {canEdit && !form && (
                <Button size="sm" onClick={openNew}>
                  {t.ojkData.asetKeuangan.add}
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
                    ? t.ojkData.asetKeuangan.editTitle
                    : t.ojkData.asetKeuangan.newTitle}
                </h5>
                <div className="grid gap-4 md:grid-cols-2">
                  <Input
                    label={t.ojkData.asetKeuangan.fieldNoRekening}
                    helperText={t.ojkData.asetKeuangan.fieldNoRekeningHint}
                    value={form.no_rekening}
                    onChange={(event) =>
                      updateForm({ no_rekening: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.asetKeuangan.fieldCounterparty}
                    helperText={t.ojkData.asetKeuangan.fieldCounterpartyHint}
                    value={form.counterparty_id}
                    onChange={(event) =>
                      updateForm({ counterparty_id: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.asetKeuangan.fieldJenis}
                    helperText={t.ojkData.asetKeuangan.fieldJenisHint}
                    placeholder={t.ojkData.asetKeuangan.jenisPlaceholder}
                    value={form.jenis_code}
                    options={jenisSelectOptions}
                    onChange={(event) =>
                      updateForm({ jenis_code: event.target.value })
                    }
                  />
                  <DateInput
                    label={t.ojkData.asetKeuangan.fieldTanggalMulai}
                    helperText={t.ojkData.asetKeuangan.fieldTanggalMulaiHint}
                    value={form.tanggal_mulai}
                    onChange={(event) =>
                      updateForm({ tanggal_mulai: event.target.value })
                    }
                  />
                  <DateInput
                    label={t.ojkData.asetKeuangan.fieldTanggalJatuhTempo}
                    helperText={
                      t.ojkData.asetKeuangan.fieldTanggalJatuhTempoHint
                    }
                    value={form.tanggal_jatuh_tempo}
                    onChange={(event) =>
                      updateForm({ tanggal_jatuh_tempo: event.target.value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.asetKeuangan.fieldSukuBunga}
                    helperText={t.ojkData.asetKeuangan.fieldSukuBungaHint}
                    currencyPrefix="%"
                    allowDecimals
                    value={form.suku_bunga}
                    onChange={(value) => updateForm({ suku_bunga: value })}
                  />
                  <CurrencyInput
                    label={t.ojkData.asetKeuangan.fieldNominal}
                    helperText={t.ojkData.asetKeuangan.fieldNominalHint}
                    value={form.nominal}
                    onChange={(value) => updateForm({ nominal: value })}
                  />
                  <CurrencyInput
                    label={t.ojkData.asetKeuangan.fieldNilaiAgunan}
                    helperText={t.ojkData.asetKeuangan.fieldNilaiAgunanHint}
                    value={form.nilai_agunan_diperhitungkan}
                    onChange={(value) =>
                      updateForm({ nilai_agunan_diperhitungkan: value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.asetKeuangan.fieldCKPN}
                    helperText={t.ojkData.asetKeuangan.fieldCKPNHint}
                    value={form.cadangan_kerugian_penurunan_nilai}
                    onChange={(value) =>
                      updateForm({ cadangan_kerugian_penurunan_nilai: value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.asetKeuangan.fieldCKPNAsetBaik}
                    helperText={t.ojkData.asetKeuangan.fieldCKPNAsetBaikHint}
                    value={form.ckpn_aset_baik}
                    onChange={(value) => updateForm({ ckpn_aset_baik: value })}
                  />
                  <CurrencyInput
                    label={t.ojkData.asetKeuangan.fieldCKPNAsetKurangBaik}
                    helperText={
                      t.ojkData.asetKeuangan.fieldCKPNAsetKurangBaikHint
                    }
                    value={form.ckpn_aset_kurang_baik}
                    onChange={(value) =>
                      updateForm({ ckpn_aset_kurang_baik: value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.asetKeuangan.fieldCKPNAsetTidakBaik}
                    helperText={
                      t.ojkData.asetKeuangan.fieldCKPNAsetTidakBaikHint
                    }
                    value={form.ckpn_aset_tidak_baik}
                    onChange={(value) =>
                      updateForm({ ckpn_aset_tidak_baik: value })
                    }
                  />
                  <Select
                    label={t.ojkData.asetKeuangan.fieldKlasifikasiAset}
                    helperText={t.ojkData.asetKeuangan.fieldKlasifikasiAsetHint}
                    placeholder={
                      t.ojkData.asetKeuangan.klasifikasiAsetPlaceholder
                    }
                    value={form.klasifikasi_aset_keuangan_code}
                    options={klasifikasiAsetSelectOptions}
                    onChange={(event) =>
                      updateForm({
                        klasifikasi_aset_keuangan_code: event.target.value,
                      })
                    }
                  />
                  <Select
                    label={t.ojkData.asetKeuangan.fieldJenisCKPN}
                    helperText={t.ojkData.asetKeuangan.fieldJenisCKPNHint}
                    placeholder={t.ojkData.asetKeuangan.jenisCKPNPlaceholder}
                    value={form.jenis_ckpn_code}
                    options={jenisCKPNSelectOptions}
                    onChange={(event) =>
                      updateForm({ jenis_ckpn_code: event.target.value })
                    }
                  />
                  <DateInput
                    label={t.ojkData.asetKeuangan.fieldAsOf}
                    helperText={t.ojkData.asetKeuangan.fieldAsOfHint}
                    value={form.as_of}
                    onChange={(event) =>
                      updateForm({ as_of: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.asetKeuangan.fieldStatus}
                    value={form.status}
                    options={statusSelectOptions}
                    onChange={(event) =>
                      updateForm({ status: event.target.value })
                    }
                  />
                </div>

                <Textarea
                  label={t.ojkData.asetKeuangan.fieldNote}
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
              emptyMessage={t.ojkData.asetKeuangan.empty}
              zebra
            />
          </>
        )}
      </CardContent>

      <ConfirmDialog
        open={deleteTarget !== null}
        title={t.ojkData.asetKeuangan.deleteConfirmTitle}
        description={t.ojkData.asetKeuangan.deleteConfirmBody}
        confirmLabel={
          deleting
            ? t.ojkData.asetKeuangan.deactivating
            : t.ojkData.asetKeuangan.deactivate
        }
        destructive
        loading={deleting}
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </Card>
  );
}
