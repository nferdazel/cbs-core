"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, request } from "@/lib/api";
import type { SindikasiItem, SindikasiItemsData } from "@/lib/operations-types";
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
import { withCurrent, labelOf, type Option } from "./register-options";

interface SindikasiForm {
  id: string;
  counterparty_id: string;
  no_rekening: string;
  jumlah_pendanaan_sindikasi: number;
  bagian_pendanaan: number;
  sandi_bank_peserta: string;
  plafon: number;
  baki_debet: number;
  status_kepesertaan_code: string;
  nomor_perjanjian_induk: string;
  pendanaan_di_bank_pelapor_code: string;
  kualitas_code: string;
  tunggakan_pokok: number;
  tunggakan_bunga: number;
  hari_tunggakan_pokok: number;
  hari_tunggakan_bunga: number;
  as_of: string;
  status: string;
  note: string;
}

const EMPTY_ITEM: SindikasiForm = {
  id: "",
  counterparty_id: "",
  no_rekening: "",
  jumlah_pendanaan_sindikasi: 0,
  bagian_pendanaan: 0,
  sandi_bank_peserta: "",
  plafon: 0,
  baki_debet: 0,
  status_kepesertaan_code: "",
  nomor_perjanjian_induk: "",
  pendanaan_di_bank_pelapor_code: "1",
  kualitas_code: "",
  tunggakan_pokok: 0,
  tunggakan_bunga: 0,
  hari_tunggakan_pokok: 0,
  hari_tunggakan_bunga: 0,
  as_of: "",
  status: "AKTIF",
  note: "",
};

/** Sandi baku Form 06.02 (domain/kredit_sindikasi.go), semuanya isian bank. */
const KEPESERTAAN_CODES: string[] = ["1", "2"];
const PENDANAAN_CODES: string[] = ["1", "2"];
const KUALITAS_CODES: string[] = ["1", "2", "3", "4", "5"];
const NO_REKENING_MAX = 64;
const COUNTERPARTY_MAX = 64;
const SANDI_BANK_MAX = 16;

function itemFormFrom(row: SindikasiItem): SindikasiForm {
  return {
    id: row.id,
    counterparty_id: row.counterparty_id ?? "",
    no_rekening: row.no_rekening ?? "",
    jumlah_pendanaan_sindikasi: Number(row.jumlah_pendanaan_sindikasi) || 0,
    bagian_pendanaan: Number(row.bagian_pendanaan) || 0,
    sandi_bank_peserta: row.sandi_bank_peserta ?? "",
    plafon: Number(row.plafon) || 0,
    baki_debet: Number(row.baki_debet) || 0,
    status_kepesertaan_code: row.status_kepesertaan_code ?? "",
    nomor_perjanjian_induk: row.nomor_perjanjian_induk ?? "",
    pendanaan_di_bank_pelapor_code: row.pendanaan_di_bank_pelapor_code ?? "1",
    kualitas_code: row.kualitas_code ?? "",
    tunggakan_pokok: Number(row.tunggakan_pokok) || 0,
    tunggakan_bunga: Number(row.tunggakan_bunga) || 0,
    hari_tunggakan_pokok: Number(row.hari_tunggakan_pokok) || 0,
    hari_tunggakan_bunga: Number(row.hari_tunggakan_bunga) || 0,
    as_of: formatDateISO(row.as_of),
    status: row.status ?? "",
    note: row.note ?? "",
  };
}

/**
 * 422 nomor rekening terpakai (katalog API kredit_sindikasi_no_rekening_used). API
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
 * Kartu pengisian register kredit sindikasi Form 06.02: satu baris per rekening
 * fasilitas kredit sindikasi. Data sebelumnya hanya dapat diisi lewat API/SQL.
 * Penjagaan izin sebenarnya di API (system:config); kartu hanya menyembunyikan kontrol
 * tulis saat peran tidak memegang izin itu. Form ini tidak punya baris JUMLAH dan tidak
 * ada kolom turunan: seluruh angka adalah isian bank. No. Rekening wajib diisi hanya
 * bila pendanaan di bank pelapor = 1 (ya), dan harus kosong bila 2 (tidak), sesuai
 * penjelasan PDF #page 177. Kolom III No. Identitas tidak ada di payload klien karena
 * sengaja tidak disimpan (keputusan privasi). Kolom I Sandi Kantor diambil server dari
 * kantor pelapor. Hapus adalah soft-delete: baris menjadi NONAKTIF dan nomor rekening
 * tetap terkunci ("tidak boleh sama" menurut form).
 */
export function OJKKreditSindikasiCard() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const canEdit = hasPermission(user, "system:config");

  const [items, setItems] = useState<SindikasiItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<SindikasiItem | null>(null);
  const [form, setForm] = useState<SindikasiForm | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    setForbidden(false);
    try {
      const response = await request<SindikasiItemsData>(
        "/reports/ojk/kredit-sindikasi/items",
      );
      setItems(response.data?.items ?? []);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setForbidden(true);
      } else {
        setLoadError(
          err instanceof ApiError
            ? err.message
            : t.ojkData.kreditSindikasi.loadError,
        );
      }
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    load();
  }, [load]);

  const kepesertaanOptions: Option[] = [
    { value: "1", label: t.ojkData.kreditSindikasi.kepesertaanArranger },
    { value: "2", label: t.ojkData.kreditSindikasi.kepesertaanAnggota },
  ];
  const pendanaanOptions: Option[] = [
    { value: "1", label: t.ojkData.kreditSindikasi.pendanaanYa },
    { value: "2", label: t.ojkData.kreditSindikasi.pendanaanTidak },
  ];
  const kualitasOptions: Option[] = [
    { value: "1", label: t.ojkData.kreditSindikasi.kualitasLancar },
    {
      value: "2",
      label: t.ojkData.kreditSindikasi.kualitasDalamPerhatianKhusus,
    },
    { value: "3", label: t.ojkData.kreditSindikasi.kualitasKurangLancar },
    { value: "4", label: t.ojkData.kreditSindikasi.kualitasDiragukan },
    { value: "5", label: t.ojkData.kreditSindikasi.kualitasMacet },
  ];
  const statusOptions: Option[] = [
    { value: "AKTIF", label: t.ojkData.kreditSindikasi.statusAktif },
    { value: "NONAKTIF", label: t.ojkData.kreditSindikasi.statusNonaktif },
  ];

  const updateForm = (patch: Partial<SindikasiForm>) =>
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

  const openEdit = (row: SindikasiItem) => {
    resetMessages();
    setForm(itemFormFrom(row));
  };

  /** Batasan yang sama dengan domain.BuildSindikasiItem, dicek sebelum menembak API. */
  const validate = (value: SindikasiForm): string | null => {
    const counterparty = value.counterparty_id.trim();
    if (!counterparty) {
      return t.ojkData.kreditSindikasi.validationCounterparty;
    }
    if (counterparty.length > COUNTERPARTY_MAX) {
      return t.ojkData.kreditSindikasi.validationCounterpartyLength;
    }
    const noRekening = value.no_rekening.trim();
    if (value.pendanaan_di_bank_pelapor_code === "1" && !noRekening) {
      return t.ojkData.kreditSindikasi.validationNoRekeningRequired;
    }
    if (value.pendanaan_di_bank_pelapor_code === "2" && noRekening) {
      return t.ojkData.kreditSindikasi.validationNoRekeningMustBeEmpty;
    }
    if (noRekening.length > NO_REKENING_MAX) {
      return t.ojkData.kreditSindikasi.validationNoRekeningLength;
    }
    if (value.jumlah_pendanaan_sindikasi < 0) {
      return t.ojkData.kreditSindikasi.validationJumlahPendanaan;
    }
    if (value.bagian_pendanaan < 0) {
      return t.ojkData.kreditSindikasi.validationBagianPendanaan;
    }
    if (value.sandi_bank_peserta.trim().length > SANDI_BANK_MAX) {
      return t.ojkData.kreditSindikasi.validationSandiBankPesertaLength;
    }
    if (value.plafon < 0) {
      return t.ojkData.kreditSindikasi.validationPlafon;
    }
    if (value.baki_debet < 0) {
      return t.ojkData.kreditSindikasi.validationBakiDebet;
    }
    if (!KEPESERTAAN_CODES.includes(value.status_kepesertaan_code)) {
      return t.ojkData.kreditSindikasi.validationKepesertaan;
    }
    if (!value.nomor_perjanjian_induk.trim()) {
      return t.ojkData.kreditSindikasi.validationNomorPerjanjian;
    }
    if (value.nomor_perjanjian_induk.trim().length > NO_REKENING_MAX) {
      return t.ojkData.kreditSindikasi.validationNomorPerjanjianLength;
    }
    if (!PENDANAAN_CODES.includes(value.pendanaan_di_bank_pelapor_code)) {
      return t.ojkData.kreditSindikasi.validationPendanaan;
    }
    if (!KUALITAS_CODES.includes(value.kualitas_code)) {
      return t.ojkData.kreditSindikasi.validationKualitas;
    }
    if (value.tunggakan_pokok < 0) {
      return t.ojkData.kreditSindikasi.validationTunggakanPokok;
    }
    if (value.tunggakan_bunga < 0) {
      return t.ojkData.kreditSindikasi.validationTunggakanBunga;
    }
    if (value.hari_tunggakan_pokok < 0) {
      return t.ojkData.kreditSindikasi.validationHariTunggakanPokok;
    }
    if (value.hari_tunggakan_bunga < 0) {
      return t.ojkData.kreditSindikasi.validationHariTunggakanBunga;
    }
    if (!value.as_of) {
      return t.ojkData.kreditSindikasi.validationAsOf;
    }
    if (value.status !== "AKTIF" && value.status !== "NONAKTIF") {
      return t.ojkData.kreditSindikasi.validationStatus;
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
      await request<SindikasiItem>("/reports/ojk/kredit-sindikasi/items", {
        method: "PUT",
        body: {
          id: form.id,
          counterparty_id: form.counterparty_id,
          no_rekening: form.no_rekening,
          jumlah_pendanaan_sindikasi: form.jumlah_pendanaan_sindikasi,
          bagian_pendanaan: form.bagian_pendanaan,
          sandi_bank_peserta: form.sandi_bank_peserta,
          plafon: form.plafon,
          baki_debet: form.baki_debet,
          status_kepesertaan_code: form.status_kepesertaan_code,
          nomor_perjanjian_induk: form.nomor_perjanjian_induk,
          pendanaan_di_bank_pelapor_code: form.pendanaan_di_bank_pelapor_code,
          kualitas_code: form.kualitas_code,
          tunggakan_pokok: form.tunggakan_pokok,
          tunggakan_bunga: form.tunggakan_bunga,
          hari_tunggakan_pokok: form.hari_tunggakan_pokok,
          hari_tunggakan_bunga: form.hari_tunggakan_bunga,
          as_of: form.as_of,
          status: form.status,
          note: form.note,
        },
      });
      setFeedback(t.ojkData.kreditSindikasi.saved);
      setForm(null);
      await load();
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setSaveError(t.ojkData.saveForbidden);
      } else if (err instanceof ApiError && err.status === 422) {
        setFieldError(
          isNoRekeningUsed(err)
            ? t.ojkData.kreditSindikasi.noRekeningUsed
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
      await request(`/reports/ojk/kredit-sindikasi/items/${deleteTarget.id}`, {
        method: "DELETE",
      });
      setFeedback(t.ojkData.kreditSindikasi.deleted);
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

  const kepesertaanSelectOptions = form
    ? withCurrent(kepesertaanOptions, form.status_kepesertaan_code)
    : kepesertaanOptions;
  const pendanaanSelectOptions = form
    ? withCurrent(pendanaanOptions, form.pendanaan_di_bank_pelapor_code)
    : pendanaanOptions;
  const kualitasSelectOptions = form
    ? withCurrent(kualitasOptions, form.kualitas_code)
    : kualitasOptions;
  const statusSelectOptions = form
    ? withCurrent(statusOptions, form.status)
    : statusOptions;

  const columns: Column<SindikasiItem>[] = [
    {
      header: t.ojkData.kreditSindikasi.colCounterparty,
      accessorKey: "counterparty_id",
      isMono: true,
    },
    {
      header: t.ojkData.kreditSindikasi.colNoRekening,
      cell: (row) => row.no_rekening || "-",
      isMono: true,
    },
    {
      header: t.ojkData.kreditSindikasi.colJumlahPendanaan,
      type: "money",
      accessorKey: "jumlah_pendanaan_sindikasi",
    },
    {
      header: t.ojkData.kreditSindikasi.colBagianPendanaan,
      type: "money",
      accessorKey: "bagian_pendanaan",
    },
    {
      header: t.ojkData.kreditSindikasi.colSandiBankPeserta,
      cell: (row) => row.sandi_bank_peserta || "-",
      isMono: true,
    },
    {
      header: t.ojkData.kreditSindikasi.colPlafon,
      type: "money",
      accessorKey: "plafon",
    },
    {
      header: t.ojkData.kreditSindikasi.colBakiDebet,
      type: "money",
      accessorKey: "baki_debet",
    },
    {
      header: t.ojkData.kreditSindikasi.colKepesertaan,
      cell: (row) => labelOf(kepesertaanOptions, row.status_kepesertaan_code),
    },
    {
      header: t.ojkData.kreditSindikasi.colNomorPerjanjian,
      accessorKey: "nomor_perjanjian_induk",
      isMono: true,
    },
    {
      header: t.ojkData.kreditSindikasi.colPendanaan,
      cell: (row) =>
        labelOf(pendanaanOptions, row.pendanaan_di_bank_pelapor_code),
    },
    {
      header: t.ojkData.kreditSindikasi.colKualitas,
      cell: (row) => labelOf(kualitasOptions, row.kualitas_code),
    },
    {
      header: t.ojkData.kreditSindikasi.colTunggakanPokok,
      type: "money",
      accessorKey: "tunggakan_pokok",
    },
    {
      header: t.ojkData.kreditSindikasi.colTunggakanBunga,
      type: "money",
      accessorKey: "tunggakan_bunga",
    },
    {
      header: t.ojkData.kreditSindikasi.colHariTunggakanPokok,
      align: "right",
      accessorKey: "hari_tunggakan_pokok",
    },
    {
      header: t.ojkData.kreditSindikasi.colHariTunggakanBunga,
      align: "right",
      accessorKey: "hari_tunggakan_bunga",
    },
    {
      header: t.ojkData.kreditSindikasi.colAsOf,
      cell: (row) => formatDateISO(row.as_of) || "-",
      isMono: true,
    },
    {
      header: t.ojkData.kreditSindikasi.colStatus,
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

  /** No. Rekening wajib diisi hanya bila pendanaan di bank pelapor = 1 (ya). */
  const noRekeningRequired = form?.pendanaan_di_bank_pelapor_code === "1";

  return (
    <Card className="mb-4">
      <CardHeader>
        <CardTitle>{t.ojkData.kreditSindikasi.title}</CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <LoadingState label={t.states.loading} />
        ) : forbidden ? (
          <ErrorState
            title={t.ojkData.kreditSindikasi.title}
            description={t.ojkData.kreditSindikasi.forbidden}
          />
        ) : loadError ? (
          <ErrorState
            title={t.ojkData.kreditSindikasi.title}
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
              {t.ojkData.kreditSindikasi.description}
            </p>
            {!canEdit && <Alert variant="info">{t.ojkData.readOnly}</Alert>}
            {feedback && <Alert variant="success">{feedback}</Alert>}
            {fieldError && <Alert variant="error">{fieldError}</Alert>}
            {saveError && <Alert variant="error">{saveError}</Alert>}

            <div className="flex items-center justify-between gap-4">
              <h4 className="text-title font-semibold text-ink-900">
                {t.ojkData.kreditSindikasi.itemsTitle}
              </h4>
              {canEdit && !form && (
                <Button size="sm" onClick={openNew}>
                  {t.ojkData.kreditSindikasi.add}
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
                    ? t.ojkData.kreditSindikasi.editTitle
                    : t.ojkData.kreditSindikasi.newTitle}
                </h5>
                <div className="grid gap-4 md:grid-cols-2">
                  <Input
                    label={t.ojkData.kreditSindikasi.fieldCounterparty}
                    helperText={t.ojkData.kreditSindikasi.fieldCounterpartyHint}
                    value={form.counterparty_id}
                    onChange={(event) =>
                      updateForm({ counterparty_id: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.kreditSindikasi.fieldPendanaan}
                    helperText={t.ojkData.kreditSindikasi.fieldPendanaanHint}
                    value={form.pendanaan_di_bank_pelapor_code}
                    options={pendanaanSelectOptions}
                    onChange={(event) =>
                      updateForm({
                        pendanaan_di_bank_pelapor_code: event.target.value,
                      })
                    }
                  />
                  <Input
                    label={t.ojkData.kreditSindikasi.fieldNoRekening}
                    helperText={t.ojkData.kreditSindikasi.fieldNoRekeningHint}
                    required={noRekeningRequired}
                    value={form.no_rekening}
                    onChange={(event) =>
                      updateForm({ no_rekening: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.kreditSindikasi.fieldNomorPerjanjian}
                    helperText={
                      t.ojkData.kreditSindikasi.fieldNomorPerjanjianHint
                    }
                    value={form.nomor_perjanjian_induk}
                    onChange={(event) =>
                      updateForm({ nomor_perjanjian_induk: event.target.value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.kreditSindikasi.fieldJumlahPendanaan}
                    helperText={
                      t.ojkData.kreditSindikasi.fieldJumlahPendanaanHint
                    }
                    value={form.jumlah_pendanaan_sindikasi}
                    onChange={(value) =>
                      updateForm({ jumlah_pendanaan_sindikasi: value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.kreditSindikasi.fieldBagianPendanaan}
                    helperText={
                      t.ojkData.kreditSindikasi.fieldBagianPendanaanHint
                    }
                    value={form.bagian_pendanaan}
                    onChange={(value) =>
                      updateForm({ bagian_pendanaan: value })
                    }
                  />
                  <Input
                    label={t.ojkData.kreditSindikasi.fieldSandiBankPeserta}
                    helperText={
                      t.ojkData.kreditSindikasi.fieldSandiBankPesertaHint
                    }
                    value={form.sandi_bank_peserta}
                    onChange={(event) =>
                      updateForm({ sandi_bank_peserta: event.target.value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.kreditSindikasi.fieldPlafon}
                    helperText={t.ojkData.kreditSindikasi.fieldPlafonHint}
                    value={form.plafon}
                    onChange={(value) => updateForm({ plafon: value })}
                  />
                  <CurrencyInput
                    label={t.ojkData.kreditSindikasi.fieldBakiDebet}
                    helperText={t.ojkData.kreditSindikasi.fieldBakiDebetHint}
                    value={form.baki_debet}
                    onChange={(value) => updateForm({ baki_debet: value })}
                  />
                  <Select
                    label={t.ojkData.kreditSindikasi.fieldKepesertaan}
                    helperText={t.ojkData.kreditSindikasi.fieldKepesertaanHint}
                    placeholder={
                      t.ojkData.kreditSindikasi.kepesertaanPlaceholder
                    }
                    value={form.status_kepesertaan_code}
                    options={kepesertaanSelectOptions}
                    onChange={(event) =>
                      updateForm({
                        status_kepesertaan_code: event.target.value,
                      })
                    }
                  />
                  <Select
                    label={t.ojkData.kreditSindikasi.fieldKualitas}
                    helperText={t.ojkData.kreditSindikasi.fieldKualitasHint}
                    placeholder={t.ojkData.kreditSindikasi.kualitasPlaceholder}
                    value={form.kualitas_code}
                    options={kualitasSelectOptions}
                    onChange={(event) =>
                      updateForm({ kualitas_code: event.target.value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.kreditSindikasi.fieldTunggakanPokok}
                    helperText={
                      t.ojkData.kreditSindikasi.fieldTunggakanPokokHint
                    }
                    value={form.tunggakan_pokok}
                    onChange={(value) => updateForm({ tunggakan_pokok: value })}
                  />
                  <CurrencyInput
                    label={t.ojkData.kreditSindikasi.fieldTunggakanBunga}
                    helperText={
                      t.ojkData.kreditSindikasi.fieldTunggakanBungaHint
                    }
                    value={form.tunggakan_bunga}
                    onChange={(value) => updateForm({ tunggakan_bunga: value })}
                  />
                  <CurrencyInput
                    label={t.ojkData.kreditSindikasi.fieldHariTunggakanPokok}
                    helperText={
                      t.ojkData.kreditSindikasi.fieldHariTunggakanPokokHint
                    }
                    allowDecimals={false}
                    value={form.hari_tunggakan_pokok}
                    onChange={(value) =>
                      updateForm({ hari_tunggakan_pokok: value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.kreditSindikasi.fieldHariTunggakanBunga}
                    helperText={
                      t.ojkData.kreditSindikasi.fieldHariTunggakanBungaHint
                    }
                    allowDecimals={false}
                    value={form.hari_tunggakan_bunga}
                    onChange={(value) =>
                      updateForm({ hari_tunggakan_bunga: value })
                    }
                  />
                  <DateInput
                    label={t.ojkData.kreditSindikasi.fieldAsOf}
                    helperText={t.ojkData.kreditSindikasi.fieldAsOfHint}
                    value={form.as_of}
                    onChange={(event) =>
                      updateForm({ as_of: event.target.value })
                    }
                  />
                  <Select
                    label={t.ojkData.kreditSindikasi.fieldStatus}
                    value={form.status}
                    options={statusSelectOptions}
                    onChange={(event) =>
                      updateForm({ status: event.target.value })
                    }
                  />
                </div>

                <Textarea
                  label={t.ojkData.kreditSindikasi.fieldNote}
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
              emptyMessage={t.ojkData.kreditSindikasi.empty}
              zebra
            />
          </>
        )}
      </CardContent>

      <ConfirmDialog
        open={deleteTarget !== null}
        title={t.ojkData.kreditSindikasi.deleteConfirmTitle}
        description={t.ojkData.kreditSindikasi.deleteConfirmBody}
        confirmLabel={
          deleting
            ? t.ojkData.kreditSindikasi.deactivating
            : t.ojkData.kreditSindikasi.deactivate
        }
        destructive
        loading={deleting}
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </Card>
  );
}
