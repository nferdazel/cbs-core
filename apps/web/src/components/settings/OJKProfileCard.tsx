"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, request } from "@/lib/api";
import type { OJKProfile } from "@/lib/types";
import { useAuth } from "@/lib/useAuth";
import { hasPermission } from "@/lib/permissions";
import { useTranslation } from "@/i18n/context";
import { Alert } from "@/components/ui/Alert";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { DefinitionList } from "@/components/ui/DefinitionList";
import { ErrorState, LoadingState } from "@/components/ui/States";

const EMPTY_PROFILE: OJKProfile = {
  bank_email: "",
  bank_website: "",
  bank_city_code: "",
  bank_ojk_region_code: "",
  pic_name: "",
  pic_division: "",
  pic_phone: "",
  pic_email: "",
  dividends_paid: "",
  annual_bonus_tantiem: "",
  audit_info: "",
  share_nominal_value: "",
  public_offering_status: "",
  pva_status: "",
  ebanking_status: "",
  it_provider: "",
  laku_pandai_provider: "",
  laku_pandai_agent_count: "",
  rups_ownership_change: "",
  ultimate_shareholders: "",
};

/** Satu bidang formulir identitas OJK; label dan petunjuk berasal dari kamus. */
interface FieldSpec {
  key: keyof OJKProfile;
  label: string;
  hint?: string;
  isMono?: boolean;
}

function toProfile(data: OJKProfile | null): OJKProfile {
  if (!data) return EMPTY_PROFILE;
  return { ...EMPTY_PROFILE, ...data };
}

/**
 * Identitas Form 00.00 (kunci system_config ojk.*). Nilai kosong berarti BELUM
 * TERSEDIA dan ditampilkan apa adanya; UI tidak mengarang angka atau nilai.
 * Galat validasi dari API ditampilkan sebagai pesan server, bukan diterjemahkan
 * ulang agar tidak menyamarkan aturan yang sebenarnya.
 */
export function OJKProfileCard() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const canEdit = hasPermission(user, "system:config");

  const [profile, setProfile] = useState<OJKProfile>(EMPTY_PROFILE);
  const [form, setForm] = useState<OJKProfile>(EMPTY_PROFILE);
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
      const response = await request<OJKProfile>("/system/ojk-profile");
      const data = toProfile(response.data ?? null);
      setProfile(data);
      setForm(data);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setForbidden(true);
      } else {
        setError(
          err instanceof ApiError ? err.message : t.ojkProfile.loadError,
        );
      }
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    load();
  }, [load]);

  const update = (field: keyof OJKProfile, value: string) => {
    setForm((prev) => ({ ...prev, [field]: value }));
  };

  const onSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    setFeedback(null);
    setSaveError(null);
    setFieldError(null);
    setSaving(true);
    try {
      // Seluruh bidang dikirim, termasuk yang kosong, agar mengosongkan nilai
      // benar-benar menandainya "belum tersedia" di server.
      const response = await request<OJKProfile>("/system/ojk-profile", {
        method: "PUT",
        body: form,
      });
      const data = toProfile(response.data ?? null);
      setProfile(data);
      setForm(data);
      setFeedback(t.ojkProfile.saved);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setSaveError(t.ojkProfile.saveForbidden);
      } else if (err instanceof ApiError && err.status === 422) {
        // Pesan validasi server menyebut aturan yang dilanggar apa adanya.
        setFieldError(err.message);
      } else {
        setSaveError(
          err instanceof ApiError ? err.message : t.ojkProfile.saveError,
        );
      }
    } finally {
      setSaving(false);
    }
  };

  const groups: { title: string; fields: FieldSpec[] }[] = [
    {
      title: t.ojkProfile.sectionBank,
      fields: [
        {
          key: "bank_email",
          label: t.ojkProfile.bankEmail,
          hint: t.ojkProfile.bankEmailHint,
        },
        {
          key: "bank_website",
          label: t.ojkProfile.bankWebsite,
          hint: t.ojkProfile.bankWebsiteHint,
        },
        {
          key: "bank_city_code",
          label: t.ojkProfile.bankCityCode,
          isMono: true,
        },
        {
          key: "bank_ojk_region_code",
          label: t.ojkProfile.bankOjkRegionCode,
          isMono: true,
        },
      ],
    },
    {
      title: t.ojkProfile.sectionPic,
      fields: [
        { key: "pic_name", label: t.ojkProfile.picName },
        { key: "pic_division", label: t.ojkProfile.picDivision },
        {
          key: "pic_phone",
          label: t.ojkProfile.picPhone,
          hint: t.ojkProfile.picPhoneHint,
        },
        {
          key: "pic_email",
          label: t.ojkProfile.picEmail,
          hint: t.ojkProfile.picEmailHint,
        },
      ],
    },
    {
      title: t.ojkProfile.sectionOther,
      fields: [
        { key: "dividends_paid", label: t.ojkProfile.dividendsPaid },
        { key: "annual_bonus_tantiem", label: t.ojkProfile.annualBonusTantiem },
        { key: "audit_info", label: t.ojkProfile.auditInfo },
        {
          key: "share_nominal_value",
          label: t.ojkProfile.shareNominalValue,
          isMono: true,
        },
        {
          key: "public_offering_status",
          label: t.ojkProfile.publicOfferingStatus,
        },
        { key: "pva_status", label: t.ojkProfile.pvaStatus },
        { key: "ebanking_status", label: t.ojkProfile.ebankingStatus },
        { key: "it_provider", label: t.ojkProfile.itProvider },
        {
          key: "laku_pandai_provider",
          label: t.ojkProfile.lakuPandaiProvider,
        },
        {
          key: "laku_pandai_agent_count",
          label: t.ojkProfile.lakuPandaiAgentCount,
          hint: t.ojkProfile.lakuPandaiAgentCountHint,
          isMono: true,
        },
        {
          key: "rups_ownership_change",
          label: t.ojkProfile.rupsOwnershipChange,
        },
        {
          key: "ultimate_shareholders",
          label: t.ojkProfile.ultimateShareholders,
        },
      ],
    },
  ];

  const allFields = groups.flatMap((group) => group.fields);
  const emptyCount = allFields.filter(
    (field) => !profile[field.key]?.trim(),
  ).length;

  return (
    <Card className="mb-4">
      <CardHeader>
        <CardTitle>{t.ojkProfile.title}</CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <LoadingState label={t.states.loading} />
        ) : forbidden ? (
          <ErrorState
            title={t.ojkProfile.title}
            description={t.ojkProfile.forbidden}
          />
        ) : error ? (
          <ErrorState
            title={t.ojkProfile.title}
            description={error}
            action={
              <Button variant="secondary" onClick={load}>
                {t.common.retry}
              </Button>
            }
          />
        ) : (
          <>
            <p className="text-body text-ink-600">{t.ojkProfile.description}</p>

            {emptyCount > 0 && (
              <Alert variant="warning">
                {t.ojkProfile.emptyNotice} ({emptyCount}{" "}
                {t.ojkProfile.emptyFieldCount})
              </Alert>
            )}

            <section>
              <h4 className="text-title font-semibold text-ink-900">
                {t.ojkProfile.storedTitle}
              </h4>
              <div className="mt-3 space-y-4">
                {groups.map((group) => (
                  <div key={group.title}>
                    <h5 className="mb-2 text-meta font-medium uppercase tracking-wide text-ink-600">
                      {group.title}
                    </h5>
                    <DefinitionList
                      columns={2}
                      items={group.fields.map((field) => ({
                        label: field.label,
                        value:
                          profile[field.key]?.trim() || t.common.notAvailable,
                        isMono: field.isMono,
                      }))}
                    />
                  </div>
                ))}
              </div>
            </section>

            {canEdit ? (
              <form onSubmit={onSubmit} className="space-y-4">
                {groups.map((group) => (
                  <fieldset
                    key={group.title}
                    className="rounded-md border border-border p-4"
                  >
                    <legend className="px-1 text-meta font-medium uppercase tracking-wide text-ink-600">
                      {group.title}
                    </legend>
                    <div className="grid gap-4 md:grid-cols-2">
                      {group.fields.map((field) => (
                        <Input
                          key={field.key}
                          label={field.label}
                          helperText={field.hint}
                          isMono={field.isMono}
                          placeholder={t.common.notAvailable}
                          value={form[field.key]}
                          onChange={(event) =>
                            update(field.key, event.target.value)
                          }
                        />
                      ))}
                    </div>
                  </fieldset>
                ))}

                {fieldError && <Alert variant="error">{fieldError}</Alert>}
                {feedback && <Alert variant="success">{feedback}</Alert>}
                {saveError && <Alert variant="error">{saveError}</Alert>}

                <div className="flex justify-end">
                  <Button type="submit" loading={saving}>
                    {saving ? t.ojkProfile.saving : t.ojkProfile.save}
                  </Button>
                </div>
              </form>
            ) : (
              <Alert variant="info">{t.ojkProfile.readOnly}</Alert>
            )}
          </>
        )}
      </CardContent>
    </Card>
  );
}
