"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, isCrossBranchError, request } from "@/lib/api";
import type { JournalEntryWithBranch } from "@/lib/operations-types";
import { formatDateTime } from "@/lib/format";
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

export default function BukuBesarPage() {
  const [journals, setJournals] = useState<JournalEntryWithBranch[]>([]);
  const [meta, setMeta] = useState<Meta | null>(null);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [forbidden, setForbidden] = useState(false);

  const load = useCallback(async (targetPage: number) => {
    setLoading(true);
    setError(null);
    setForbidden(false);
    try {
      const response = await request<JournalEntryWithBranch[]>(
        `/transactions/journals?page=${targetPage}&page_size=${PAGE_SIZE}`
      );
      setJournals(response.data ?? []);
      if (response.meta) setMeta(response.meta as Meta);
    } catch (err) {
      setForbidden(isCrossBranchError(err));
      setError(err instanceof ApiError ? err.message : "Gagal memuat jurnal.");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load(page);
  }, [page, load]);

  const columns: Column<JournalEntryWithBranch>[] = [
    {
      header: "Nomor Referensi",
      accessorKey: "reference_number",
      isMono: true,
    },
    {
      header: "Waktu Posting",
      cell: (row) => formatDateTime(row.posted_at),
      isMono: true,
    },
    {
      header: "Cabang",
      cell: (row) => row.branch_code || "-",
      isMono: true,
    },
    { header: "Jenis", accessorKey: "transaction_type" },
    { header: "Keterangan", accessorKey: "description" },
    { header: "Status", accessorKey: "status", type: "status" },
  ];

  return (
    <>
      <PageHeader
        title="Buku Besar"
        description="Audit trail jurnal double-entry. Setiap baris harus seimbang debit dan kredit."
      />
      {error && !loading ? (
        <ErrorState
          title={
            forbidden ? "Akses lintas cabang ditolak" : "Gagal memuat jurnal"
          }
          description={error}
        />
      ) : (
        <Card>
          <CardContent className="p-0">
            <DataTable
              columns={columns}
              data={journals}
              keyExtractor={(row) => row.id}
              loading={loading}
              emptyMessage="Belum ada jurnal yang diposting."
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
