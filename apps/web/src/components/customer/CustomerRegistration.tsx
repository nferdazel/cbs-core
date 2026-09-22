"use client";

import { useState } from "react";
import type { Customer } from "@cbs/shared-types";
import { ApiError, request, unwrap } from "@/lib/api";
import { useTranslation } from "@/i18n/context";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";

/**
 * Tombol pendaftaran hanya tampil bila pengguna memegang izin customers:create
 * dari GET /auth/me (bukan salinan peran di web). Backend menegakkannya lewat
 * middleware RequirePermission; menyembunyikan tombol bukan batas keamanan.
 */

type FieldKey = "full_name" | "id_card_number" | "email";

type FieldErrors = Partial<Record<FieldKey, string>>;

/**
 * Email nasabah opsional. Pemeriksaan bentuk hanya dilakukan bila field diisi,
 * supaya form tidak memaksa teller melengkapi data yang belum ada.
 */
const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

/**
 * Memetakan pesan galat API ke field formulir. API mengirim satu pesan teks, bukan
 * galat per field, jadi pencocokan dilakukan dari kata kunci yang memang dipakai
 * service: "NIK sudah terdaftar", "email sudah terdaftar", "nama lengkap wajib diisi".
 */
function fieldForApiMessage(message: string): FieldKey | null {
  const lower = message.toLowerCase();
  if (lower.includes("nik")) return "id_card_number";
  if (lower.includes("email")) return "email";
  if (lower.includes("nama")) return "full_name";
  return null;
}

export interface CustomerRegistrationProps {
  /** Menutup formulir tanpa mendaftarkan nasabah. */
  onClose: () => void;
  /** Dipanggil setelah API membalas sukses, untuk menyegarkan daftar. */
  onRegistered: (customer: Customer) => void;
  /**
   * Bila diisi, panel sukses menampilkan aksi lanjut membuka rekening. Hanya
   * diberikan kepada pengguna yang memegang izin accounts:open.
   */
  onOpenAccount?: (customer: Customer) => void;
}

/**
 * Form pendaftaran nasabah. Field mengikuti domain.CreateCustomerInput; field
 * branch_code tidak dikirim karena service mengambil cabang dari JWT, bukan body.
 * Validasi klien mendahului API, galat API dipetakan kembali ke field bila cocok.
 */
export function CustomerRegistration({
  onClose,
  onRegistered,
  onOpenAccount,
}: CustomerRegistrationProps) {
  const { t } = useTranslation();
  const [fullName, setFullName] = useState("");
  const [idCardNumber, setIdCardNumber] = useState("");
  const [email, setEmail] = useState("");
  const [phoneNumber, setPhoneNumber] = useState("");
  const [address, setAddress] = useState("");
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [registered, setRegistered] = useState<Customer | null>(null);

  // Galat per field diumumkan lewat satu live region agar pembaca layar
  // mendengarnya walau pesan visualnya tersebar di bawah masing-masing input.
  const fieldAnnouncement = [
    fieldErrors.full_name,
    fieldErrors.id_card_number,
    fieldErrors.email,
  ]
    .filter(Boolean)
    .join(" ");

  const validate = (): FieldErrors => {
    const errors: FieldErrors = {};
    if (!fullName.trim()) errors.full_name = t.customerRegistration.requiredFullName;
    if (!idCardNumber.trim()) errors.id_card_number = t.customerRegistration.requiredIdCard;
    if (email.trim() && !EMAIL_PATTERN.test(email.trim())) {
      errors.email = t.customerRegistration.invalidEmail;
    }
    return errors;
  };

  const openConfirm = (event: React.FormEvent) => {
    event.preventDefault();
    const errors = validate();
    setFieldErrors(errors);
    if (Object.keys(errors).length > 0) {
      // Bisa berupa field wajib yang kosong atau format email yang salah.
      setFormError(t.customerRegistration.fillHighlighted);
      return;
    }
    setFormError(null);
    setConfirmOpen(true);
  };

  const submit = async () => {
    setSubmitting(true);
    setFormError(null);
    try {
      const response = await request<Customer>("/customers", {
        method: "POST",
        body: {
          full_name: fullName.trim(),
          id_card_number: idCardNumber.trim(),
          email: email.trim(),
          phone_number: phoneNumber.trim(),
          address: address.trim(),
        },
      });
      const customer = unwrap(response);
      setConfirmOpen(false);
      setRegistered(customer);
      onRegistered(customer);
    } catch (err) {
      const message =
        err instanceof ApiError ? err.message : t.customerRegistration.submitError;
      const field = fieldForApiMessage(message);
      setFieldErrors(field ? { [field]: message } : {});
      setFormError(field ? null : message);
      setConfirmOpen(false);
    } finally {
      setSubmitting(false);
    }
  };

  if (registered) {
    return (
      <Card className="mb-4">
        <CardHeader>
          <CardTitle>{t.customerRegistration.successTitle}</CardTitle>
          <Button variant="ghost" size="sm" onClick={onClose}>
            {t.customerRegistration.closeButton}
          </Button>
        </CardHeader>
        <CardContent>
          <div
            role="status"
            aria-live="polite"
            className="rounded-md border border-credit-700/30 bg-credit-50 px-4 py-3"
          >
            <p className="text-meta text-ink-600">
              {t.customerRegistration.successCifLabel}
            </p>
            <p className="font-mono text-page text-ink-900">
              {registered.cif_number}
            </p>
            <p className="mt-1 text-body text-ink-900">{registered.full_name}</p>
            <p className="mt-1 text-meta text-ink-600">
              {t.customerRegistration.successHint}
            </p>
          </div>
          <div className="flex gap-2">
            {onOpenAccount && (
              <Button onClick={() => onOpenAccount(registered)}>
                {t.customerRegistration.openAccountButton}
              </Button>
            )}
            <Button variant="secondary" onClick={onClose}>
              {t.customerRegistration.closeButton}
            </Button>
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className="mb-4">
      <CardHeader>
        <CardTitle>{t.customerRegistration.title}</CardTitle>
        <Button variant="ghost" size="sm" onClick={onClose}>
          {t.customerRegistration.closeButton}
        </Button>
      </CardHeader>
      <CardContent>
        <form onSubmit={openConfirm} className="space-y-4">
          <p className="sr-only" role="alert" aria-live="assertive">
            {fieldAnnouncement}
          </p>
          <p className="text-meta text-ink-600">
            {t.customerRegistration.description}
          </p>
          <div className="grid grid-cols-2 gap-4">
            <Input
              label={t.customerRegistration.fullName}
              value={fullName}
              onChange={(event) => setFullName(event.target.value)}
              error={fieldErrors.full_name}
              autoFocus
            />
            <Input
              label={t.customerRegistration.idCardNumber}
              value={idCardNumber}
              onChange={(event) => setIdCardNumber(event.target.value)}
              error={fieldErrors.id_card_number}
              helperText={t.customerRegistration.idCardHint}
              isMono
              inputMode="numeric"
            />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <Input
              label={`${t.customerRegistration.email} (${t.customerRegistration.optionalSuffix})`}
              type="email"
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              error={fieldErrors.email}
            />
            <Input
              label={`${t.customerRegistration.phoneNumber} (${t.customerRegistration.optionalSuffix})`}
              value={phoneNumber}
              onChange={(event) => setPhoneNumber(event.target.value)}
              isMono
            />
          </div>
          <Input
            label={`${t.customerRegistration.address} (${t.customerRegistration.optionalSuffix})`}
            value={address}
            onChange={(event) => setAddress(event.target.value)}
          />

          {formError && (
            <p className="text-meta text-debit-700" role="alert" aria-live="assertive">
              {formError}
            </p>
          )}

          <Button type="submit">{t.customerRegistration.submit}</Button>
        </form>
      </CardContent>

      <ConfirmDialog
        open={confirmOpen}
        title={t.customerRegistration.confirmTitle}
        loading={submitting}
        confirmLabel={t.customerRegistration.openButton}
        onCancel={() => setConfirmOpen(false)}
        onConfirm={submit}
        description={
          <>
            <p>{t.customerRegistration.confirmDescription}</p>
            <dl className="mt-2 space-y-1">
              <div className="flex justify-between gap-4">
                <dt className="text-ink-600">{t.customerRegistration.fullName}</dt>
                <dd className="text-right text-ink-900">{fullName.trim()}</dd>
              </div>
              <div className="flex justify-between gap-4">
                <dt className="text-ink-600">
                  {t.customerRegistration.idCardNumber}
                </dt>
                <dd className="font-mono text-ink-900">{idCardNumber.trim()}</dd>
              </div>
              <div className="flex justify-between gap-4">
                <dt className="text-ink-600">{t.customerRegistration.email}</dt>
                <dd className="text-right text-ink-900">{email.trim() || "-"}</dd>
              </div>
            </dl>
          </>
        }
      />
    </Card>
  );
}
