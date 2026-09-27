"use client";

import { useCallback, useEffect, useState } from "react";
import type { Customer } from "@cbs/shared-types";
import { ApiError, request } from "@/lib/api";
import type { Loan } from "@/lib/operations-types";
import { useAuth } from "@/lib/useAuth";
import { hasPermission } from "@/lib/permissions";
import { useTranslation } from "@/i18n/context";
import { Alert } from "@/components/ui/Alert";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
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
}

function loanCodesFrom(loan: Loan | null): LoanCodesForm {
  return {
    jenisPenggunaan: loan?.ojk_jenis_penggunaan_code ?? "",
    periodePembayaran: loan?.ojk_periode_pembayaran_code ?? "",
    kabupaten: loan?.ojk_kabupaten_code ?? "",
  };
}

/**
 * Sandi OJK kredit (Form 06.00 VIII/XI/XXII) lewat endpoint khusus
 * PUT /ojk/loan-codes/{loanId}. Kredit dipilih dari halaman pertama daftar kredit.
 */
function LoanCodesSection({ canEdit }: { canEdit: boolean }) {
  const { t } = useTranslation();
  const [loans, setLoans] = useState<Loan[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [selectedId, setSelectedId] = useState("");
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
    setSelectedId(id);
    setForm(loanCodesFrom(loans.find((loan) => loan.id === id) ?? null));
    setFeedback(null);
    setFieldError(null);
    setSaveError(null);
  };

  const onSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!selectedId) return;
    setSaving(true);
    setFieldError(null);
    setSaveError(null);
    setFeedback(null);
    try {
      await request<Loan>(`/ojk/loan-codes/${selectedId}`, {
        method: "PUT",
        body: {
          ojk_jenis_penggunaan_code: form.jenisPenggunaan,
          ojk_periode_pembayaran_code: form.periodePembayaran,
          ojk_kabupaten_code: form.kabupaten,
        },
      });
      setFeedback(t.ojkCodes.savedLoan);
      await loadLoans();
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
              value={selectedId}
              options={loans.map((loan) => ({
                value: loan.id,
                label: `${loan.loan_number} - ${loan.status}`,
              }))}
              onChange={(event) => selectLoan(event.target.value)}
            />
          </div>

          {selectedId && (
            <>
              <div className="grid gap-4 md:grid-cols-2">
                <Select
                  label={t.ojkCodes.jenisPenggunaanLabel}
                  placeholder={t.ojkCodes.inlineEmptyOption}
                  value={form.jenisPenggunaan}
                  disabled={!canEdit}
                  options={JENIS_PENGGUNAAN_CODES.map((code) => ({
                    value: code,
                    label: code,
                  }))}
                  onChange={(event) =>
                    setForm((prev) => ({
                      ...prev,
                      jenisPenggunaan: event.target.value,
                    }))
                  }
                />
                <Select
                  label={t.ojkCodes.periodePembayaranLabel}
                  placeholder={t.ojkCodes.inlineEmptyOption}
                  value={form.periodePembayaran}
                  disabled={!canEdit}
                  options={PERIODE_PEMBAYARAN_CODES.map((code) => ({
                    value: code,
                    label: code,
                  }))}
                  onChange={(event) =>
                    setForm((prev) => ({
                      ...prev,
                      periodePembayaran: event.target.value,
                    }))
                  }
                />
                <Input
                  label={t.ojkCodes.kabupatenLabel}
                  helperText={t.ojkCodes.kabupatenHint}
                  isMono
                  value={form.kabupaten}
                  disabled={!canEdit}
                  onChange={(event) =>
                    setForm((prev) => ({
                      ...prev,
                      kabupaten: event.target.value,
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
