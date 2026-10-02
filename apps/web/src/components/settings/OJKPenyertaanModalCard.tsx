"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, request } from "@/lib/api";
import type {
  PenyertaanItem,
  PenyertaanItemsData,
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

interface PenyertaanForm {
  id: string;
  no_register: string;
  counterparty_id: string;
  metode_penyertaan_code: string;
  kualitas_code: string;
  tujuan_penyertaan_code: string;
  tanggal_mulai: string;
  persentase_penyertaan: number;
  nominal: number;
  jumlah_bulan_laporan: number;
  cadangan_kerugian_penurunan_nilai: number;
  ckpn_aset_baik: number;
  ckpn_aset_kurang_baik: number;
  ckpn_aset_tidak_baik: number;
  jenis_ckpn_code: string;
  as_of: string;
  status: string;
  note: string;
}

const EMPTY_ITEM: PenyertaanForm = {
  id: "",
  no_register: "",
  counterparty_id: "",
  metode_penyertaan_code: "",
  kualitas_code: "",
  tujuan_penyertaan_code: "",
  tanggal_mulai: "",
  persentase_penyertaan: 0,
  nominal: 0,
  jumlah_bulan_laporan: 0,
  cadangan_kerugian_penurunan_nilai: 0,
  ckpn_aset_baik: 0,
  ckpn_aset_kurang_baik: 0,
  ckpn_aset_tidak_baik: 0,
  jenis_ckpn_code: "",
  as_of: "",
  status: "AKTIF",
  note: "",
};

/** Sandi baku Form 16.00 (domain/penyertaan.go), semuanya isian bank. */
const METODE_CODES: string[] = ["1", "2"];
const KUALITAS_CODES: string[] = ["1", "3", "4", "5"];
const TUJUAN_CODES: string[] = ["1", "9"];
const JENIS_CKPN_CODES: string[] = ["1", "2"];
const NO_REGISTER_MAX = 64;
const COUNTERPARTY_MAX = 64;

function itemFormFrom(row: PenyertaanItem): PenyertaanForm {
  return {
    id: row.id,
    no_register: row.no_register ?? "",
    counterparty_id: row.counterparty_id ?? "",
    metode_penyertaan_code: row.metode_penyertaan_code ?? "",
    kualitas_code: row.kualitas_code ?? "",
    tujuan_penyertaan_code: row.tujuan_penyertaan_code ?? "",
    tanggal_mulai: formatDateISO(row.tanggal_mulai),
    persentase_penyertaan: Number(row.persentase_penyertaan) || 0,
    nominal: Number(row.nominal) || 0,
    jumlah_bulan_laporan: Number(row.jumlah_bulan_laporan) || 0,
    cadangan_kerugian_penurunan_nilai:
      Number(row.cadangan_kerugian_penurunan_nilai) || 0,
    ckpn_aset_baik: Number(row.ckpn_aset_baik) || 0,
    ckpn_aset_kurang_baik: Number(row.ckpn_aset_kurang_baik) || 0,
    ckpn_aset_tidak_baik: Number(row.ckpn_aset_tidak_baik) || 0,
    jenis_ckpn_code: row.jenis_ckpn_code ?? "",
    as_of: formatDateISO(row.as_of),
    status: row.status ?? "",
    note: row.note ?? "",
  };
}

/**
 * 422 nomor register terpakai (katalog API penyertaan_no_register_used). API
 * mengirim pesan terterjemah, bukan kode mesin, jadi kecocokan lewat penanda pesan
 * katalog ID/EN. Pesan 422 lain tetap ditampilkan apa adanya.
 */
function isNoRegisterUsed(err: ApiError): boolean {
  const message = err.message.toLowerCase();
  return (
    message.includes("tidak boleh dipakai ulang") ||
    message.includes("cannot be reused")
  );
}

/**
 * Kartu pengisian register penyertaan modal Form 16.00: satu baris per penyertaan.
 * Data sebelumnya hanya dapat diisi lewat API/SQL. Penjagaan izin sebenarnya di API
 * (system:config); kartu hanya menyembunyikan kontrol tulis saat peran tidak memegang
 * izin itu. Tidak ada kolom turunan: kolom X dan blok CKPN adalah isian bank, bukan
 * hasil hitungan, jadi semuanya diminta apa adanya. Kolom I Sandi Kantor tidak ada di
 * payload klien (diambil server dari kantor pelapor), jadi tidak ditampilkan. Hapus
 * adalah soft-delete: baris menjadi NONAKTIF dan nomor register tetap terkunci, sesuai
 * aturan no reuse/no recycle.
 */
export function OJKPenyertaanModalCard() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const canEdit = hasPermission(user, "system:config");

  const [items, setItems] = useState<PenyertaanItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<PenyertaanItem | null>(null);
  const [form, setForm] = useState<PenyertaanForm | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    setForbidden(false);
    try {
      const response = await request<PenyertaanItemsData>(
        "/reports/ojk/penyertaan/items",
      );
      setItems(response.data?.items ?? []);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setForbidden(true);
      } else {
        setLoadError(
          err instanceof ApiError
            ? err.message
            : t.ojkData.penyertaan.loadError,
        );
      }
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    load();
  }, [load]);

  const metodeOptions: Option[] = [
    { value: "1", label: t.ojkData.penyertaan.metodeBiayaPerolehan },
    { value: "2", label: t.ojkData.penyertaan.metodeEkuitas },
  ];
  const kualitasOptions: Option[] = [
    { value: "1", label: t.ojkData.penyertaan.kualitasLancar },
    { value: "3", label: t.ojkData.penyertaan.kualitasKurangLancar },
    { value: "4", label: t.ojkData.penyertaan.kualitasDiragukan },
    { value: "5", label: t.ojkData.penyertaan.kualitasMacet },
  ];
  const tujuanOptions: Option[] = [
    { value: "1", label: t.ojkData.penyertaan.tujuanLembagaPenunjang },
    { value: "9", label: t.ojkData.penyertaan.tujuanLainnya },
  ];
  const jenisCKPNOptions: Option[] = [
    { value: "1", label: t.ojkData.penyertaan.jenisCKPNIndividual },
    { value: "2", label: t.ojkData.penyertaan.jenisCKPNKolektif },
  ];
  const statusOptions: Option[] = [
    { value: "AKTIF", label: t.ojkData.penyertaan.statusAktif },
    { value: "NONAKTIF", label: t.ojkData.penyertaan.statusNonaktif },
  ];

  const updateForm = (patch: Partial<PenyertaanForm>) =>
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

  const openEdit = (row: PenyertaanItem) => {
    resetMessages();
    setForm(itemFormFrom(row));
  };

  /** Batasan yang sama dengan domain.BuildPenyertaanItem, dicek sebelum menembak API. */
  const validate = (value: PenyertaanForm): string | null => {
    const noRegister = value.no_register.trim();
    if (!noRegister) return t.ojkData.penyertaan.validationNoRegister;
    if (noRegister.length > NO_REGISTER_MAX) {
      return t.ojkData.penyertaan.validationNoRegisterLength;
    }
    const counterparty = value.counterparty_id.trim();
    if (!counterparty) return t.ojkData.penyertaan.validationCounterparty;
    if (counterparty.length > COUNTERPARTY_MAX) {
      return t.ojkData.penyertaan.validationCounterpartyLength;
    }
    if (!METODE_CODES.includes(value.metode_penyertaan_code)) {
      return t.ojkData.penyertaan.validationMetode;
    }
    if (!KUALITAS_CODES.includes(value.kualitas_code)) {
      return t.ojkData.penyertaan.validationKualitas;
    }
    if (!TUJUAN_CODES.includes(value.tujuan_penyertaan_code)) {
      return t.ojkData.penyertaan.validationTujuan;
    }
    if (!value.tanggal_mulai) {
      return t.ojkData.penyertaan.validationTanggalMulai;
    }
    if (value.persentase_penyertaan < 0 || value.persentase_penyertaan > 100) {
      return t.ojkData.penyertaan.validationPersentase;
    }
    if (value.nominal < 0) return t.ojkData.penyertaan.validationNominal;
    if (value.jumlah_bulan_laporan < 0) {
      return t.ojkData.penyertaan.validationJumlahBulanLaporan;
    }
    if (!JENIS_CKPN_CODES.includes(value.jenis_ckpn_code)) {
      return t.ojkData.penyertaan.validationJenisCKPN;
    }
    if (value.cadangan_kerugian_penurunan_nilai < 0) {
      return t.ojkData.penyertaan.validationCKPN;
    }
    if (value.ckpn_aset_baik < 0) {
      return t.ojkData.penyertaan.validationCKPNAsetBaik;
    }
    if (value.ckpn_aset_kurang_baik < 0) {
      return t.ojkData.penyertaan.validationCKPNAsetKurangBaik;
    }
    if (value.ckpn_aset_tidak_baik < 0) {
      return t.ojkData.penyertaan.validationCKPNAsetTidakBaik;
    }
    if (!value.as_of) return t.ojkData.penyertaan.validationAsOf;
    if (value.status !== "AKTIF" && value.status !== "NONAKTIF") {
      return t.ojkData.penyertaan.validationStatus;
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
      await request<PenyertaanItem>("/reports/ojk/penyertaan/items", {
        method: "PUT",
        body: {
          id: form.id,
          no_register: form.no_register,
          counterparty_id: form.counterparty_id,
          metode_penyertaan_code: form.metode_penyertaan_code,
          kualitas_code: form.kualitas_code,
          tujuan_penyertaan_code: form.tujuan_penyertaan_code,
          tanggal_mulai: form.tanggal_mulai,
          persentase_penyertaan: form.persentase_penyertaan,
          nominal: form.nominal,
          jumlah_bulan_laporan: form.jumlah_bulan_laporan,
          cadangan_kerugian_penurunan_nilai:
            form.cadangan_kerugian_penurunan_nilai,
          ckpn_aset_baik: form.ckpn_aset_baik,
          ckpn_aset_kurang_baik: form.ckpn_aset_kurang_baik,
          ckpn_aset_tidak_baik: form.ckpn_aset_tidak_baik,
          jenis_ckpn_code: form.jenis_ckpn_code,
          as_of: form.as_of,
          status: form.status,
          note: form.note,
        },
      });
      setFeedback(t.ojkData.penyertaan.saved);
      setForm(null);
      await load();
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setSaveError(t.ojkData.saveForbidden);
      } else if (err instanceof ApiError && err.status === 422) {
        setFieldError(
          isNoRegisterUsed(err)
            ? t.ojkData.penyertaan.noRegisterUsed
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
      await request(`/reports/ojk/penyertaan/items/${deleteTarget.id}`, {
        method: "DELETE",
      });
      setFeedback(t.ojkData.penyertaan.deleted);
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

  const metodeSelectOptions = form
    ? withCurrent(metodeOptions, form.metode_penyertaan_code)
    : metodeOptions;
  const kualitasSelectOptions = form
    ? withCurrent(kualitasOptions, form.kualitas_code)
    : kualitasOptions;
  const tujuanSelectOptions = form
    ? withCurrent(tujuanOptions, form.tujuan_penyertaan_code)
    : tujuanOptions;
  const jenisCKPNSelectOptions = form
    ? withCurrent(jenisCKPNOptions, form.jenis_ckpn_code)
    : jenisCKPNOptions;
  const statusSelectOptions = form
    ? withCurrent(statusOptions, form.status)
    : statusOptions;

  const columns: Column<PenyertaanItem>[] = [
    {
      header: t.ojkData.penyertaan.colNoRegister,
      accessorKey: "no_register",
      isMono: true,
    },
    {
      header: t.ojkData.penyertaan.colCounterparty,
      accessorKey: "counterparty_id",
      isMono: true,
    },
    {
      header: t.ojkData.penyertaan.colMetode,
      cell: (row) => labelOf(metodeOptions, row.metode_penyertaan_code),
    },
    {
      header: t.ojkData.penyertaan.colKualitas,
      cell: (row) => labelOf(kualitasOptions, row.kualitas_code),
    },
    {
      header: t.ojkData.penyertaan.colTujuan,
      cell: (row) => labelOf(tujuanOptions, row.tujuan_penyertaan_code),
    },
    {
      header: t.ojkData.penyertaan.colTanggalMulai,
      cell: (row) => formatDateISO(row.tanggal_mulai) || "-",
      isMono: true,
    },
    {
      header: t.ojkData.penyertaan.colPersentase,
      align: "right",
      cell: (row) => formatRate(row.persentase_penyertaan),
    },
    {
      header: t.ojkData.penyertaan.colNominal,
      type: "money",
      accessorKey: "nominal",
    },
    {
      header: t.ojkData.penyertaan.colJumlahBulanLaporan,
      type: "money",
      accessorKey: "jumlah_bulan_laporan",
    },
    {
      header: t.ojkData.penyertaan.colCKPN,
      type: "money",
      accessorKey: "cadangan_kerugian_penurunan_nilai",
    },
    {
      header: t.ojkData.penyertaan.colCKPNAsetBaik,
      type: "money",
      accessorKey: "ckpn_aset_baik",
    },
    {
      header: t.ojkData.penyertaan.colCKPNAsetKurangBaik,
      type: "money",
      accessorKey: "ckpn_aset_kurang_baik",
    },
    {
      header: t.ojkData.penyertaan.colCKPNAsetTidakBaik,
      type: "money",
      accessorKey: "ckpn_aset_tidak_baik",
    },
    {
      header: t.ojkData.penyertaan.colJenisCKPN,
      cell: (row) => labelOf(jenisCKPNOptions, row.jenis_ckpn_code),
    },
    {
      header: t.ojkData.penyertaan.colAsOf,
      cell: (row) => formatDateISO(row.as_of) || "-",
      isMono: true,
    },
    {
      header: t.ojkData.penyertaan.colStatus,
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
        <CardTitle>{t.ojkData.penyertaan.title}</CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <LoadingState label={t.states.loading} />
        ) : forbidden ? (
          <ErrorState
            title={t.ojkData.penyertaan.title}
            description={t.ojkData.penyertaan.forbidden}
          />
        ) : loadError ? (
          <ErrorState
            title={t.ojkData.penyertaan.title}
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
              {t.ojkData.penyertaan.description}
            </p>
            {!canEdit && <Alert variant="info">{t.ojkData.readOnly}</Alert>}
            {feedback && <Alert variant="success">{feedback}</Alert>}
            {fieldError && <Alert variant="error">{fieldError}</Alert>}
            {saveError && <Alert variant="error">{saveError}</Alert>}

            <div className="flex items-center justify-between gap-4">
              <h4 className="text-title font-semibold text-ink-900">
                {t.ojkData.penyertaan.itemsTitle}
              </h4>
              {canEdit && !form && (
                <Button size="sm" onClick={openNew}>
                  {t.ojkData.penyertaan.add}
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
                    ? t.ojkData.penyertaan.editTitle
                    : t.ojkData.penyertaan.newTitle}
                </h5>
                <div className="grid gap-4 md:grid-cols-2">
                  <Input
                    label={t.ojkData.penyertaan.fieldNoRegister}
                    helperText={t.ojkData.penyertaan.fieldNoRegisterHint}
                    value={form.no_register}
                    onChange={(event) =>
                      updateForm({ no_register: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.penyertaan.fieldCounterparty}
                    helperText={t.ojkData.penyertaan.fieldCounterpartyHint}
                    value={form.counterparty_id}
                    onChange={(event) =>
                      updateForm({ counterparty_id: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.penyertaan.fieldMetode}
                    helperText={t.ojkData.penyertaan.fieldMetodeHint}
                    placeholder={t.ojkData.penyertaan.metodePlaceholder}
                    value={form.metode_penyertaan_code}
                    options={metodeSelectOptions}
                    onChange={(event) =>
                      updateForm({ metode_penyertaan_code: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.penyertaan.fieldKualitas}
                    helperText={t.ojkData.penyertaan.fieldKualitasHint}
                    placeholder={t.ojkData.penyertaan.kualitasPlaceholder}
                    value={form.kualitas_code}
                    options={kualitasSelectOptions}
                    onChange={(event) =>
                      updateForm({ kualitas_code: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.penyertaan.fieldTujuan}
                    helperText={t.ojkData.penyertaan.fieldTujuanHint}
                    placeholder={t.ojkData.penyertaan.tujuanPlaceholder}
                    value={form.tujuan_penyertaan_code}
                    options={tujuanSelectOptions}
                    onChange={(event) =>
                      updateForm({ tujuan_penyertaan_code: event.target.value })
                    }
                  />
                  <DateInput
                    label={t.ojkData.penyertaan.fieldTanggalMulai}
                    helperText={t.ojkData.penyertaan.fieldTanggalMulaiHint}
                    value={form.tanggal_mulai}
                    onChange={(event) =>
                      updateForm({ tanggal_mulai: event.target.value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.penyertaan.fieldPersentase}
                    helperText={t.ojkData.penyertaan.fieldPersentaseHint}
                    currencyPrefix="%"
                    allowDecimals
                    value={form.persentase_penyertaan}
                    onChange={(value) =>
                      updateForm({ persentase_penyertaan: value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.penyertaan.fieldNominal}
                    helperText={t.ojkData.penyertaan.fieldNominalHint}
                    value={form.nominal}
                    onChange={(value) => updateForm({ nominal: value })}
                  />
                  <CurrencyInput
                    label={t.ojkData.penyertaan.fieldJumlahBulanLaporan}
                    helperText={
                      t.ojkData.penyertaan.fieldJumlahBulanLaporanHint
                    }
                    value={form.jumlah_bulan_laporan}
                    onChange={(value) =>
                      updateForm({ jumlah_bulan_laporan: value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.penyertaan.fieldCKPN}
                    helperText={t.ojkData.penyertaan.fieldCKPNHint}
                    value={form.cadangan_kerugian_penurunan_nilai}
                    onChange={(value) =>
                      updateForm({ cadangan_kerugian_penurunan_nilai: value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.penyertaan.fieldCKPNAsetBaik}
                    helperText={t.ojkData.penyertaan.fieldCKPNAsetBaikHint}
                    value={form.ckpn_aset_baik}
                    onChange={(value) => updateForm({ ckpn_aset_baik: value })}
                  />
                  <CurrencyInput
                    label={t.ojkData.penyertaan.fieldCKPNAsetKurangBaik}
                    helperText={
                      t.ojkData.penyertaan.fieldCKPNAsetKurangBaikHint
                    }
                    value={form.ckpn_aset_kurang_baik}
                    onChange={(value) =>
                      updateForm({ ckpn_aset_kurang_baik: value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.penyertaan.fieldCKPNAsetTidakBaik}
                    helperText={t.ojkData.penyertaan.fieldCKPNAsetTidakBaikHint}
                    value={form.ckpn_aset_tidak_baik}
                    onChange={(value) =>
                      updateForm({ ckpn_aset_tidak_baik: value })
                    }
                  />
                  <Select
                    label={t.ojkData.penyertaan.fieldJenisCKPN}
                    helperText={t.ojkData.penyertaan.fieldJenisCKPNHint}
                    placeholder={t.ojkData.penyertaan.jenisCKPNPlaceholder}
                    value={form.jenis_ckpn_code}
                    options={jenisCKPNSelectOptions}
                    onChange={(event) =>
                      updateForm({ jenis_ckpn_code: event.target.value })
                    }
                  />
                  <DateInput
                    label={t.ojkData.penyertaan.fieldAsOf}
                    helperText={t.ojkData.penyertaan.fieldAsOfHint}
                    value={form.as_of}
                    onChange={(event) =>
                      updateForm({ as_of: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.penyertaan.fieldStatus}
                    value={form.status}
                    options={statusSelectOptions}
                    onChange={(event) =>
                      updateForm({ status: event.target.value })
                    }
                  />
                </div>

                <Textarea
                  label={t.ojkData.penyertaan.fieldNote}
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
              emptyMessage={t.ojkData.penyertaan.empty}
              zebra
            />
          </>
        )}
      </CardContent>

      <ConfirmDialog
        open={deleteTarget !== null}
        title={t.ojkData.penyertaan.deleteConfirmTitle}
        description={t.ojkData.penyertaan.deleteConfirmBody}
        confirmLabel={
          deleting
            ? t.ojkData.penyertaan.deactivating
            : t.ojkData.penyertaan.deactivate
        }
        destructive
        loading={deleting}
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </Card>
  );
}
