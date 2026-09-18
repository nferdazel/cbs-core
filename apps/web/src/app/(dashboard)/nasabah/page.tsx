"use client";

import { useCallback, useEffect, useState } from "react";
import type { Customer } from "@cbs/shared-types";
import { ApiError, request } from "@/lib/api";
import { PageHeader } from "@/components/ui/PageHeader";
import { Card, CardContent } from "@/components/ui/Card";
import { DataTable, Column } from "@/components/ui/DataTable";
import { Pagination } from "@/components/ui/Pagination";
import { ErrorState } from "@/components/ui/States";

const PAGE_SIZE = 20;

interface Meta {
  page: number;
  page_size: number;
  total_items: number;
  total_pages: number;
}

export default function NasabahPage() {
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [meta, setMeta] = useState<Meta | null>(null);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async (targetPage: number) => {
    setLoading(true);
    setError(null);
    try {
      const response = await request<Customer[]>(
        `/customers?page=${targetPage}&page_size=${PAGE_SIZE}`
      );
      setCustomers(response.data ?? []);
      if (response.meta) setMeta(response.meta as Meta);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Gagal memuat nasabah.");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load(page);
  }, [page, load]);

  const columns: Column<Customer>[] = [
    { header: "CIF", accessorKey: "cif_number", isMono: true },
    { header: "Nama", accessorKey: "full_name" },
    { header: "Telepon", accessorKey: "phone_number", isMono: true },
    { header: "Email", accessorKey: "email" },
    { header: "Status", accessorKey: "status", type: "status" },
  ];

  return (
    <>
      <PageHeader
        title="Nasabah"
        description="Data induk nasabah (CIF). Nilai pribadi disimpan terenkripsi di backend."
      />
      {error && !loading ? (
        <ErrorState title="Gagal memuat nasabah" description={error} />
      ) : (
        <Card>
          <CardContent className="p-0">
            <DataTable
              columns={columns}
              data={customers}
              keyExtractor={(row) => row.id}
              loading={loading}
              emptyMessage="Belum ada nasabah."
              zebra
            />
            {meta && (
              <Pagination
                page={meta.page}
                pageSize={meta.page_size}
                totalItems={meta.total_items}
                totalPages={meta.total_pages}
                onPageChange={(next) => setPage(next)}
              />
            )}
          </CardContent>
        </Card>
      )}
    </>
  );
}
