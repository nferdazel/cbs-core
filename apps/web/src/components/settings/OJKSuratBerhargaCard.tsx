"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, request } from "@/lib/api";
import type {
  SuratBerhargaItem,
  SuratBerhargaItemsData,
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

/**
 * Mengurai isian angka yang boleh negatif (kolom VIII diskonto/premium dan X
 * laba/rugi Form 04.00). Koma diperlakukan sebagai pemisah desimal; isian yang
 * bukan angka menjadi 0. Dipakai karena CurrencyInput menolak tanda minus.
 */
function parseSignedNumber(raw: string): number {
  const cleaned = raw.trim().replace(",", ".");
  if (cleaned === "" || cleaned === "-") return 0;
  const parsed = Number(cleaned);
  return Number.isFinite(parsed) ? parsed : 0;
}

interface SuratBerhargaForm {
  id: string;
  klasifikasi_code: string;
  suku_bunga: number;
  tanggal_mulai: string;
  tanggal_jatuh_tempo: string;
  nominal: number;
  nominal_dijaminkan: number;
  biaya_perolehan: number;
  diskonto_premium_belum_diamortisasi: number;
  biaya_transaksi_belum_diamortisasi: number;
  laba_rugi_belum_direalisasi: number;
  biaya_perolehan_diamortisasi: number;
  nomor_surat_berharga: string;
  counterparty_id: string;
  jenis_code: string;
  kualitas_code: string;
  cadangan_kerugian_penurunan_nilai: number;
  lembaga_pemeringkat_code: string;
  peringkat_surat_berharga_code: string;
  tanggal_pemeringkatan: string;
  tanggal_penerbitan: string;
  ckpn_aset_baik: number;
  ckpn_aset_kurang_baik: number;
  ckpn_aset_tidak_baik: number;
  klasifikasi_aset_keuangan_code: string;
  jenis_ckpn_code: string;
  as_of: string;
  status: string;
  note: string;
}

const EMPTY_ITEM: SuratBerhargaForm = {
  id: "",
  klasifikasi_code: "",
  suku_bunga: 0,
  tanggal_mulai: "",
  tanggal_jatuh_tempo: "",
  nominal: 0,
  nominal_dijaminkan: 0,
  biaya_perolehan: 0,
  diskonto_premium_belum_diamortisasi: 0,
  biaya_transaksi_belum_diamortisasi: 0,
  laba_rugi_belum_direalisasi: 0,
  biaya_perolehan_diamortisasi: 0,
  nomor_surat_berharga: "",
  counterparty_id: "",
  jenis_code: "",
  kualitas_code: "",
  cadangan_kerugian_penurunan_nilai: 0,
  lembaga_pemeringkat_code: "",
  peringkat_surat_berharga_code: "",
  tanggal_pemeringkatan: "",
  tanggal_penerbitan: "",
  ckpn_aset_baik: 0,
  ckpn_aset_kurang_baik: 0,
  ckpn_aset_tidak_baik: 0,
  klasifikasi_aset_keuangan_code: "",
  jenis_ckpn_code: "",
  as_of: "",
  status: "AKTIF",
  note: "",
};

/** Sandi baku Form 04.00 (domain/surat_berharga.go), semuanya isian bank. */
const KLASIFIKASI_CODES: string[] = ["1", "2"];
const JENIS_CODES: string[] = ["1", "2", "3"];
const KUALITAS_CODES: string[] = ["1", "3", "5"];
const JENIS_CKPN_CODES: string[] = ["1", "2"];
const KLASIFIKASI_ASET_CODES: string[] = ["1", "2", "3"];
const NOMOR_MAX = 64;
const COUNTERPARTY_MAX = 64;
const SANDI_PEMERINGKAT_MAX = 9;

function itemFormFrom(row: SuratBerhargaItem): SuratBerhargaForm {
  return {
    id: row.id,
    klasifikasi_code: row.klasifikasi_code ?? "",
    suku_bunga: Number(row.suku_bunga) || 0,
    tanggal_mulai: formatDateISO(row.tanggal_mulai),
    tanggal_jatuh_tempo: formatDateISO(row.tanggal_jatuh_tempo),
    nominal: Number(row.nominal) || 0,
    nominal_dijaminkan: Number(row.nominal_dijaminkan) || 0,
    biaya_perolehan: Number(row.biaya_perolehan) || 0,
    diskonto_premium_belum_diamortisasi:
      Number(row.diskonto_premium_belum_diamortisasi) || 0,
    biaya_transaksi_belum_diamortisasi:
      Number(row.biaya_transaksi_belum_diamortisasi) || 0,
    laba_rugi_belum_direalisasi: Number(row.laba_rugi_belum_direalisasi) || 0,
    biaya_perolehan_diamortisasi: Number(row.biaya_perolehan_diamortisasi) || 0,
    nomor_surat_berharga: row.nomor_surat_berharga ?? "",
    counterparty_id: row.counterparty_id ?? "",
    jenis_code: row.jenis_code ?? "",
    kualitas_code: row.kualitas_code ?? "",
    cadangan_kerugian_penurunan_nilai:
      Number(row.cadangan_kerugian_penurunan_nilai) || 0,
    lembaga_pemeringkat_code: row.lembaga_pemeringkat_code ?? "",
    peringkat_surat_berharga_code: row.peringkat_surat_berharga_code ?? "",
    tanggal_pemeringkatan: row.tanggal_pemeringkatan
      ? formatDateISO(row.tanggal_pemeringkatan)
      : "",
    tanggal_penerbitan: formatDateISO(row.tanggal_penerbitan),
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
 * Kartu pengisian register surat berharga Form 04.00: satu baris per surat berharga.
 * Data sebelumnya hanya dapat diisi lewat API/SQL. Penjagaan izin sebenarnya di API
 * (system:config); kartu hanya menyembunyikan kontrol tulis saat peran tidak memegang
 * izin itu. Tidak ada kolom turunan: kolom XI adalah isian bank karena form memberi dua
 * kemungkinan, jadi diminta apa adanya. Kolom I Sandi Kantor tidak ada di payload klien
 * (diambil server dari kantor pelapor). Hapus di sini adalah DELETE fisik: form tidak
 * menetapkan nomor register unik, jadi dialog memakai konfirmasi hapus generik.
 */
export function OJKSuratBerhargaCard() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const canEdit = hasPermission(user, "system:config");

  const [items, setItems] = useState<SuratBerhargaItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<SuratBerhargaItem | null>(
    null,
  );
  const [form, setForm] = useState<SuratBerhargaForm | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    setForbidden(false);
    try {
      const response = await request<SuratBerhargaItemsData>(
        "/reports/ojk/surat-berharga/items",
      );
      setItems(response.data?.items ?? []);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setForbidden(true);
      } else {
        setLoadError(
          err instanceof ApiError
            ? err.message
            : t.ojkData.suratBerharga.loadError,
        );
      }
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    load();
  }, [load]);

  const klasifikasiOptions: Option[] = [
    {
      value: "1",
      label: t.ojkData.suratBerharga.klasifikasiTersediaUntukDijual,
    },
    {
      value: "2",
      label: t.ojkData.suratBerharga.klasifikasiDimilikiHinggaJatuhTempo,
    },
  ];
  const jenisOptions: Option[] = [
    { value: "1", label: t.ojkData.suratBerharga.jenisBankIndonesia },
    { value: "2", label: t.ojkData.suratBerharga.jenisPemerintah },
    { value: "3", label: t.ojkData.suratBerharga.jenisPemerintahDaerah },
  ];
  const kualitasOptions: Option[] = [
    { value: "1", label: t.ojkData.suratBerharga.kualitasLancar },
    { value: "3", label: t.ojkData.suratBerharga.kualitasKurangLancar },
    { value: "5", label: t.ojkData.suratBerharga.kualitasMacet },
  ];
  const jenisCKPNOptions: Option[] = [
    { value: "1", label: t.ojkData.suratBerharga.jenisCKPNIndividual },
    { value: "2", label: t.ojkData.suratBerharga.jenisCKPNKolektif },
  ];
  const klasifikasiAsetOptions: Option[] = [
    {
      value: "1",
      label: t.ojkData.suratBerharga.klasifikasiNilaiWajarLabaRugi,
    },
    { value: "2", label: t.ojkData.suratBerharga.klasifikasiNilaiWajarPKL },
    { value: "3", label: t.ojkData.suratBerharga.klasifikasiBiayaPerolehan },
  ];
  const statusOptions: Option[] = [
    { value: "AKTIF", label: t.ojkData.suratBerharga.statusAktif },
    { value: "NONAKTIF", label: t.ojkData.suratBerharga.statusNonaktif },
  ];

  const updateForm = (patch: Partial<SuratBerhargaForm>) =>
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

  const openEdit = (row: SuratBerhargaItem) => {
    resetMessages();
    setForm(itemFormFrom(row));
  };

  /** Batasan yang sama dengan domain.BuildSuratBerhargaItem, dicek sebelum menembak API. */
  const validate = (value: SuratBerhargaForm): string | null => {
    if (!KLASIFIKASI_CODES.includes(value.klasifikasi_code)) {
      return t.ojkData.suratBerharga.validationKlasifikasi;
    }
    if (value.suku_bunga < 0) {
      return t.ojkData.suratBerharga.validationSukuBunga;
    }
    if (!value.tanggal_mulai) {
      return t.ojkData.suratBerharga.validationTanggalMulai;
    }
    if (!value.tanggal_jatuh_tempo) {
      return t.ojkData.suratBerharga.validationTanggalJatuhTempo;
    }
    if (value.nominal < 0) return t.ojkData.suratBerharga.validationNominal;
    if (value.nominal_dijaminkan < 0) {
      return t.ojkData.suratBerharga.validationNominalDijaminkan;
    }
    if (value.biaya_perolehan < 0) {
      return t.ojkData.suratBerharga.validationBiayaPerolehan;
    }
    if (value.biaya_transaksi_belum_diamortisasi < 0) {
      return t.ojkData.suratBerharga.validationBiayaTransaksi;
    }
    if (value.biaya_perolehan_diamortisasi < 0) {
      return t.ojkData.suratBerharga.validationBiayaPerolehanDiamortisasi;
    }
    if (value.nomor_surat_berharga.length > NOMOR_MAX) {
      return t.ojkData.suratBerharga.validationNomorSuratBerhargaLength;
    }
    if (value.counterparty_id.length > COUNTERPARTY_MAX) {
      return t.ojkData.suratBerharga.validationCounterpartyLength;
    }
    if (!JENIS_CODES.includes(value.jenis_code)) {
      return t.ojkData.suratBerharga.validationJenis;
    }
    if (!KUALITAS_CODES.includes(value.kualitas_code)) {
      return t.ojkData.suratBerharga.validationKualitas;
    }
    if (value.cadangan_kerugian_penurunan_nilai < 0) {
      return t.ojkData.suratBerharga.validationCKPN;
    }
    if (value.ckpn_aset_baik < 0) {
      return t.ojkData.suratBerharga.validationCKPNAsetBaik;
    }
    if (value.ckpn_aset_kurang_baik < 0) {
      return t.ojkData.suratBerharga.validationCKPNAsetKurangBaik;
    }
    if (value.ckpn_aset_tidak_baik < 0) {
      return t.ojkData.suratBerharga.validationCKPNAsetTidakBaik;
    }
    if (value.lembaga_pemeringkat_code.length > SANDI_PEMERINGKAT_MAX) {
      return t.ojkData.suratBerharga.validationLembagaPemeringkatLength;
    }
    if (value.peringkat_surat_berharga_code.length > SANDI_PEMERINGKAT_MAX) {
      return t.ojkData.suratBerharga.validationPeringkatLength;
    }
    if (!value.tanggal_penerbitan) {
      return t.ojkData.suratBerharga.validationTanggalPenerbitan;
    }
    // XXIV boleh kosong; bila terisi harus sandi yang sah.
    if (
      value.klasifikasi_aset_keuangan_code !== "" &&
      !KLASIFIKASI_ASET_CODES.includes(value.klasifikasi_aset_keuangan_code)
    ) {
      return t.ojkData.suratBerharga.validationKlasifikasiAset;
    }
    if (!JENIS_CKPN_CODES.includes(value.jenis_ckpn_code)) {
      return t.ojkData.suratBerharga.validationJenisCKPN;
    }
    if (!value.as_of) return t.ojkData.suratBerharga.validationAsOf;
    if (value.status !== "AKTIF" && value.status !== "NONAKTIF") {
      return t.ojkData.suratBerharga.validationStatus;
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
      await request<SuratBerhargaItem>("/reports/ojk/surat-berharga/items", {
        method: "PUT",
        body: {
          id: form.id,
          klasifikasi_code: form.klasifikasi_code,
          suku_bunga: form.suku_bunga,
          tanggal_mulai: form.tanggal_mulai,
          tanggal_jatuh_tempo: form.tanggal_jatuh_tempo,
          nominal: form.nominal,
          nominal_dijaminkan: form.nominal_dijaminkan,
          biaya_perolehan: form.biaya_perolehan,
          diskonto_premium_belum_diamortisasi:
            form.diskonto_premium_belum_diamortisasi,
          biaya_transaksi_belum_diamortisasi:
            form.biaya_transaksi_belum_diamortisasi,
          laba_rugi_belum_direalisasi: form.laba_rugi_belum_direalisasi,
          biaya_perolehan_diamortisasi: form.biaya_perolehan_diamortisasi,
          nomor_surat_berharga: form.nomor_surat_berharga,
          counterparty_id: form.counterparty_id,
          jenis_code: form.jenis_code,
          kualitas_code: form.kualitas_code,
          cadangan_kerugian_penurunan_nilai:
            form.cadangan_kerugian_penurunan_nilai,
          lembaga_pemeringkat_code: form.lembaga_pemeringkat_code,
          peringkat_surat_berharga_code: form.peringkat_surat_berharga_code,
          tanggal_pemeringkatan: form.tanggal_pemeringkatan,
          tanggal_penerbitan: form.tanggal_penerbitan,
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
      setFeedback(t.ojkData.suratBerharga.saved);
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
      await request(`/reports/ojk/surat-berharga/items/${deleteTarget.id}`, {
        method: "DELETE",
      });
      setFeedback(t.ojkData.suratBerharga.deleted);
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

  const klasifikasiSelectOptions = form
    ? withCurrent(klasifikasiOptions, form.klasifikasi_code)
    : klasifikasiOptions;
  const jenisSelectOptions = form
    ? withCurrent(jenisOptions, form.jenis_code)
    : jenisOptions;
  const kualitasSelectOptions = form
    ? withCurrent(kualitasOptions, form.kualitas_code)
    : kualitasOptions;
  const jenisCKPNSelectOptions = form
    ? withCurrent(jenisCKPNOptions, form.jenis_ckpn_code)
    : jenisCKPNOptions;
  const klasifikasiAsetSelectOptions = form
    ? withCurrent(klasifikasiAsetOptions, form.klasifikasi_aset_keuangan_code)
    : klasifikasiAsetOptions;
  const statusSelectOptions = form
    ? withCurrent(statusOptions, form.status)
    : statusOptions;

  const columns: Column<SuratBerhargaItem>[] = [
    {
      header: t.ojkData.suratBerharga.colKlasifikasi,
      cell: (row) => labelOf(klasifikasiOptions, row.klasifikasi_code),
    },
    {
      header: t.ojkData.suratBerharga.colNomorSuratBerharga,
      accessorKey: "nomor_surat_berharga",
      isMono: true,
    },
    {
      header: t.ojkData.suratBerharga.colJenis,
      cell: (row) => labelOf(jenisOptions, row.jenis_code),
    },
    {
      header: t.ojkData.suratBerharga.colJangkaWaktu,
      cell: (row) =>
        `${formatDateISO(row.tanggal_mulai) || "-"} s.d. ${
          formatDateISO(row.tanggal_jatuh_tempo) || "-"
        }`,
      isMono: true,
    },
    {
      header: t.ojkData.suratBerharga.colSukuBunga,
      align: "right",
      cell: (row) => formatRate(row.suku_bunga),
    },
    {
      header: t.ojkData.suratBerharga.colNominal,
      type: "money",
      accessorKey: "nominal",
    },
    {
      header: t.ojkData.suratBerharga.colNominalDijaminkan,
      type: "money",
      accessorKey: "nominal_dijaminkan",
    },
    {
      header: t.ojkData.suratBerharga.colBiayaPerolehan,
      type: "money",
      accessorKey: "biaya_perolehan",
    },
    {
      header: t.ojkData.suratBerharga.colBiayaPerolehanDiamortisasi,
      type: "money",
      accessorKey: "biaya_perolehan_diamortisasi",
    },
    {
      header: t.ojkData.suratBerharga.colKualitas,
      cell: (row) => labelOf(kualitasOptions, row.kualitas_code),
    },
    {
      header: t.ojkData.suratBerharga.colKlasifikasiAset,
      cell: (row) =>
        labelOf(klasifikasiAsetOptions, row.klasifikasi_aset_keuangan_code) ||
        "-",
    },
    {
      header: t.ojkData.suratBerharga.colCKPN,
      type: "money",
      accessorKey: "cadangan_kerugian_penurunan_nilai",
    },
    {
      header: t.ojkData.suratBerharga.colAsOf,
      cell: (row) => formatDateISO(row.as_of) || "-",
      isMono: true,
    },
    {
      header: t.ojkData.suratBerharga.colStatus,
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
        <CardTitle>{t.ojkData.suratBerharga.title}</CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <LoadingState label={t.states.loading} />
        ) : forbidden ? (
          <ErrorState
            title={t.ojkData.suratBerharga.title}
            description={t.ojkData.suratBerharga.forbidden}
          />
        ) : loadError ? (
          <ErrorState
            title={t.ojkData.suratBerharga.title}
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
              {t.ojkData.suratBerharga.description}
            </p>
            {!canEdit && <Alert variant="info">{t.ojkData.readOnly}</Alert>}
            {feedback && <Alert variant="success">{feedback}</Alert>}
            {fieldError && <Alert variant="error">{fieldError}</Alert>}
            {saveError && <Alert variant="error">{saveError}</Alert>}

            <div className="flex items-center justify-between gap-4">
              <h4 className="text-title font-semibold text-ink-900">
                {t.ojkData.suratBerharga.itemsTitle}
              </h4>
              {canEdit && !form && (
                <Button size="sm" onClick={openNew}>
                  {t.ojkData.suratBerharga.add}
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
                    ? t.ojkData.suratBerharga.editTitle
                    : t.ojkData.suratBerharga.newTitle}
                </h5>
                <div className="grid gap-4 md:grid-cols-2">
                  <Select
                    label={t.ojkData.suratBerharga.fieldKlasifikasi}
                    helperText={t.ojkData.suratBerharga.fieldKlasifikasiHint}
                    placeholder={t.ojkData.suratBerharga.klasifikasiPlaceholder}
                    value={form.klasifikasi_code}
                    options={klasifikasiSelectOptions}
                    onChange={(event) =>
                      updateForm({ klasifikasi_code: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.suratBerharga.fieldNomorSuratBerharga}
                    helperText={
                      t.ojkData.suratBerharga.fieldNomorSuratBerhargaHint
                    }
                    value={form.nomor_surat_berharga}
                    onChange={(event) =>
                      updateForm({ nomor_surat_berharga: event.target.value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.suratBerharga.fieldSukuBunga}
                    helperText={t.ojkData.suratBerharga.fieldSukuBungaHint}
                    currencyPrefix="%"
                    allowDecimals
                    value={form.suku_bunga}
                    onChange={(value) => updateForm({ suku_bunga: value })}
                  />
                  <Input
                    label={t.ojkData.suratBerharga.fieldCounterparty}
                    helperText={t.ojkData.suratBerharga.fieldCounterpartyHint}
                    value={form.counterparty_id}
                    onChange={(event) =>
                      updateForm({ counterparty_id: event.target.value })
                    }
                  />
                  <DateInput
                    label={t.ojkData.suratBerharga.fieldTanggalMulai}
                    helperText={t.ojkData.suratBerharga.fieldTanggalMulaiHint}
                    value={form.tanggal_mulai}
                    onChange={(event) =>
                      updateForm({ tanggal_mulai: event.target.value })
                    }
                  />
                  <DateInput
                    label={t.ojkData.suratBerharga.fieldTanggalJatuhTempo}
                    helperText={
                      t.ojkData.suratBerharga.fieldTanggalJatuhTempoHint
                    }
                    value={form.tanggal_jatuh_tempo}
                    onChange={(event) =>
                      updateForm({ tanggal_jatuh_tempo: event.target.value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.suratBerharga.fieldNominal}
                    helperText={t.ojkData.suratBerharga.fieldNominalHint}
                    value={form.nominal}
                    onChange={(value) => updateForm({ nominal: value })}
                  />
                  <CurrencyInput
                    label={t.ojkData.suratBerharga.fieldNominalDijaminkan}
                    helperText={
                      t.ojkData.suratBerharga.fieldNominalDijaminkanHint
                    }
                    value={form.nominal_dijaminkan}
                    onChange={(value) =>
                      updateForm({ nominal_dijaminkan: value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.suratBerharga.fieldBiayaPerolehan}
                    helperText={t.ojkData.suratBerharga.fieldBiayaPerolehanHint}
                    value={form.biaya_perolehan}
                    onChange={(value) => updateForm({ biaya_perolehan: value })}
                  />
                  <Input
                    label={t.ojkData.suratBerharga.fieldDiskontoPremium}
                    helperText={
                      t.ojkData.suratBerharga.fieldDiskontoPremiumHint
                    }
                    inputMode="decimal"
                    value={String(form.diskonto_premium_belum_diamortisasi)}
                    onChange={(event) =>
                      updateForm({
                        diskonto_premium_belum_diamortisasi: parseSignedNumber(
                          event.target.value,
                        ),
                      })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.suratBerharga.fieldBiayaTransaksi}
                    helperText={t.ojkData.suratBerharga.fieldBiayaTransaksiHint}
                    value={form.biaya_transaksi_belum_diamortisasi}
                    onChange={(value) =>
                      updateForm({
                        biaya_transaksi_belum_diamortisasi: value,
                      })
                    }
                  />
                  <Input
                    label={t.ojkData.suratBerharga.fieldLabaRugi}
                    helperText={t.ojkData.suratBerharga.fieldLabaRugiHint}
                    inputMode="decimal"
                    value={String(form.laba_rugi_belum_direalisasi)}
                    onChange={(event) =>
                      updateForm({
                        laba_rugi_belum_direalisasi: parseSignedNumber(
                          event.target.value,
                        ),
                      })
                    }
                  />
                  <CurrencyInput
                    label={
                      t.ojkData.suratBerharga.fieldBiayaPerolehanDiamortisasi
                    }
                    helperText={
                      t.ojkData.suratBerharga
                        .fieldBiayaPerolehanDiamortisasiHint
                    }
                    value={form.biaya_perolehan_diamortisasi}
                    onChange={(value) =>
                      updateForm({ biaya_perolehan_diamortisasi: value })
                    }
                  />
                  <Select
                    label={t.ojkData.suratBerharga.fieldJenis}
                    helperText={t.ojkData.suratBerharga.fieldJenisHint}
                    placeholder={t.ojkData.suratBerharga.jenisPlaceholder}
                    value={form.jenis_code}
                    options={jenisSelectOptions}
                    onChange={(event) =>
                      updateForm({ jenis_code: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.suratBerharga.fieldKualitas}
                    helperText={t.ojkData.suratBerharga.fieldKualitasHint}
                    placeholder={t.ojkData.suratBerharga.kualitasPlaceholder}
                    value={form.kualitas_code}
                    options={kualitasSelectOptions}
                    onChange={(event) =>
                      updateForm({ kualitas_code: event.target.value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.suratBerharga.fieldCKPN}
                    helperText={t.ojkData.suratBerharga.fieldCKPNHint}
                    value={form.cadangan_kerugian_penurunan_nilai}
                    onChange={(value) =>
                      updateForm({ cadangan_kerugian_penurunan_nilai: value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.suratBerharga.fieldCKPNAsetBaik}
                    helperText={t.ojkData.suratBerharga.fieldCKPNAsetBaikHint}
                    value={form.ckpn_aset_baik}
                    onChange={(value) => updateForm({ ckpn_aset_baik: value })}
                  />
                  <CurrencyInput
                    label={t.ojkData.suratBerharga.fieldCKPNAsetKurangBaik}
                    helperText={
                      t.ojkData.suratBerharga.fieldCKPNAsetKurangBaikHint
                    }
                    value={form.ckpn_aset_kurang_baik}
                    onChange={(value) =>
                      updateForm({ ckpn_aset_kurang_baik: value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.suratBerharga.fieldCKPNAsetTidakBaik}
                    helperText={
                      t.ojkData.suratBerharga.fieldCKPNAsetTidakBaikHint
                    }
                    value={form.ckpn_aset_tidak_baik}
                    onChange={(value) =>
                      updateForm({ ckpn_aset_tidak_baik: value })
                    }
                  />
                  <Input
                    label={t.ojkData.suratBerharga.fieldLembagaPemeringkat}
                    helperText={
                      t.ojkData.suratBerharga.fieldLembagaPemeringkatHint
                    }
                    value={form.lembaga_pemeringkat_code}
                    onChange={(event) =>
                      updateForm({
                        lembaga_pemeringkat_code: event.target.value,
                      })
                    }
                  />
                  <Input
                    label={t.ojkData.suratBerharga.fieldPeringkat}
                    helperText={t.ojkData.suratBerharga.fieldPeringkatHint}
                    value={form.peringkat_surat_berharga_code}
                    onChange={(event) =>
                      updateForm({
                        peringkat_surat_berharga_code: event.target.value,
                      })
                    }
                  />
                  <DateInput
                    label={t.ojkData.suratBerharga.fieldTanggalPemeringkatan}
                    helperText={
                      t.ojkData.suratBerharga.fieldTanggalPemeringkatanHint
                    }
                    value={form.tanggal_pemeringkatan}
                    onChange={(event) =>
                      updateForm({ tanggal_pemeringkatan: event.target.value })
                    }
                  />
                  <DateInput
                    label={t.ojkData.suratBerharga.fieldTanggalPenerbitan}
                    helperText={
                      t.ojkData.suratBerharga.fieldTanggalPenerbitanHint
                    }
                    value={form.tanggal_penerbitan}
                    onChange={(event) =>
                      updateForm({ tanggal_penerbitan: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.suratBerharga.fieldKlasifikasiAset}
                    helperText={
                      t.ojkData.suratBerharga.fieldKlasifikasiAsetHint
                    }
                    placeholder={
                      t.ojkData.suratBerharga.klasifikasiAsetPlaceholder
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
                    label={t.ojkData.suratBerharga.fieldJenisCKPN}
                    helperText={t.ojkData.suratBerharga.fieldJenisCKPNHint}
                    placeholder={t.ojkData.suratBerharga.jenisCKPNPlaceholder}
                    value={form.jenis_ckpn_code}
                    options={jenisCKPNSelectOptions}
                    onChange={(event) =>
                      updateForm({ jenis_ckpn_code: event.target.value })
                    }
                  />
                  <DateInput
                    label={t.ojkData.suratBerharga.fieldAsOf}
                    helperText={t.ojkData.suratBerharga.fieldAsOfHint}
                    value={form.as_of}
                    onChange={(event) =>
                      updateForm({ as_of: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.suratBerharga.fieldStatus}
                    value={form.status}
                    options={statusSelectOptions}
                    onChange={(event) =>
                      updateForm({ status: event.target.value })
                    }
                  />
                </div>

                <Textarea
                  label={t.ojkData.suratBerharga.fieldNote}
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
              emptyMessage={t.ojkData.suratBerharga.empty}
              zebra
            />
          </>
        )}
      </CardContent>

      <ConfirmDialog
        open={deleteTarget !== null}
        title={t.ojkData.suratBerharga.deleteConfirmTitle}
        description={t.ojkData.suratBerharga.deleteConfirmBody}
        confirmLabel={deleting ? t.ojkData.deleting : t.ojkData.delete}
        destructive
        loading={deleting}
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </Card>
  );
}
