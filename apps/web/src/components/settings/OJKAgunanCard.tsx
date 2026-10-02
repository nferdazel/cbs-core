"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, request } from "@/lib/api";
import type { AgunanRow, AgunanItemsData } from "@/lib/operations-types";
import { formatDateISO } from "@/lib/format";
import { useAuth } from "@/lib/useAuth";
import { hasPermission } from "@/lib/permissions";
import { useTranslation } from "@/i18n/context";
import { Alert } from "@/components/ui/Alert";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { CurrencyInput } from "@/components/ui/CurrencyInput";
import { DataTable, Column } from "@/components/ui/DataTable";
import { DateInput } from "@/components/ui/DateInput";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { ErrorState, LoadingState } from "@/components/ui/States";
import { withCurrent, labelOf, type Option } from "./register-options";

interface AgunanForm {
  id: string;
  kode_register: string;
  jenis_agunan_code: string;
  alamat_agunan: string;
  nilai_diagunkan: number;
  nilai_agunan: number;
  penilai_code: string;
  tanggal_penilaian: string;
  ppka_amount: number;
}

const KODE_REGISTER_MAX = 64;

/** Sandi baku Form 06.01 kolom VII.b (domain/form06_01_agunan.go). */
const PENILAI_CODES: string[] = ["1", "2"];

function formFrom(row: AgunanRow): AgunanForm {
  return {
    id: row.id,
    kode_register: row.kode_register ?? "",
    jenis_agunan_code: row.jenis_agunan_code ?? "",
    alamat_agunan: row.alamat_agunan ?? "",
    nilai_diagunkan: Number(row.nilai_diagunkan) || 0,
    nilai_agunan: Number(row.nilai_agunan) || 0,
    penilai_code: row.penilai_code ?? "",
    tanggal_penilaian: formatDateISO(row.tanggal_penilaian),
    ppka_amount: Number(row.ppka_amount) || 0,
  };
}

/**
 * 422 kode register terpakai (katalog API agunan_register_used). API mengirim pesan
 * terterjemah, bukan kode mesin, jadi kecocokan lewat penanda pesan katalog ID/EN.
 */
function isKodeRegisterUsed(err: ApiError): boolean {
  const message = err.message.toLowerCase();
  return (
    message.includes("tidak boleh dipakai ulang") ||
    message.includes("cannot be reused")
  );
}

/**
 * Kartu pengisian kolom Form 06.01 pada agunan yang sudah tercatat. Ini BUKAN modul
 * agunan: kartu TIDAK membuat atau menghapus agunan, hanya melengkapi kolom pelaporan
 * OJK-nya. Penjagaan izin sebenarnya di API (system:config). Seluruh angka adalah isian
 * bank dan tidak dihitung klien maupun server. Kolom I Sandi Kantor diambil server dari
 * kantor pelapor. Kode register wajib dan tidak boleh dipakai ulang.
 */
export function OJKAgunanCard() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const canEdit = hasPermission(user, "system:config");

  const [items, setItems] = useState<AgunanRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState<AgunanForm | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    setForbidden(false);
    try {
      const response = await request<AgunanItemsData>(
        "/reports/ojk/agunan/items",
      );
      setItems(response.data?.items ?? []);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setForbidden(true);
      } else {
        setLoadError(
          err instanceof ApiError ? err.message : t.ojkData.agunan.loadError,
        );
      }
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    load();
  }, [load]);

  const penilaiOptions: Option[] = [
    { value: "1", label: t.ojkData.agunan.penilaiIndependen },
    { value: "2", label: t.ojkData.agunan.penilaiInternal },
  ];

  const updateForm = (patch: Partial<AgunanForm>) =>
    setForm((prev) => (prev ? { ...prev, ...patch } : prev));

  const resetMessages = () => {
    setFeedback(null);
    setFieldError(null);
    setSaveError(null);
  };

  const openEdit = (row: AgunanRow) => {
    resetMessages();
    setForm(formFrom(row));
  };

  /** Batasan yang sama dengan domain.BuildAgunanUpdate, dicek sebelum menembak API. */
  const validate = (value: AgunanForm): string | null => {
    const kode = value.kode_register.trim();
    if (!kode) return t.ojkData.agunan.validationKodeRegister;
    if (kode.length > KODE_REGISTER_MAX) {
      return t.ojkData.agunan.validationKodeRegisterLength;
    }
    if (!value.jenis_agunan_code.trim()) {
      return t.ojkData.agunan.validationJenisAgunan;
    }
    if (!value.alamat_agunan.trim()) {
      return t.ojkData.agunan.validationAlamat;
    }
    if (value.nilai_diagunkan < 0) {
      return t.ojkData.agunan.validationNilaiDiagunkan;
    }
    if (value.nilai_agunan <= 0) {
      return t.ojkData.agunan.validationNilaiAgunan;
    }
    if (!PENILAI_CODES.includes(value.penilai_code)) {
      return t.ojkData.agunan.validationPenilai;
    }
    if (!value.tanggal_penilaian) {
      return t.ojkData.agunan.validationTanggalPenilaian;
    }
    if (value.ppka_amount < 0) {
      return t.ojkData.agunan.validationPPKA;
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
      await request(`/reports/ojk/agunan/items/${form.id}`, {
        method: "PUT",
        body: {
          kode_register: form.kode_register,
          jenis_agunan_code: form.jenis_agunan_code,
          alamat_agunan: form.alamat_agunan,
          nilai_diagunkan: form.nilai_diagunkan,
          nilai_agunan: form.nilai_agunan,
          penilai_code: form.penilai_code,
          tanggal_penilaian: form.tanggal_penilaian,
          ppka_amount: form.ppka_amount,
        },
      });
      setFeedback(t.ojkData.agunan.saved);
      setForm(null);
      await load();
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setSaveError(t.ojkData.saveForbidden);
      } else if (err instanceof ApiError && err.status === 422) {
        setFieldError(
          isKodeRegisterUsed(err)
            ? t.ojkData.agunan.noKodeRegisterUsed
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

  const penilaiSelectOptions = form
    ? withCurrent(penilaiOptions, form.penilai_code)
    : penilaiOptions;

  const columns: Column<AgunanRow>[] = [
    {
      header: t.ojkData.agunan.colLoanNumber,
      accessorKey: "loan_number",
      isMono: true,
    },
    {
      header: t.ojkData.agunan.colKodeRegister,
      cell: (row) => row.kode_register || "-",
      isMono: true,
    },
    {
      header: t.ojkData.agunan.colJenisAgunan,
      cell: (row) => row.jenis_agunan_code || "-",
      isMono: true,
    },
    {
      header: t.ojkData.agunan.colAlamat,
      cell: (row) => row.alamat_agunan || "-",
    },
    {
      header: t.ojkData.agunan.colNilaiDiagunkan,
      type: "money",
      accessorKey: "nilai_diagunkan",
    },
    {
      header: t.ojkData.agunan.colNilaiAgunan,
      type: "money",
      accessorKey: "nilai_agunan",
    },
    {
      header: t.ojkData.agunan.colPenilai,
      cell: (row) => labelOf(penilaiOptions, row.penilai_code),
    },
    {
      header: t.ojkData.agunan.colTanggalPenilaian,
      cell: (row) => formatDateISO(row.tanggal_penilaian) || "-",
      isMono: true,
    },
    {
      header: t.ojkData.agunan.colPPKA,
      type: "money",
      accessorKey: "ppka_amount",
    },
  ];
  if (canEdit) {
    columns.push({
      header: t.common.actions,
      align: "right",
      cell: (row) => (
        <Button size="sm" variant="secondary" onClick={() => openEdit(row)}>
          {t.ojkData.edit}
        </Button>
      ),
    });
  }

  return (
    <Card className="mb-4">
      <CardHeader>
        <CardTitle>{t.ojkData.agunan.title}</CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <LoadingState label={t.states.loading} />
        ) : forbidden ? (
          <ErrorState
            title={t.ojkData.agunan.title}
            description={t.ojkData.agunan.forbidden}
          />
        ) : loadError ? (
          <ErrorState
            title={t.ojkData.agunan.title}
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
              {t.ojkData.agunan.description}
            </p>
            {!canEdit && <Alert variant="info">{t.ojkData.readOnly}</Alert>}
            {feedback && <Alert variant="success">{feedback}</Alert>}
            {fieldError && <Alert variant="error">{fieldError}</Alert>}
            {saveError && <Alert variant="error">{saveError}</Alert>}

            <h4 className="text-title font-semibold text-ink-900">
              {t.ojkData.agunan.itemsTitle}
            </h4>

            {form && (
              <form
                onSubmit={onSubmit}
                className="space-y-4 rounded-md border border-border p-4"
              >
                <h5 className="text-meta font-medium uppercase tracking-wide text-ink-600">
                  {t.ojkData.agunan.editTitle}
                </h5>
                <div className="grid gap-4 md:grid-cols-2">
                  <Input
                    label={t.ojkData.agunan.fieldKodeRegister}
                    helperText={t.ojkData.agunan.fieldKodeRegisterHint}
                    value={form.kode_register}
                    onChange={(event) =>
                      updateForm({ kode_register: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.agunan.fieldJenisAgunan}
                    helperText={t.ojkData.agunan.fieldJenisAgunanHint}
                    value={form.jenis_agunan_code}
                    onChange={(event) =>
                      updateForm({ jenis_agunan_code: event.target.value })
                    }
                  />
                  <Input
                    label={t.ojkData.agunan.fieldAlamat}
                    helperText={t.ojkData.agunan.fieldAlamatHint}
                    value={form.alamat_agunan}
                    onChange={(event) =>
                      updateForm({ alamat_agunan: event.target.value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.agunan.fieldNilaiDiagunkan}
                    helperText={t.ojkData.agunan.fieldNilaiDiagunkanHint}
                    value={form.nilai_diagunkan}
                    onChange={(value) => updateForm({ nilai_diagunkan: value })}
                  />
                  <CurrencyInput
                    label={t.ojkData.agunan.fieldNilaiAgunan}
                    helperText={t.ojkData.agunan.fieldNilaiAgunanHint}
                    value={form.nilai_agunan}
                    onChange={(value) => updateForm({ nilai_agunan: value })}
                  />
                  <Select
                    label={t.ojkData.agunan.fieldPenilai}
                    helperText={t.ojkData.agunan.fieldPenilaiHint}
                    placeholder={t.ojkData.agunan.penilaiPlaceholder}
                    value={form.penilai_code}
                    options={penilaiSelectOptions}
                    onChange={(event) =>
                      updateForm({ penilai_code: event.target.value })
                    }
                  />
                  <DateInput
                    label={t.ojkData.agunan.fieldTanggalPenilaian}
                    helperText={t.ojkData.agunan.fieldTanggalPenilaianHint}
                    value={form.tanggal_penilaian}
                    onChange={(event) =>
                      updateForm({ tanggal_penilaian: event.target.value })
                    }
                  />
                  <CurrencyInput
                    label={t.ojkData.agunan.fieldPPKA}
                    helperText={t.ojkData.agunan.fieldPPKAHint}
                    value={form.ppka_amount}
                    onChange={(value) => updateForm({ ppka_amount: value })}
                  />
                </div>

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
              emptyMessage={t.ojkData.agunan.empty}
              zebra
            />
          </>
        )}
      </CardContent>
    </Card>
  );
}
