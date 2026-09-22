"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, isCrossBranchError, request } from "@/lib/api";
import type { JournalEntryWithBranch } from "@/lib/operations-types";
import { formatDateTime } from "@/lib/format";
import { useTranslation } from "@/i18n/context";
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
  const { t } = useTranslation();
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
      setError(err instanceof ApiError ? err.message : t.ledger.loadError);
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    load(page);
  }, [page, load]);

  const columns: Column<JournalEntryWithBranch>[] = [
    {
      header: t.ledger.colReference,
      accessorKey: "reference_number",
      isMono: true,
    },
    {
      header: t.ledger.colPostedAt,
      cell: (row) => formatDateTime(row.posted_at),
      isMono: true,
    },
    {
      header: t.common.branch,
      cell: (row) => row.branch_code || "-",
      isMono: true,
    },
    { header: t.ledger.colType, accessorKey: "transaction_type" },
    { header: t.ledger.colDescription, accessorKey: "description" },
    { header: t.common.status, accessorKey: "status", type: "status" },
  ];

  return (
    <>
      <PageHeader
        title={t.ledger.title}
        description={t.ledger.description}
      />
      {error && !loading ? (
        <ErrorState
          title={forbidden ? t.ledger.forbiddenTitle : t.ledger.errorTitle}
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
              emptyMessage={t.ledger.empty}
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
