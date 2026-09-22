"use client";

import { useCallback, useEffect, useState } from "react";
import type { JournalLine } from "@cbs/shared-types";
import { ApiError, isCrossBranchError, request } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import type { AccountRecord } from "@/lib/types";
import { useTranslation } from "@/i18n/context";
import { Alert } from "@/components/ui/Alert";
import { PageHeader } from "@/components/ui/PageHeader";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { DataTable, Column } from "@/components/ui/DataTable";
import { MoneyText } from "@/components/ui/MoneyText";
import { Badge } from "@/components/ui/Badge";
import { StatusBadge } from "@/components/ui/StatusBadge";
import { DefinitionList } from "@/components/ui/DefinitionList";
import { Pagination } from "@/components/ui/Pagination";
import { ErrorState, EmptyState } from "@/components/ui/States";
import { AccountReactivation } from "@/components/account/AccountReactivation";

const PAGE_SIZE = 25;

interface Meta {
  page: number;
  page_size: number;
  total_items: number;
  total_pages: number;
}

export default function TransaksiPage() {
  const { t } = useTranslation();
  const [accountInput, setAccountInput] = useState("");
  const [account, setAccount] = useState<string | null>(null);
  const [lines, setLines] = useState<JournalLine[]>([]);
  const [meta, setMeta] = useState<Meta | null>(null);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [forbidden, setForbidden] = useState(false);
  const [accountInfo, setAccountInfo] = useState<AccountRecord | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);

  const load = useCallback(
    async (accNumber: string, targetPage: number) => {
      setLoading(true);
      setError(null);
      setForbidden(false);
      try {
        const response = await request<JournalLine[]>(
          `/accounts/${encodeURIComponent(accNumber)}/statements?page=${targetPage}&page_size=${PAGE_SIZE}`,
        );
        setLines(response.data ?? []);
        if (response.meta) setMeta(response.meta as Meta);
      } catch (err) {
        // Rekening cabang lain dibalas 403, bukan daftar mutasi kosong. Tampilkan
        // sebabnya agar tidak disalahartikan sebagai "tidak ada transaksi".
        setForbidden(isCrossBranchError(err));
        setError(
          err instanceof ApiError ? err.message : t.transactions.loadError,
        );
        setLines([]);
      } finally {
        setLoading(false);
      }
    },
    [t],
  );

  useEffect(() => {
    if (account) load(account, page);
  }, [account, page, load]);

  const loadAccount = useCallback(async (accNumber: string) => {
    setAccountInfo(null);
    try {
      const response = await request<AccountRecord>(
        `/accounts/${encodeURIComponent(accNumber)}`,
      );
      setAccountInfo(response.data ?? null);
    } catch {
      // Status rekening tidak wajib untuk membaca mutasi; panel rincian cukup
      // tidak ditampilkan bila endpoint detail menolak.
      setAccountInfo(null);
    }
  }, []);

  useEffect(() => {
    if (account) loadAccount(account);
  }, [account, loadAccount]);

  const handleSearch = (event: React.FormEvent) => {
    event.preventDefault();
    const trimmed = accountInput.trim();
    if (!trimmed) return;
    setPage(1);
    setSuccessMessage(null);
    setAccount(trimmed);
  };

  const handleReactivated = (updated: AccountRecord) => {
    setAccountInfo(updated);
    setSuccessMessage(
      `${t.transactions.reactivatedPrefix}${updated.account_number}${t.transactions.reactivatedSuffix}`,
    );
  };

  const columns: Column<JournalLine>[] = [
    {
      header: t.transactions.colTime,
      cell: (row) => formatDateTime(row.created_at),
      isMono: true,
    },
    {
      header: t.transactions.colDirection,
      cell: (row) => (
        <Badge variant={row.direction === "DEBIT" ? "debit" : "credit"}>
          {row.direction}
        </Badge>
      ),
    },
    {
      header: t.transactions.colAmount,
      type: "money",
      cell: (row) => (
        <MoneyText
          value={row.amount}
          tone={row.direction === "DEBIT" ? "debit" : "credit"}
        />
      ),
    },
    {
      header: t.transactions.colBalanceAfter,
      type: "money",
      cell: (row) => <MoneyText value={row.balance_after} />,
    },
    { header: t.transactions.colDescription, accessorKey: "description" },
    {
      header: t.transactions.colReference,
      accessorKey: "journal_entry_id",
      isMono: true,
    },
  ];

  return (
    <>
      <PageHeader
        title={t.transactions.title}
        description={t.transactions.description}
      />

      {successMessage && (
        <Alert variant="success" className="mb-4">
          {successMessage}
        </Alert>
      )}

      <Card className="mb-4">
        <CardContent>
          <form onSubmit={handleSearch} className="flex items-end gap-3">
            <div className="w-72">
              <Input
                label={t.transactions.accountNumber}
                value={accountInput}
                onChange={(e) => setAccountInput(e.target.value)}
                isMono
                placeholder={t.transactions.accountNumberPlaceholder}
              />
            </div>
            <Button type="submit" loading={loading && account !== null}>
              {t.transactions.showButton}
            </Button>
          </form>
        </CardContent>
      </Card>

      {accountInfo && (
        <Card className="mb-4">
          <CardHeader>
            <CardTitle>{t.transactions.detailsTitle}</CardTitle>
            <div className="flex items-center gap-3">
              <StatusBadge status={accountInfo.status} domain="account" />
              <AccountReactivation
                account={accountInfo}
                onReactivated={handleReactivated}
              />
            </div>
          </CardHeader>
          <CardContent>
            <DefinitionList
              items={[
                {
                  label: t.transactions.accountNumber,
                  value: accountInfo.account_number,
                  isMono: true,
                },
                {
                  label: t.transactions.owner,
                  value: accountInfo.customer_name || "-",
                },
                {
                  label: t.common.branch,
                  value: accountInfo.branch_code || "-",
                  isMono: true,
                },
                {
                  label: t.transactions.lastActivity,
                  value: accountInfo.last_activity_at
                    ? formatDateTime(accountInfo.last_activity_at)
                    : "-",
                  isMono: true,
                },
              ]}
            />
          </CardContent>
        </Card>
      )}

      {error && !loading ? (
        <ErrorState
          title={
            forbidden
              ? t.transactions.forbiddenTitle
              : t.transactions.errorTitle
          }
          description={error}
        />
      ) : account === null ? (
        <EmptyState
          title={t.transactions.emptyNoAccountTitle}
          description={t.transactions.emptyNoAccountDesc}
        />
      ) : (
        <Card>
          <CardContent className="p-0">
            <DataTable
              columns={columns}
              data={lines}
              keyExtractor={(row) => row.id}
              loading={loading}
              emptyMessage={t.transactions.empty}
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
