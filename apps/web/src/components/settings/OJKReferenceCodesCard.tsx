"use client";

import { useCallback, useEffect, useState } from "react";
import type { Customer } from "@cbs/shared-types";
import { ApiError, request } from "@/lib/api";
import type { Loan } from "@/lib/operations-types";
import { useAuth } from "@/lib/useAuth";
import { hasPermission } from "@/lib/permissions";
import { useTranslation } from "@/i18n/context";
import { formatDateISO } from "@/lib/format";
import { Alert } from "@/components/ui/Alert";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { CurrencyInput } from "@/components/ui/CurrencyInput";
import { DateInput } from "@/components/ui/DateInput";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { LoadingState } from "@/components/ui/States";

// Himpunan sandi inline Form 06.00-2 (Lampiran II SEOJK 16/2024). Nilai harus sama
// dengan validator API; di sini hanya dipakai menyusun pilihan, bukan menebak nilai.
const JENIS_PENGGUNAAN_CODES = ["10", "20", "31", "32", "35", "39"];
const PERIODE_PEMBAYARAN_CODES = ["1", "2", "3", "4", "5", "6", "7", "8"];
const HUBUNGAN_BANK_CODES = ["11", "12", "20"];

interface CustomerCodesForm {
  pihakLawan: string;
  sektorEkonomi: string;
  hubunganBank: string;
}

function customerCodesFrom(customer: Customer | null): CustomerCodesForm {
  return {
    pihakLawan: customer?.ojk_pihak_lawan_code ?? "",
    sektorEkonomi: customer?.ojk_sektor_ekonomi_code ?? "",
    hubunganBank: customer?.ojk_hubungan_bank_code ?? "",
  };
}

/**
 * Sandi OJK nasabah (Form 06.00 VIII/IX/XX). Nasabah dicari lewat CIF, lalu seluruh
 * bidang inti dikirim ulang apa adanya bersama sandi OJK agar PUT /customers/{id}
 * yang sudah ada tidak kehilangan data. String kosong berarti "kosongkan".
 */
function CustomerCodesSection({ canEdit }: { canEdit: boolean }) {
  const { t } = useTranslation();
  const [cif, setCif] = useState("");
  const [customer, setCustomer] = useState<Customer | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [form, setForm] = useState<CustomerCodesForm>(customerCodesFrom(null));
  const [saving, setSaving] = useState(false);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);

  const load = useCallback(async () => {
    const term = cif.trim();
    if (!term) {
      setError(t.ojkCodes.requiredCif);
      return;
    }
    setLoading(true);
    setError(null);
    setFeedback(null);
    try {
      const params = new URLSearchParams({
        q: term,
        page: "1",
        page_size: "5",
      });
      const response = await request<Customer[]>(
        `/customers?${params.toString()}`,
      );
      const found = (response.data ?? [])[0];
      if (!found) {
        setCustomer(null);
        setError(t.ojkCodes.cifNotFound);
        return;
      }
      setCustomer(found);
      setForm(customerCodesFrom(found));
    } catch (err) {
      setCustomer(null);
      setError(
        err instanceof ApiError ? err.message : t.ojkCodes.cifSearchError,
      );
    } finally {
      setLoading(false);
    }
  }, [cif, t]);

  const onSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!customer) return;
    setSaving(true);
    setSaveError(null);
    setFieldError(null);
    setFeedback(null);
    try {
      await request<Customer>(`/customers/${customer.id}`, {
        method: "PUT",
        body: {
          full_name: customer.full_name,
          id_card_number: customer.id_card_number,
          email: customer.email,
          phone_number: customer.phone_number,
          address: customer.address,
          metadata: customer.metadata ?? {},
          ojk_pihak_lawan_code: form.pihakLawan,
          ojk_sektor_ekonomi_code: form.sektorEkonomi,
          ojk_hubungan_bank_code: form.hubunganBank,
        },
      });
      setFeedback(t.ojkCodes.savedCustomer);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setSaveError(t.ojkCodes.saveForbidden);
      } else if (err instanceof ApiError && err.status === 422) {
        setFieldError(err.message);
      } else {
        setSaveError(
          err instanceof ApiError ? err.message : t.ojkCodes.saveError,
        );
      }
    } finally {
      setSaving(false);
    }
  };

  const reset = () => {
    setCif("");
    setCustomer(null);
    setForm(customerCodesFrom(null));
    setError(null);
    setFeedback(null);
    setFieldError(null);
    setSaveError(null);
  };

  return (
    <section>
      <h4 className="mb-1 text-title font-semibold text-ink-900">
        {t.ojkCodes.customerTitle}
      </h4>
      <p className="text-body text-ink-600">{t.ojkCodes.customerHint}</p>

      <div className="mt-3 flex items-end gap-2">
        <div className="w-72">
          <Input
            label={t.ojkCodes.cifLabel}
            helperText={t.ojkCodes.cifHint}
            placeholder={t.ojkCodes.cifPlaceholder}
            value={cif}
            onChange={(event) => setCif(event.target.value)}
          />
        </div>
        <Button variant="secondary" onClick={load} loading={loading}>
          {loading ? t.ojkCodes.cifLoading : t.ojkCodes.cifLoad}
        </Button>
        {customer && (
          <Button variant="ghost" onClick={reset}>
            {t.ojkCodes.changeCustomer}
          </Button>
        )}
      </div>

      {error && <Alert variant="error">{error}</Alert>}

      {customer && (
        <form onSubmit={onSubmit} className="mt-4 space-y-4">
          <p className="text-body text-ink-900">
            {t.ojkCodes.customerSelected}:{" "}
            <span className="font-mono">{customer.cif_number}</span>{" "}
            {customer.full_name}
          </p>
          <div className="grid gap-4 md:grid-cols-2">
            <Input
              label={t.ojkCodes.pihakLawanLabel}
              helperText={t.ojkCodes.pihakLawanHint}
              isMono
              value={form.pihakLawan}
              disabled={!canEdit}
              onChange={(event) =>
                setForm((prev) => ({ ...prev, pihakLawan: event.target.value }))
              }
            />
            <Input
              label={t.ojkCodes.sektorEkonomiLabel}
              helperText={t.ojkCodes.sektorEkonomiHint}
              isMono
              value={form.sektorEkonomi}
              disabled={!canEdit}
              onChange={(event) =>
                setForm((prev) => ({
                  ...prev,
                  sektorEkonomi: event.target.value,
                }))
              }
            />
            <Select
              label={t.ojkCodes.hubunganBankLabel}
              placeholder={t.ojkCodes.inlineEmptyOption}
              value={form.hubunganBank}
              disabled={!canEdit}
              options={HUBUNGAN_BANK_CODES.map((code) => ({
                value: code,
                label: code,
              }))}
              onChange={(event) =>
                setForm((prev) => ({
                  ...prev,
                  hubunganBank: event.target.value,
                }))
              }
            />
          </div>

          {fieldError && <Alert variant="error">{fieldError}</Alert>}
          {feedback && <Alert variant="success">{feedback}</Alert>}
          {saveError && <Alert variant="error">{saveError}</Alert>}

          {canEdit ? (
            <div className="flex justify-end">
              <Button type="submit" loading={saving}>
                {saving ? t.ojkCodes.saving : t.ojkCodes.save}
              </Button>
            </div>
          ) : (
            <Alert variant="info">{t.ojkCodes.readOnly}</Alert>
          )}
        </form>
      )}
    </section>
  );
}

interface LoanCodesForm {
  jenisPenggunaan: string;
  periodePembayaran: string;
  kabupaten: string;
  kelompokKredit: string;
  sumberDana: string;
  kategoriUsaha: string;
  sifatKredit: string;
  penjamin: string;
  penjaminBagianPct: string;
  tanggalMulaiMacet: string;
  agunanPpkaAmount: string;
  kelonggaranTarikAmount: string;
  provisiBelumDiamortisasiAmount: string;
  biayaTransaksiBelumDiamortisasiAmount: string;
  pendapatanBungaDitangguhkanAmount: string;
  cadanganKerugianRestrukturisasiAmount: string;
  klasifikasiAset: string;
}

const EMPTY_LOAN_CODES: LoanCodesForm = {
  jenisPenggunaan: "",
  periodePembayaran: "",
  kabupaten: "",
  kelompokKredit: "",
  sumberDana: "",
  kategoriUsaha: "",
  sifatKredit: "",
  penjamin: "",
  penjaminBagianPct: "",
  tanggalMulaiMacet: "",
  agunanPpkaAmount: "",
  kelonggaranTarikAmount: "",
  provisiBelumDiamortisasiAmount: "",
  biayaTransaksiBelumDiamortisasiAmount: "",
  pendapatanBungaDitangguhkanAmount: "",
  cadanganKerugianRestrukturisasiAmount: "",
  klasifikasiAset: "",
};

/** Nilai API bisa string desimal, angka, atau null; formulir selalu memakai string. */
function textValue(value: string | number | null | undefined): string {
  if (value === null || value === undefined) return "";
  return String(value);
}

function loanCodesFrom(loan: Loan | null): LoanCodesForm {
  if (!loan) return { ...EMPTY_LOAN_CODES };
  return {
    jenisPenggunaan: loan.ojk_jenis_penggunaan_code ?? "",
    periodePembayaran: loan.ojk_periode_pembayaran_code ?? "",
    kabupaten: loan.ojk_kabupaten_code ?? "",
    kelompokKredit: loan.ojk_kelompok_kredit_code ?? "",
    sumberDana: loan.ojk_sumber_dana_code ?? "",
    kategoriUsaha: loan.ojk_kategori_usaha_code ?? "",
    sifatKredit: loan.ojk_sifat_kredit_code ?? "",
    penjamin: loan.ojk_penjamin_code ?? "",
    penjaminBagianPct: textValue(loan.ojk_penjamin_bagian_pct),
    tanggalMulaiMacet: formatDateISO(loan.ojk_tanggal_mulai_macet),
    agunanPpkaAmount: textValue(loan.ojk_agunan_ppka_amount),
    kelonggaranTarikAmount: textValue(loan.ojk_kelonggaran_tarik_amount),
    provisiBelumDiamortisasiAmount: textValue(
      loan.ojk_provisi_belum_diamortisasi_amount,
    ),
    biayaTransaksiBelumDiamortisasiAmount: textValue(
      loan.ojk_biaya_transaksi_belum_diamortisasi_amount,
    ),
    pendapatanBungaDitangguhkanAmount: textValue(
      loan.ojk_pendapatan_bunga_ditangguhkan_amount,
    ),
    cadanganKerugianRestrukturisasiAmount: textValue(
      loan.ojk_cadangan_kerugian_restrukturisasi_amount,
    ),
    klasifikasiAset: loan.ojk_klasifikasi_aset_code ?? "",
  };
}

/** Membandingkan dua nilai nominal/persentase sebagai angka; string kosong = tidak diisi. */
function sameNumber(a: string, b: string): boolean {
  if (a === "" || b === "") return a === b;
  return Number(a) === Number(b);
}

/**
 * Sandi OJK kredit (Form 06.00) lewat endpoint khusus PUT /ojk/loan-codes/{loanId}.
 * Kredit dipilih dari halaman pertama daftar kredit, nilai tersimpan ditampilkan apa
 * adanya, dan hanya bidang yang benar-benar berubah yang dikirim agar nilai lain tidak
 * tertimpa menjadi kosong.
 */
function LoanCodesSection({ canEdit }: { canEdit: boolean }) {
  const { t } = useTranslation();
  const [loans, setLoans] = useState<Loan[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [selected, setSelected] = useState<Loan | null>(null);
  const [form, setForm] = useState<LoanCodesForm>(loanCodesFrom(null));
  const [saving, setSaving] = useState(false);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);

  const loadLoans = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    try {
      const response = await request<Loan[]>("/loans?page=1&page_size=100");
      setLoans(response.data ?? []);
    } catch (err) {
      setLoadError(
        err instanceof ApiError ? err.message : t.ojkCodes.loanSelectError,
      );
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    loadLoans();
  }, [loadLoans]);

  const selectLoan = (id: string) => {
    const loan = loans.find((item) => item.id === id) ?? null;
    setSelected(loan);
    setForm(loanCodesFrom(loan));
    setFeedback(null);
    setFieldError(null);
    setSaveError(null);
  };

  const inlineOptions = (codes: string[]) =>
    codes.map((code) => ({ value: code, label: code }));

  const sumberDanaOptions = [
    { value: "10", label: t.ojkCodes.sumberDanaGajiHonor },
    { value: "21", label: t.ojkCodes.sumberDanaUsahaSubsidi },
    { value: "22", label: t.ojkCodes.sumberDanaUsahaNonsubsidi },
    { value: "31", label: t.ojkCodes.sumberDanaLainSubsidi },
    { value: "32", label: t.ojkCodes.sumberDanaLainNonsubsidi },
  ];
  const kategoriUsahaOptions = [
    { value: "1", label: t.ojkCodes.kategoriUsahaMikro },
    { value: "2", label: t.ojkCodes.kategoriUsahaKecil },
    { value: "3", label: t.ojkCodes.kategoriUsahaMenengah },
    { value: "4", label: t.ojkCodes.kategoriUsahaLainnya },
  ];
  const sifatKreditOptions = [
    { value: "2", label: t.ojkCodes.sifatKreditPengalihan },
    { value: "9", label: t.ojkCodes.sifatKreditLainnya },
  ];

  const update = (field: keyof LoanCodesForm, value: string) =>
    setForm((prev) => ({ ...prev, [field]: value }));

  const buildBody = (): Record<string, string> => {
    const before = loanCodesFrom(selected);
    const body: Record<string, string> = {};
    const setText = (
      field: string,
      current: string,
      original: string,
    ): void => {
      if (current !== original) body[field] = current;
    };
    const setAmount = (
      field: string,
      current: string,
      original: string,
    ): void => {
      if (sameNumber(current, original)) return;
      body[field] = current === "" ? "" : String(Number(current));
    };

    setText(
      "ojk_jenis_penggunaan_code",
      form.jenisPenggunaan,
      before.jenisPenggunaan,
    );
    setText(
      "ojk_periode_pembayaran_code",
      form.periodePembayaran,
      before.periodePembayaran,
    );
    setText("ojk_kabupaten_code", form.kabupaten, before.kabupaten);
    setText(
      "ojk_kelompok_kredit_code",
      form.kelompokKredit,
      before.kelompokKredit,
    );
    setText("ojk_sumber_dana_code", form.sumberDana, before.sumberDana);
    setText(
      "ojk_kategori_usaha_code",
      form.kategoriUsaha,
      before.kategoriUsaha,
    );
    setText("ojk_sifat_kredit_code", form.sifatKredit, before.sifatKredit);
    setText("ojk_penjamin_code", form.penjamin, before.penjamin);
    setAmount(
      "ojk_penjamin_bagian_pct",
      form.penjaminBagianPct,
      before.penjaminBagianPct,
    );
    setText(
      "ojk_tanggal_mulai_macet",
      form.tanggalMulaiMacet,
      before.tanggalMulaiMacet,
    );
    setAmount(
      "ojk_agunan_ppka_amount",
      form.agunanPpkaAmount,
      before.agunanPpkaAmount,
    );
    setAmount(
      "ojk_kelonggaran_tarik_amount",
      form.kelonggaranTarikAmount,
      before.kelonggaranTarikAmount,
    );
    setAmount(
      "ojk_provisi_belum_diamortisasi_amount",
      form.provisiBelumDiamortisasiAmount,
      before.provisiBelumDiamortisasiAmount,
    );
    setAmount(
      "ojk_biaya_transaksi_belum_diamortisasi_amount",
      form.biayaTransaksiBelumDiamortisasiAmount,
      before.biayaTransaksiBelumDiamortisasiAmount,
    );
    setAmount(
      "ojk_pendapatan_bunga_ditangguhkan_amount",
      form.pendapatanBungaDitangguhkanAmount,
      before.pendapatanBungaDitangguhkanAmount,
    );
    setAmount(
      "ojk_cadangan_kerugian_restrukturisasi_amount",
      form.cadanganKerugianRestrukturisasiAmount,
      before.cadanganKerugianRestrukturisasiAmount,
    );
    setText(
      "ojk_klasifikasi_aset_code",
      form.klasifikasiAset,
      before.klasifikasiAset,
    );
    return body;
  };

  const onSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!selected) return;
    setFieldError(null);
    setSaveError(null);
    setFeedback(null);

    const body = buildBody();
    if (Object.keys(body).length === 0) {
      setFieldError(t.ojkCodes.noChanges);
      return;
    }

    setSaving(true);
    try {
      const response = await request<Loan>(`/ojk/loan-codes/${selected.id}`, {
        method: "PUT",
        body,
      });
      const updated = response.data ?? selected;
      setSelected(updated);
      setForm(loanCodesFrom(updated));
      setLoans((prev) =>
        prev.map((loan) => (loan.id === updated.id ? updated : loan)),
      );
      setFeedback(t.ojkCodes.savedLoan);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setSaveError(t.ojkCodes.saveForbidden);
      } else if (err instanceof ApiError && err.status === 422) {
        setFieldError(err.message);
      } else {
        setSaveError(
          err instanceof ApiError ? err.message : t.ojkCodes.saveError,
        );
      }
    } finally {
      setSaving(false);
    }
  };

  const amountValue = (raw: string) => (raw === "" ? "" : Number(raw));
  // Simpan digit mentah, bukan angka terurai: "0" yang diketik harus tetap "0"
  // dan tidak dianggap kosong.
  const amountChange = (field: keyof LoanCodesForm, raw: string) =>
    update(field, raw);

  return (
    <section>
      <h4 className="mb-1 text-title font-semibold text-ink-900">
        {t.ojkCodes.loanTitle}
      </h4>
      <p className="text-body text-ink-600">{t.ojkCodes.loanHint}</p>

      {loading ? (
        <div className="mt-3">
          <LoadingState label={t.ojkCodes.loanLoading} />
        </div>
      ) : loadError ? (
        <Alert variant="error">{loadError}</Alert>
      ) : loans.length === 0 ? (
        <Alert variant="info">{t.ojkCodes.loanEmpty}</Alert>
      ) : (
        <form onSubmit={onSubmit} className="mt-3 space-y-4">
          <div className="w-80">
            <Select
              label={t.ojkCodes.loanSelectLabel}
              placeholder={t.ojkCodes.loanSelectPlaceholder}
              value={selected?.id ?? ""}
              options={loans.map((loan) => ({
                value: loan.id,
                label: `${loan.loan_number} - ${loan.status}`,
              }))}
              onChange={(event) => selectLoan(event.target.value)}
            />
          </div>

          {selected && (
            <>
              <Alert variant="info">{t.ojkCodes.onlyChangedHint}</Alert>

              <div className="grid gap-4 md:grid-cols-2">
                <Select
                  label={t.ojkCodes.jenisPenggunaanLabel}
                  placeholder={t.ojkCodes.inlineEmptyOption}
                  value={form.jenisPenggunaan}
                  disabled={!canEdit}
                  options={inlineOptions(JENIS_PENGGUNAAN_CODES)}
                  onChange={(event) =>
                    update("jenisPenggunaan", event.target.value)
                  }
                />
                <Select
                  label={t.ojkCodes.periodePembayaranLabel}
                  placeholder={t.ojkCodes.inlineEmptyOption}
                  value={form.periodePembayaran}
                  disabled={!canEdit}
                  options={inlineOptions(PERIODE_PEMBAYARAN_CODES)}
                  onChange={(event) =>
                    update("periodePembayaran", event.target.value)
                  }
                />
                <Input
                  label={t.ojkCodes.kabupatenLabel}
                  helperText={t.ojkCodes.kabupatenHint}
                  isMono
                  value={form.kabupaten}
                  disabled={!canEdit}
                  onChange={(event) => update("kabupaten", event.target.value)}
                />
                <Input
                  label={t.ojkCodes.kelompokKreditLabel}
                  helperText={t.ojkCodes.kelompokKreditHint}
                  isMono
                  value={form.kelompokKredit}
                  disabled={!canEdit}
                  onChange={(event) =>
                    update("kelompokKredit", event.target.value)
                  }
                />
                <Select
                  label={t.ojkCodes.sumberDanaLabel}
                  placeholder={t.ojkCodes.inlineEmptyOption}
                  value={form.sumberDana}
                  disabled={!canEdit}
                  options={sumberDanaOptions}
                  onChange={(event) => update("sumberDana", event.target.value)}
                />
                <Select
                  label={t.ojkCodes.kategoriUsahaLabel}
                  placeholder={t.ojkCodes.inlineEmptyOption}
                  value={form.kategoriUsaha}
                  disabled={!canEdit}
                  options={kategoriUsahaOptions}
                  onChange={(event) =>
                    update("kategoriUsaha", event.target.value)
                  }
                />
                <Select
                  label={t.ojkCodes.sifatKreditLabel}
                  placeholder={t.ojkCodes.inlineEmptyOption}
                  value={form.sifatKredit}
                  disabled={!canEdit}
                  options={sifatKreditOptions}
                  onChange={(event) =>
                    update("sifatKredit", event.target.value)
                  }
                />
                <Input
                  label={t.ojkCodes.penjaminLabel}
                  helperText={t.ojkCodes.penjaminHint}
                  isMono
                  value={form.penjamin}
                  disabled={!canEdit}
                  onChange={(event) => update("penjamin", event.target.value)}
                />
                <Input
                  label={t.ojkCodes.penjaminPctLabel}
                  helperText={t.ojkCodes.penjaminPctHint}
                  type="number"
                  min="0"
                  max="100"
                  step="0.01"
                  value={form.penjaminBagianPct}
                  disabled={!canEdit}
                  onChange={(event) =>
                    update("penjaminBagianPct", event.target.value)
                  }
                />
                <DateInput
                  label={t.ojkCodes.tanggalMulaiMacetLabel}
                  helperText={t.ojkCodes.tanggalMulaiMacetHint}
                  value={form.tanggalMulaiMacet}
                  disabled={!canEdit}
                  onChange={(event) =>
                    update("tanggalMulaiMacet", event.target.value)
                  }
                />
                <Input
                  label={t.ojkCodes.klasifikasiAsetLabel}
                  helperText={t.ojkCodes.klasifikasiAsetHint}
                  isMono
                  value={form.klasifikasiAset}
                  disabled={!canEdit}
                  onChange={(event) =>
                    update("klasifikasiAset", event.target.value)
                  }
                />
              </div>

              <div className="grid gap-4 md:grid-cols-2">
                <CurrencyInput
                  label={t.ojkCodes.agunanPpkaLabel}
                  helperText={t.ojkCodes.agunanPpkaHint}
                  value={amountValue(form.agunanPpkaAmount)}
                  disabled={!canEdit}
                  onChange={(_value, raw) =>
                    amountChange("agunanPpkaAmount", raw)
                  }
                />
                <CurrencyInput
                  label={t.ojkCodes.kelonggaranTarikLabel}
                  helperText={t.ojkCodes.kelonggaranTarikHint}
                  value={amountValue(form.kelonggaranTarikAmount)}
                  disabled={!canEdit}
                  onChange={(_value, raw) =>
                    amountChange("kelonggaranTarikAmount", raw)
                  }
                />
                <CurrencyInput
                  label={t.ojkCodes.provisiBelumDiamortisasiLabel}
                  helperText={t.ojkCodes.provisiBelumDiamortisasiHint}
                  value={amountValue(form.provisiBelumDiamortisasiAmount)}
                  disabled={!canEdit}
                  onChange={(_value, raw) =>
                    amountChange("provisiBelumDiamortisasiAmount", raw)
                  }
                />
                <CurrencyInput
                  label={t.ojkCodes.biayaTransaksiBelumDiamortisasiLabel}
                  helperText={t.ojkCodes.biayaTransaksiBelumDiamortisasiHint}
                  value={amountValue(
                    form.biayaTransaksiBelumDiamortisasiAmount,
                  )}
                  disabled={!canEdit}
                  onChange={(_value, raw) =>
                    amountChange("biayaTransaksiBelumDiamortisasiAmount", raw)
                  }
                />
                <CurrencyInput
                  label={t.ojkCodes.pendapatanBungaDitangguhkanLabel}
                  helperText={t.ojkCodes.pendapatanBungaDitangguhkanHint}
                  value={amountValue(form.pendapatanBungaDitangguhkanAmount)}
                  disabled={!canEdit}
                  onChange={(_value, raw) =>
                    amountChange("pendapatanBungaDitangguhkanAmount", raw)
                  }
                />
                <CurrencyInput
                  label={t.ojkCodes.cadanganKerugianRestrukturisasiLabel}
                  helperText={t.ojkCodes.cadanganKerugianRestrukturisasiHint}
                  value={amountValue(
                    form.cadanganKerugianRestrukturisasiAmount,
                  )}
                  disabled={!canEdit}
                  onChange={(_value, raw) =>
                    amountChange("cadanganKerugianRestrukturisasiAmount", raw)
                  }
                />
              </div>

              {fieldError && <Alert variant="error">{fieldError}</Alert>}
              {feedback && <Alert variant="success">{feedback}</Alert>}
              {saveError && <Alert variant="error">{saveError}</Alert>}

              {canEdit ? (
                <div className="flex justify-end">
                  <Button type="submit" loading={saving}>
                    {saving ? t.ojkCodes.saving : t.ojkCodes.save}
                  </Button>
                </div>
              ) : (
                <Alert variant="info">{t.ojkCodes.readOnly}</Alert>
              )}
            </>
          )}
        </form>
      )}
    </section>
  );
}

/**
 * Kartu pengisian sandi referensi/inline OJK tanpa SQL: nasabah lewat
 * PUT /customers/{id} (customers:update) dan kredit lewat
 * PUT /ojk/loan-codes/{loanId} (system:config). Bagian yang tidak boleh dibaca
 * pengguna tidak dirender; penegakan izin tetap di API.
 */
export function OJKReferenceCodesCard() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const canSearchCustomer = hasPermission(user, "customers:read");
  const canEditCustomer = hasPermission(user, "customers:update");
  const canReadLoans = hasPermission(user, "loans:read");
  const canEditLoan = hasPermission(user, "system:config");

  return (
    <Card className="mb-4">
      <CardHeader>
        <CardTitle>{t.ojkCodes.title}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-6">
        <p className="text-body text-ink-600">{t.ojkCodes.description}</p>
        {canSearchCustomer && (
          <CustomerCodesSection canEdit={canEditCustomer} />
        )}
        {canReadLoans && <LoanCodesSection canEdit={canEditLoan} />}
      </CardContent>
    </Card>
  );
}
