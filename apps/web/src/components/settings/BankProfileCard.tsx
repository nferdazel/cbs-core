"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, request } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import type { BankProfile } from "@/lib/types";
import { useTranslation } from "@/i18n/context";
import { Alert } from "@/components/ui/Alert";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { ErrorState, LoadingState } from "@/components/ui/States";

type FormState = {
  name: string;
  address: string;
  city: string;
  phone: string;
  npwp: string;
};

const EMPTY_FORM: FormState = {
  name: "",
  address: "",
  city: "",
  phone: "",
  npwp: "",
};

function toForm(profile: BankProfile | null): FormState {
  if (!profile) return EMPTY_FORM;
  return {
    name: profile.name ?? "",
    address: profile.address ?? "",
    city: profile.city ?? "",
    phone: profile.phone ?? "",
    npwp: profile.npwp ?? "",
  };
}

/**
 * Identitas bank tingkat instalasi. Bank mengisi nama/alamat di sini tanpa SQL;
 * nilainya dipakai dokumen cetak dan endpoint /app-info. Nama kosong ditandai
 * terang-terangan agar tidak tampak seperti bidang kosong tanpa penjelasan.
 */
export function BankProfileCard() {
  const { t } = useTranslation();
  const [profile, setProfile] = useState<BankProfile | null>(null);
  const [form, setForm] = useState<FormState>(EMPTY_FORM);
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [feedback, setFeedback] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    setForbidden(false);
    try {
      const response = await request<BankProfile>("/system/bank-profile");
      const data = response.data ?? null;
      setProfile(data);
      setForm(toForm(data));
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setForbidden(true);
      } else {
        setError(
          err instanceof ApiError ? err.message : t.bankProfile.loadError,
        );
      }
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    load();
  }, [load]);

  const update = (field: keyof FormState, value: string) => {
    setForm((prev) => ({ ...prev, [field]: value }));
  };

  const onSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    setFeedback(null);
    setSaveError(null);
    setFieldError(null);

    if (!form.name.trim()) {
      // Cek cepat di klien agar pengguna tidak menunggu; server tetap memvalidasi.
      setFieldError(t.bankProfile.requiredName);
      return;
    }

    setSaving(true);
    try {
      const response = await request<BankProfile>("/system/bank-profile", {
        method: "PUT",
        body: {
          name: form.name,
          address: form.address,
          city: form.city,
          phone: form.phone,
          npwp: form.npwp,
        },
      });
      const data = response.data ?? null;
      setProfile(data);
      setForm(toForm(data));
      setFeedback(t.bankProfile.saved);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setSaveError(t.bankProfile.saveForbidden);
      } else if (err instanceof ApiError && err.status === 422) {
        // Pesan validasi server sudah menyebut bidang yang harus diperbaiki.
        setFieldError(err.message);
      } else {
        setSaveError(
          err instanceof ApiError ? err.message : t.bankProfile.saveError,
        );
      }
    } finally {
      setSaving(false);
    }
  };

  const unconfigured = !profile?.name?.trim();

  return (
    <Card className="mb-4">
      <CardHeader>
        <CardTitle>{t.bankProfile.title}</CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <LoadingState label={t.states.loading} />
        ) : forbidden ? (
          <ErrorState
            title={t.bankProfile.title}
            description={t.bankProfile.forbidden}
          />
        ) : error ? (
          <ErrorState
            title={t.bankProfile.title}
            description={error}
            action={
              <Button variant="secondary" onClick={load}>
                {t.common.retry}
              </Button>
            }
          />
        ) : (
          <>
            <p className="text-body text-ink-600">
              {t.bankProfile.description}
            </p>

            {unconfigured && (
              <Alert variant="warning">
                {t.bankProfile.unconfiguredNotice}
              </Alert>
            )}

            <form onSubmit={onSubmit} className="space-y-4">
              <div className="grid gap-4 md:grid-cols-2">
                <div className="md:col-span-2">
                  <Input
                    label={t.bankProfile.name}
                    value={form.name}
                    onChange={(event) => update("name", event.target.value)}
                    placeholder={t.bankProfile.namePlaceholder}
                    helperText={t.bankProfile.nameHint}
                    error={fieldError ?? undefined}
                  />
                </div>
                <div className="md:col-span-2">
                  <Input
                    label={t.bankProfile.address}
                    value={form.address}
                    onChange={(event) => update("address", event.target.value)}
                    placeholder={t.bankProfile.addressPlaceholder}
                  />
                </div>
                <Input
                  label={t.bankProfile.city}
                  value={form.city}
                  onChange={(event) => update("city", event.target.value)}
                  placeholder={t.bankProfile.cityPlaceholder}
                />
                <Input
                  label={t.bankProfile.phone}
                  value={form.phone}
                  onChange={(event) => update("phone", event.target.value)}
                  placeholder={t.bankProfile.phonePlaceholder}
                />
                <Input
                  label={t.bankProfile.npwp}
                  value={form.npwp}
                  onChange={(event) => update("npwp", event.target.value)}
                  helperText={t.bankProfile.npwpHint}
                />
              </div>

              {feedback && <Alert variant="success">{feedback}</Alert>}
              {saveError && <Alert variant="error">{saveError}</Alert>}

              <div className="flex flex-wrap items-center justify-between gap-3">
                <p className="text-meta text-ink-600">
                  {t.bankProfile.lastUpdated}:{" "}
                  <span className="font-mono">
                    {profile?.updated_at
                      ? formatDateTime(profile.updated_at)
                      : t.bankProfile.neverUpdated}
                  </span>
                </p>
                <Button type="submit" loading={saving}>
                  {saving ? t.bankProfile.saving : t.bankProfile.save}
                </Button>
              </div>
            </form>
          </>
        )}
      </CardContent>
    </Card>
  );
}
