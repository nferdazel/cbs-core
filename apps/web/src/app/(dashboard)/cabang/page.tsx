"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { ApiError, request } from "@/lib/api";
import { useAuth } from "@/lib/useAuth";
import { useTranslation } from "@/i18n/context";
import type { Branch, OrgUnitLevel } from "@/lib/operations-types";
import { PageHeader } from "@/components/ui/PageHeader";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Badge } from "@/components/ui/Badge";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { DataTable, Column } from "@/components/ui/DataTable";
import { ErrorState } from "@/components/ui/States";

// Urutan jenjang dipakai menyaring pilihan atasan: atasan wajib lebih tinggi.
const LEVEL_RANK: Record<OrgUnitLevel, number> = {
  CABANG: 1,
  AREA: 2,
  WILAYAH: 3,
};

const LEVEL_BADGE: Record<OrgUnitLevel, "neutral" | "accent" | "info"> = {
  CABANG: "neutral",
  AREA: "accent",
  WILAYAH: "info",
};

type FormState = {
  code: string;
  name: string;
  level: "" | OrgUnitLevel;
  parentCode: string;
  address: string;
  phone: string;
};

const EMPTY_FORM: FormState = {
  code: "",
  name: "",
  level: "",
  parentCode: "",
  address: "",
  phone: "",
};

export default function CabangPage() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const [units, setUnits] = useState<Branch[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);

  const [form, setForm] = useState<FormState>(EMPTY_FORM);
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<string | null>(null);

  // Mengubah susunan organisasi adalah fungsi administratif kantor pusat
  // (branches:create). Menyembunyikan form bukan batas keamanan: API tetap
  // menolak peran yang tidak berwenang.
  const canManage = user?.permissions?.includes("branches:create") ?? false;

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await request<Branch[]>("/branches/org-units");
      setUnits(response.data ?? []);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t.orgUnits.loadError);
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    load();
  }, [load, reloadKey]);

  const parentOptions = useMemo(() => {
    if (form.level === "") return [];
    return units
      .filter((unit) => LEVEL_RANK[unit.unit_level] > LEVEL_RANK[form.level as OrgUnitLevel])
      .map((unit) => ({ value: unit.code, label: `${unit.code} - ${unit.name}` }));
  }, [units, form.level]);

  const parentName = useMemo(() => {
    const byId = new Map(units.map((unit) => [unit.id, unit]));
    return (unit: Branch) => {
      if (!unit.parent_id) return null;
      const parent = byId.get(unit.parent_id);
      return parent ? `${parent.code} - ${parent.name}` : null;
    };
  }, [units]);

  const levelLabel = (level: OrgUnitLevel) => {
    switch (level) {
      case "AREA":
        return t.orgUnits.levelArea;
      case "WILAYAH":
        return t.orgUnits.levelRegion;
      default:
        return t.orgUnits.levelBranch;
    }
  };

  const update = (field: keyof FormState, value: string) => {
    setForm((prev) => {
      // Atasan yang tidak lagi valid (jenjang lebih rendah/setingkat) direset saat
      // jenjang berubah agar pilihan tidak menyesatkan.
      if (field === "level") {
        const next = value as FormState["level"];
        const stillValid =
          prev.parentCode === "" ||
          (next !== "" &&
            units.some(
              (unit) =>
                unit.code === prev.parentCode &&
                LEVEL_RANK[unit.unit_level] > LEVEL_RANK[next as OrgUnitLevel]
            ));
        return { ...prev, level: next, parentCode: stillValid ? prev.parentCode : "" };
      }
      return { ...prev, [field]: value };
    });
  };

  const onSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    setFeedback(null);
    setFormError(null);

    if (!form.code.trim() || !form.name.trim() || form.level === "") {
      setFormError(t.orgUnits.requiredFields);
      return;
    }

    setSaving(true);
    try {
      await request<Branch>("/branches/org-units", {
        method: "POST",
        body: {
          code: form.code,
          name: form.name,
          level: form.level,
          parent_code: form.parentCode,
          address: form.address,
          phone: form.phone,
        },
      });
      setForm(EMPTY_FORM);
      setFeedback(t.orgUnits.created);
      setReloadKey((key) => key + 1);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setFormError(t.orgUnits.forbidden);
      } else {
        setFormError(err instanceof ApiError ? err.message : t.orgUnits.createError);
      }
    } finally {
      setSaving(false);
    }
  };

  const columns: Column<Branch>[] = [
    { header: t.orgUnits.colCode, accessorKey: "code", isMono: true },
    { header: t.orgUnits.colName, accessorKey: "name" },
    {
      header: t.orgUnits.colLevel,
      cell: (row) => (
        <Badge variant={LEVEL_BADGE[row.unit_level]}>{levelLabel(row.unit_level)}</Badge>
      ),
    },
    {
      header: t.orgUnits.colParent,
      cell: (row) => {
        const parent = parentName(row);
        return parent ? <span className="font-mono text-meta">{parent}</span> : <span className="text-ink-600">{t.orgUnits.topLevel}</span>;
      },
    },
    {
      header: t.orgUnits.colStatus,
      cell: (row) =>
        row.is_head_office ? (
          <Badge variant="accent">{t.nav.cabang}</Badge>
        ) : row.is_active ? (
          <Badge variant="credit">{t.common.status}</Badge>
        ) : (
          <Badge variant="outline">{t.common.notAvailable}</Badge>
        ),
    },
  ];

  return (
    <>
      <PageHeader title={t.orgUnits.title} description={t.orgUnits.description} />

      {canManage && (
        <Card className="mb-4">
          <CardHeader>
            <CardTitle>{t.orgUnits.addTitle}</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="mb-3 text-meta text-ink-600">{t.orgUnits.addDescription}</p>
            <form onSubmit={onSubmit} className="space-y-3">
              <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
                <Input
                  label={t.orgUnits.code}
                  value={form.code}
                  onChange={(event) => update("code", event.target.value)}
                  placeholder={t.orgUnits.codePlaceholder}
                  helperText={t.orgUnits.codeHint}
                  isMono
                />
                <Input
                  label={t.orgUnits.name}
                  value={form.name}
                  onChange={(event) => update("name", event.target.value)}
                  placeholder={t.orgUnits.namePlaceholder}
                />
                <Select
                  label={t.orgUnits.level}
                  value={form.level}
                  onChange={(event) => update("level", event.target.value)}
                  placeholder={t.orgUnits.levelPlaceholder}
                  options={[
                    { value: "CABANG", label: t.orgUnits.levelBranch },
                    { value: "AREA", label: t.orgUnits.levelArea },
                    { value: "WILAYAH", label: t.orgUnits.levelRegion },
                  ]}
                />
              </div>
              <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
                <Select
                  label={t.orgUnits.parent}
                  value={form.parentCode}
                  onChange={(event) => update("parentCode", event.target.value)}
                  placeholder={t.orgUnits.parentPlaceholder}
                  options={parentOptions}
                  disabled={form.level === "" || parentOptions.length === 0}
                  helperText={form.level === "" ? t.orgUnits.levelPlaceholder : undefined}
                />
                <Input
                  label={t.orgUnits.address}
                  value={form.address}
                  onChange={(event) => update("address", event.target.value)}
                />
                <Input
                  label={t.orgUnits.phone}
                  value={form.phone}
                  onChange={(event) => update("phone", event.target.value)}
                  isMono
                />
              </div>
              <div className="flex items-center gap-3">
                <Button type="submit" loading={saving}>
                  {saving ? t.orgUnits.submitting : t.orgUnits.submit}
                </Button>
                {formError && <span className="text-meta text-debit-700">{formError}</span>}
                {feedback && <span className="text-meta text-credit-700">{feedback}</span>}
              </div>
            </form>
          </CardContent>
        </Card>
      )}

      {error && !loading ? (
        <ErrorState
          title={t.orgUnits.loadError}
          description={error}
          action={
            <Button variant="secondary" onClick={() => setReloadKey((key) => key + 1)}>
              {t.common.retry}
            </Button>
          }
        />
      ) : (
        <Card>
          <CardHeader>
            <CardTitle>{t.orgUnits.listTitle}</CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            <DataTable
              columns={columns}
              data={units}
              keyExtractor={(row) => row.id}
              loading={loading}
              emptyMessage={t.orgUnits.empty}
              zebra
            />
          </CardContent>
        </Card>
      )}
    </>
  );
}
