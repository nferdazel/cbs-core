"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, request } from "@/lib/api";
import type { Branch } from "@/lib/operations-types";
import { PageHeader } from "@/components/ui/PageHeader";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Badge } from "@/components/ui/Badge";
import { DataTable, Column } from "@/components/ui/DataTable";
import { ErrorState } from "@/components/ui/States";

export default function CabangPage() {
  const [branches, setBranches] = useState<Branch[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await request<Branch[]>("/branches");
      setBranches(response.data ?? []);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Gagal memuat cabang.");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load, reloadKey]);

  const columns: Column<Branch>[] = [
    { header: "Kode", accessorKey: "code", isMono: true },
    { header: "Nama", accessorKey: "name" },
    {
      header: "Alamat",
      cell: (row) => row.address || "-",
    },
    {
      header: "Telepon",
      isMono: true,
      cell: (row) => row.phone || "-",
    },
    {
      header: "Kantor Pusat",
      cell: (row) =>
        row.is_head_office ? (
          <Badge variant="accent">Kantor Pusat</Badge>
        ) : (
          <span className="text-ink-600">-</span>
        ),
    },
    {
      header: "Status",
      cell: (row) =>
        row.is_active ? (
          <Badge variant="credit">Aktif</Badge>
        ) : (
          <Badge variant="outline">Nonaktif</Badge>
        ),
    },
  ];

  return (
    <>
      <PageHeader
        title="Cabang"
        description="Master kantor cabang bank. Halaman ini hanya baca."
      />

      {error && !loading ? (
        <ErrorState
          title="Gagal memuat cabang"
          description={error}
          action={
            <Button variant="secondary" onClick={() => setReloadKey((key) => key + 1)}>
              Coba lagi
            </Button>
          }
        />
      ) : (
        <Card>
          <CardHeader>
            <CardTitle>Daftar Cabang</CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            <DataTable
              columns={columns}
              data={branches}
              keyExtractor={(row) => row.id}
              loading={loading}
              emptyMessage="Belum ada data cabang."
              zebra
            />
          </CardContent>
        </Card>
      )}
    </>
  );
}
