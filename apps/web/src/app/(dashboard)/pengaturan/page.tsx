"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, request } from "@/lib/api";
import { formatDate, formatDateTime } from "@/lib/format";
import type { SystemBusinessDate } from "@/lib/types";
import { useAuth } from "@/lib/useAuth";
import { PageHeader } from "@/components/ui/PageHeader";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { DefinitionList } from "@/components/ui/DefinitionList";
import { ErrorState, LoadingState } from "@/components/ui/States";
import { TransactionLimits } from "@/components/settings/TransactionLimits";
import { BankProfileCard } from "@/components/settings/BankProfileCard";

export default function PengaturanPage() {
  const { user } = useAuth();
  const [businessDate, setBusinessDate] = useState<SystemBusinessDate | null>(
    null
  );
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await request<SystemBusinessDate>(
        "/system/business-date"
      );
      setBusinessDate(response.data ?? null);
    } catch (err) {
      setError(
        err instanceof ApiError
          ? err.message
          : "Gagal memuat tanggal bisnis sistem."
      );
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  return (
    <>
      <PageHeader
        title="Pengaturan"
        description="Profil pengguna, tanggal bisnis sistem, dan batas transaksi."
      />

      <Card className="mb-4">
        <CardHeader>
          <CardTitle>Profil Pengguna</CardTitle>
        </CardHeader>
        <CardContent>
          <DefinitionList
            items={[
              { label: "Nama", value: user?.full_name || user?.username || "-" },
              { label: "Username", value: user?.username || "-", isMono: true },
              { label: "Role", value: user?.role || "-" },
              { label: "Cabang", value: user?.branch_code || "-", isMono: true },
            ]}
          />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Tanggal Bisnis</CardTitle>
        </CardHeader>
        <CardContent>
          {loading ? (
            <LoadingState label="Memuat tanggal bisnis..." />
          ) : error ? (
            <ErrorState
              title="Gagal memuat tanggal bisnis"
              description={error}
            />
          ) : businessDate ? (
            <DefinitionList
              items={[
                {
                  label: "Tanggal Berjalan",
                  value: formatDate(businessDate.current_date),
                  isMono: true,
                },
                { label: "Status", value: businessDate.status },
                {
                  label: "Terakhir Diperbarui",
                  value: formatDateTime(businessDate.updated_at),
                  isMono: true,
                },
                ...(businessDate.updated_by
                  ? [
                      {
                        label: "Diperbarui Oleh",
                        value: businessDate.updated_by,
                        isMono: true,
                      },
                    ]
                  : []),
              ]}
            />
          ) : (
            <p className="text-body text-ink-600">
              Tanggal bisnis belum tersedia.
            </p>
          )}
        </CardContent>
      </Card>

      <TransactionLimits />
      <BankProfileCard />
    </>
  );
}
