"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import type { Customer } from "@cbs/shared-types";
import { ApiError, newIdempotencyKey, request, unwrap } from "@/lib/api";
import { formatDate, formatRate } from "@/lib/format";
import type {
  AROInstruction,
  BankingProduct,
  Deposit,
  DepositPreview,
  DepositStatus,
  ProfitType,
} from "@/lib/operations-types";
import { useTranslation } from "@/i18n/context";
import type { Dictionary } from "@/i18n/dictionaries/id";
import { PageHeader } from "@/components/ui/PageHeader";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Badge } from "@/components/ui/Badge";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { CurrencyInput } from "@/components/ui/CurrencyInput";
import { DataTable, Column } from "@/components/ui/DataTable";
import { MoneyText } from "@/components/ui/MoneyText";
import { DefinitionList } from "@/components/ui/DefinitionList";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { Pagination } from "@/components/ui/Pagination";
import { ErrorState, LoadingState } from "@/components/ui/States";

const PAGE_SIZE = 20;

interface Meta {
  page: number;
  page_size: number;
  total_items: number;
  total_pages: number;
}

const DEPOSIT_STATUS: Record<
  DepositStatus,
  { variant: "neutral" | "accent" | "credit" | "debit" }
> = {
  PLACED: { variant: "credit" },
  MATURED: { variant: "accent" },
  CLOSED: { variant: "neutral" },
  BROKEN: { variant: "debit" },
};

function DepositStatusBadge({ status }: { status: DepositStatus }) {
  return <Badge variant={DEPOSIT_STATUS[status]?.variant ?? "outline"}>{status}</Badge>;
}

// Skema imbal hasil disimpan sebagai kunci teknis (MARGIN dll.); hanya label
// tampilannya yang diambil dari kamus.
function profitTypeLabel(t: Dictionary["deposits"], type: ProfitType): string {
  if (type === "MARGIN") return t.profitMargin;
  if (type === "BAGI_HASIL") return t.profitBagiHasil;
  return t.profitInterest;
}

// Instruksi ARO juga kunci teknis; labelnya dari kamus.
function aroLabel(t: Dictionary["deposits"], instruction: AROInstruction): string {
  if (instruction === "PRINCIPAL_AND_PROFIT") return t.aroPrincipalAndProfitShort;
  if (instruction === "PRINCIPAL") return t.aroPrincipalShort;
  return t.aroNone;
}

function customerLabel(
  customerId: string,
  names: Record<string, string>
): React.ReactNode {
  const name = names[customerId];
  if (name) return name;
  return <span className="font-mono">{customerId}</span>;
}

interface PlaceFormProps {
  products: BankingProduct[];
  customers: Customer[];
  onPlaced: (deposit: Deposit) => void;
  onClose: () => void;
}

function DepositPlaceForm({
  products,
  customers,
  onPlaced,
  onClose,
}: PlaceFormProps) {
  const { t } = useTranslation();
  const [customerId, setCustomerId] = useState("");
  const [productId, setProductId] = useState("");
  const [amount, setAmount] = useState(0);
  const [termMonths, setTermMonths] = useState(12);
  const [aro, setAro] = useState(false);
  const [instruction, setInstruction] = useState<AROInstruction>("PRINCIPAL");
  const [formError, setFormError] = useState<string | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [preview, setPreview] = useState<DepositPreview | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);

  const selectedProduct = products.find((p) => p.id === productId);

  const validate = (): string | null => {
    if (!customerId) return t.deposits.requiredCustomer;
    if (!productId) return t.deposits.requiredProduct;
    if (amount <= 0) return t.deposits.requiredAmount;
    if (termMonths <= 0) return t.deposits.requiredTerm;
    return null;
  };

  // Pratinjau dihitung backend lewat /deposits/preview, memakai fungsi yang sama
  // dengan akrual. Frontend sengaja TIDAK menghitung bunga/pajak sendiri agar angka
  // yang diperiksa teller identik dengan yang nanti diposting.
  const openConfirm = async (event: React.FormEvent) => {
    event.preventDefault();
    const error = validate();
    if (error) {
      setFormError(error);
      return;
    }
    setFormError(null);
    setPreviewLoading(true);
    try {
      const response = await request<DepositPreview>("/deposits/preview", {
        method: "POST",
        body: {
          customer_id: customerId,
          product_id: productId,
          placement_amount: String(amount),
          term_months: termMonths,
          currency: "IDR",
          aro,
          aro_instruction: aro ? instruction : "NONE",
        },
      });
      setPreview(unwrap(response));
      setConfirmOpen(true);
    } catch (err) {
      setPreview(null);
      setFormError(
        err instanceof ApiError ? err.message : t.deposits.previewError
      );
    } finally {
      setPreviewLoading(false);
    }
  };

  const submit = async () => {
    setSubmitting(true);
    setFormError(null);
    try {
      const idempotencyKey = newIdempotencyKey();
      const response = await request<Deposit>("/deposits/place", {
        method: "POST",
        idempotencyKey,
        body: {
          customer_id: customerId,
          product_id: productId,
          placement_amount: String(amount),
          term_months: termMonths,
          currency: "IDR",
          aro,
          aro_instruction: aro ? instruction : "NONE",
        },
      });
      onPlaced(unwrap(response));
      setConfirmOpen(false);
    } catch (err) {
      setFormError(
        err instanceof ApiError ? err.message : t.deposits.placeError
      );
      setConfirmOpen(false);
    } finally {
      setSubmitting(false);
    }
  };

  const customerOptions = customers.map((c) => ({
    value: c.id,
    label: `${c.cif_number} - ${c.full_name}`,
  }));
  const productOptions = products.map((p) => ({
    value: p.id,
    label: `${p.code} - ${p.name}`,
  }));

  return (
    <Card className="mb-4">
      <CardHeader>
        <CardTitle>{t.deposits.placeTitle}</CardTitle>
        <Button variant="ghost" size="sm" onClick={onClose}>
          {t.common.close}
        </Button>
      </CardHeader>
      <CardContent>
        <form onSubmit={openConfirm} className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <Select
              label={t.deposits.customer}
              value={customerId}
              onChange={(e) => setCustomerId(e.target.value)}
              options={customerOptions}
              placeholder={t.deposits.selectCustomer}
            />
            <Select
              label={t.deposits.product}
              value={productId}
              onChange={(e) => setProductId(e.target.value)}
              options={productOptions}
              placeholder={
                productOptions.length === 0
                  ? t.deposits.noActiveProducts
                  : t.deposits.selectProduct
              }
            />
          </div>

          {selectedProduct && (
            <p className="text-meta text-ink-600">
              {t.deposits.productRange}{" "}
              <MoneyText value={selectedProduct.min_amount} />{" "}
              {t.deposits.toRange}{" "}
              <MoneyText value={selectedProduct.max_amount} />,{" "}
              {t.deposits.termLabel}{" "}
              <span className="font-mono">
                {selectedProduct.min_term_months}-{selectedProduct.max_term_months}
              </span>{" "}
              {t.deposits.termSuffix}.
            </p>
          )}

          <div className="grid grid-cols-2 gap-4">
            <CurrencyInput label={t.deposits.principal} value={amount} onChange={setAmount} />
            <Input
              label={t.deposits.termMonths}
              type="number"
              min={1}
              value={termMonths}
              onChange={(e) => setTermMonths(Number(e.target.value))}
              isMono
            />
          </div>

          <div className="flex items-end gap-4">
            <label className="flex items-center gap-2 text-body text-ink-900">
              <input
                type="checkbox"
                checked={aro}
                onChange={(e) => setAro(e.target.checked)}
                className="h-4 w-4 rounded-sm border border-border-strong"
              />
              {t.deposits.aroCheckbox}
            </label>
            {aro && (
              <div className="w-64">
                <Select
                  label={t.deposits.aroInstruction}
                  value={instruction}
                  onChange={(e) =>
                    setInstruction(e.target.value as AROInstruction)
                  }
                  options={[
                    { value: "PRINCIPAL", label: t.deposits.aroPrincipal },
                    {
                      value: "PRINCIPAL_AND_PROFIT",
                      label: t.deposits.aroPrincipalProfit,
                    },
                  ]}
                />
              </div>
            )}
          </div>

          {formError && (
            <p className="text-meta text-debit-700" role="alert">
              {formError}
            </p>
          )}

          <Button type="submit" loading={previewLoading}>
            {previewLoading
              ? t.deposits.computingPreview
              : t.deposits.continueButton}
          </Button>
        </form>
      </CardContent>

      <ConfirmDialog
        open={confirmOpen}
        title={t.deposits.confirmPlaceTitle}
        loading={submitting}
        confirmLabel={t.deposits.confirmPlaceButton}
        onCancel={() => setConfirmOpen(false)}
        onConfirm={submit}
        description={
          preview ? (
            <>
              <p>
                {t.deposits.confirmPlaceDesc}
              </p>
              <dl className="mt-2 space-y-1">
                <div className="flex justify-between">
                  <dt className="text-ink-600">{t.deposits.labelProduct}</dt>
                  <dd>{selectedProduct?.name ?? "-"}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-ink-600">{t.deposits.labelAmount}</dt>
                  <dd>
                    <MoneyText value={preview.placement_amount} />
                  </dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-ink-600">{t.deposits.labelTerm}</dt>
                  <dd className="font-mono">
                    {preview.term_months} {t.deposits.termSuffix}
                  </dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-ink-600">{t.deposits.labelMaturity}</dt>
                  <dd className="font-mono">
                    {formatDate(preview.maturity_date)}
                  </dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-ink-600">
                    {preview.profit_type === "MARGIN"
                      ? t.deposits.profitMargin
                      : preview.profit_type === "BAGI_HASIL"
                        ? t.deposits.labelProfitShareOwner
                        : t.deposits.labelInterestPerYear}
                  </dt>
                  <dd className="font-mono">
                    {preview.profit_type === "BAGI_HASIL"
                      ? formatRate(String(Number(preview.profit_rate) * 100))
                      : formatRate(preview.profit_rate)}
                  </dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-ink-600">
                    {t.deposits.estimatedPrefix}{" "}
                    {profitTypeLabel(t.deposits, preview.profit_type)}
                  </dt>
                  <dd>
                    <MoneyText value={preview.estimated_profit} />
                  </dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-ink-600">{t.deposits.labelEstimatedTax}</dt>
                  <dd>
                    <MoneyText value={preview.estimated_tax} />
                  </dd>
                </div>
                <div className="flex justify-between font-medium">
                  <dt>{t.deposits.labelMaturityValue}</dt>
                  <dd>
                    <MoneyText value={preview.maturity_proceeds} />
                  </dd>
                </div>
              </dl>
            </>
          ) : (
            <p>{t.deposits.previewUnavailable}</p>
          )
        }
      />
    </Card>
  );
}

interface DetailPanelProps {
  depositId: string;
  products: BankingProduct[];
  customerNames: Record<string, string>;
  onClose: () => void;
  onChanged: () => void;
}

function DepositDetailPanel({
  depositId,
  products,
  customerNames,
  onClose,
  onChanged,
}: DetailPanelProps) {
  const { t } = useTranslation();
  const [deposit, setDeposit] = useState<Deposit | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);
  const [pending, setPending] = useState<"accrue" | "withdraw" | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<{
    title: string;
    reference: string;
    detail?: React.ReactNode;
  } | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await request<Deposit>(`/deposits/${depositId}`);
      setDeposit(unwrap(response));
    } catch (err) {
      setError(
        err instanceof ApiError ? err.message : t.deposits.detailLoadError
      );
    } finally {
      setLoading(false);
    }
  }, [depositId, t]);

  useEffect(() => {
    load();
  }, [load, reloadKey]);

  const product = products.find((p) => p.id === deposit?.product_id);
  const isActive =
    deposit?.status === "PLACED" || deposit?.status === "MATURED";
  const early =
    isActive &&
    deposit !== null &&
    new Date(deposit.maturity_date).getTime() > Date.now();

  const runAction = async () => {
    if (!pending || !deposit) return;
    setSubmitting(true);
    setActionError(null);
    try {
      const response = await request<Deposit>(
        `/deposits/${deposit.id}/${pending}`,
        { method: "POST", idempotencyKey: newIdempotencyKey() }
      );
      const updated = unwrap(response);
      if (pending === "withdraw") {
        setFeedback({
          title: t.deposits.withdrawnTitle,
          reference: updated.account_number,
          detail: (
            <DefinitionList
              columns={2}
              items={[
                {
                  label: t.deposits.labelDisbursement,
                  value: <MoneyText value={updated.maturity_proceeds} />,
                },
                {
                  label: t.deposits.labelPenalty,
                  value: <MoneyText value={updated.early_withdrawal_penalty} />,
                },
                {
                  label: t.deposits.labelTaxPaid,
                  value: <MoneyText value={updated.paid_tax} />,
                },
                { label: t.common.status, value: updated.status },
              ]}
            />
          ),
        });
      } else {
        setFeedback({
          title: t.deposits.accruedTitle,
          reference: updated.account_number,
          detail: (
            <DefinitionList
              columns={2}
              items={[
                {
                  label: t.deposits.labelAccruedProfit,
                  value: <MoneyText value={updated.accrued_profit} />,
                },
                {
                  label: t.deposits.labelAccruedTax,
                  value: <MoneyText value={updated.accrued_tax} />,
                },
              ]}
            />
          ),
        });
      }
      setPending(null);
      setReloadKey((key) => key + 1);
      onChanged();
    } catch (err) {
      setActionError(
        err instanceof ApiError ? err.message : t.deposits.actionFailed
      );
      setPending(null);
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) {
    return (
      <Card className="mt-4">
        <CardContent>
          <LoadingState label={t.deposits.detailLoading} />
        </CardContent>
      </Card>
    );
  }

  if (error || !deposit) {
    return (
      <div className="mt-4">
        <ErrorState
          title={t.deposits.detailErrorTitle}
          description={error ?? t.deposits.notFound}
          action={
            <Button variant="secondary" onClick={onClose}>
              {t.common.close}
            </Button>
          }
        />
      </div>
    );
  }

  const bagiHasil = deposit.profit_type === "BAGI_HASIL";

  return (
    <Card className="mt-4">
      <CardHeader>
        <CardTitle>
          {t.deposits.detailTitle}{" "}
          <span className="font-mono">{deposit.account_number}</span>
        </CardTitle>
        <Button variant="ghost" size="sm" onClick={onClose}>
          {t.common.close}
        </Button>
      </CardHeader>
      <CardContent>
        {feedback && (
          <div className="rounded-md border border-credit-700/30 bg-credit-50 px-4 py-3">
            <p className="text-title font-medium text-credit-700">
              {feedback.title}
            </p>
            <p className="mt-1 text-body text-ink-900">
              {t.deposits.referenceLabel}{" "}
              <span className="font-mono">{feedback.reference}</span>
            </p>
            {feedback.detail && <div className="mt-2">{feedback.detail}</div>}
          </div>
        )}

        {actionError && (
          <p className="text-body text-debit-700" role="alert">
            {actionError}
          </p>
        )}

        {early && (
          <div className="rounded-md border border-accent-600/30 bg-accent-50 px-4 py-3">
            <p className="text-body text-accent-600">
              {t.deposits.earlyPrefix}{" "}
              <span className="font-mono">{formatDate(deposit.maturity_date)}</span>
              . {t.deposits.earlyPenalty}
              {product
                ? ` ${formatRate(product.early_withdrawal_penalty_rate)} ${t.deposits.earlyPenaltyOfPrincipal}`
                : ""}
              .
            </p>
          </div>
        )}

        <DefinitionList
          items={[
            {
              label: t.deposits.customer,
              value: customerLabel(deposit.customer_id, customerNames),
            },
            {
              label: t.deposits.labelProduct,
              value: product ? `${product.code} - ${product.name}` : "-",
            },
            {
              label: t.common.status,
              value: <DepositStatusBadge status={deposit.status} />,
            },
            { label: t.deposits.principal, value: <MoneyText value={deposit.placement_amount} /> },
            {
              label: t.deposits.labelStartDate,
              value: formatDate(deposit.start_date),
              isMono: true,
            },
            {
              label: t.deposits.labelMaturity,
              value: formatDate(deposit.maturity_date),
              isMono: true,
            },
            {
              label: t.deposits.labelScheme,
              value: profitTypeLabel(t.deposits, deposit.profit_type),
            },
            bagiHasil
              ? {
                  label: t.deposits.labelProfitShareOwner,
                  value: formatRate(String(Number(deposit.profit_rate) * 100)),
                  isMono: true,
                }
              : {
                  label: t.deposits.labelInterestPerYear,
                  value: formatRate(deposit.profit_rate),
                  isMono: true,
                },
            ...(bagiHasil
              ? [
                  {
                    label: t.deposits.labelYieldProjection,
                    value: formatRate(deposit.yield_rate),
                    isMono: true,
                  },
                ]
              : []),
            {
              label: t.deposits.labelAccruedProfit,
              value: <MoneyText value={deposit.accrued_profit} />,
            },
            {
              label: t.deposits.labelAccruedTax,
              value: <MoneyText value={deposit.accrued_tax} />,
            },
            {
              label: t.deposits.labelTaxRate,
              value: formatRate(deposit.tax_rate),
              isMono: true,
            },
            ...(deposit.status === "CLOSED" || deposit.status === "BROKEN"
              ? [
                  {
                    label: t.deposits.labelDisbursement,
                    value: <MoneyText value={deposit.maturity_proceeds} />,
                  },
                  {
                    label: t.deposits.labelEarlyPenalty,
                    value: (
                      <MoneyText value={deposit.early_withdrawal_penalty} />
                    ),
                  },
                ]
              : []),
            {
              label: t.deposits.labelAro,
              value: deposit.aro ? aroLabel(t.deposits, deposit.aro_instruction) : t.deposits.notAro,
            },
          ]}
        />

        <div className="flex gap-2">
          {isActive && (
            <>
              <Button size="sm" onClick={() => setPending("accrue")}>
                {t.deposits.accrueButton}
              </Button>
              <Button
                size="sm"
                variant={early ? "danger" : "primary"}
                onClick={() => setPending("withdraw")}
              >
                {t.deposits.withdrawButton}
              </Button>
            </>
          )}
        </div>
      </CardContent>

      <ConfirmDialog
        open={pending !== null}
        title={
          pending === "withdraw"
            ? t.deposits.confirmWithdrawTitle
            : t.deposits.confirmAccrueTitle
        }
        destructive={pending === "withdraw" && early}
        loading={submitting}
        confirmLabel={
          pending === "withdraw"
            ? t.deposits.confirmWithdrawButton
            : t.deposits.confirmAccrueButton
        }
        onCancel={() => setPending(null)}
        onConfirm={runAction}
        description={
          pending === "withdraw" ? (
            <>
              <p>
                {t.deposits.withdrawDescPrefix}{" "}
                <span className="font-mono">{deposit.account_number}</span>{" "}
                {t.deposits.withdrawDescSuffix}
              </p>
              {early && (
                <p className="text-debit-700">
                  {t.deposits.earlyWithdrawNotice}
                </p>
              )}
            </>
          ) : (
            <p>
              {t.deposits.accrueDescPrefix}{" "}
              <span className="font-mono">{deposit.account_number}</span>
              .
            </p>
          )
        }
      />
    </Card>
  );
}

export default function DepositoPage() {
  const { t } = useTranslation();
  const [deposits, setDeposits] = useState<Deposit[]>([]);
  const [meta, setMeta] = useState<Meta | null>(null);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [listReloadKey, setListReloadKey] = useState(0);

  const [products, setProducts] = useState<BankingProduct[]>([]);
  const [customers, setCustomers] = useState<Customer[]>([]);
  const customerNames = useMemo(
    () => Object.fromEntries(customers.map((c) => [c.id, c.full_name])),
    [customers]
  );

  const [formOpen, setFormOpen] = useState(false);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<{ title: string; reference: string } | null>(
    null
  );

  const [aroOpen, setAroOpen] = useState(false);
  const [aroSubmitting, setAroSubmitting] = useState(false);
  const [aroError, setAroError] = useState<string | null>(null);

  const loadDeposits = useCallback(async (targetPage: number) => {
    setLoading(true);
    setError(null);
    try {
      const response = await request<Deposit[]>(
        `/deposits?page=${targetPage}&page_size=${PAGE_SIZE}`
      );
      setDeposits(response.data ?? []);
      if (response.meta) setMeta(response.meta as Meta);
    } catch (err) {
      setError(
        err instanceof ApiError ? err.message : t.deposits.listLoadError
      );
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    loadDeposits(page);
  }, [page, loadDeposits, listReloadKey]);

  useEffect(() => {
    let cancelled = false;
    request<BankingProduct[]>("/products")
      .then((res) => {
        if (!cancelled) setProducts(res.data ?? []);
      })
      .catch(() => undefined);
    request<Customer[]>("/customers?page=1&page_size=100")
      .then((res) => {
        if (!cancelled) setCustomers(res.data ?? []);
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, []);

  const depositProducts = useMemo(
    () =>
      products.filter((p) => p.family === "TIME_DEPOSIT" && p.is_active),
    [products]
  );

  const runAro = async () => {
    setAroSubmitting(true);
    setAroError(null);
    try {
      const response = await request<{ processed: number }>("/deposits/run-aro", {
        method: "POST",
        idempotencyKey: newIdempotencyKey(),
      });
      const processed = response.data?.processed ?? 0;
      setFeedback({
        title: t.deposits.aroRunTitle,
        reference: `${processed} ${t.deposits.contractsExtendedSuffix}`,
      });
      setAroOpen(false);
      setListReloadKey((key) => key + 1);
    } catch (err) {
      setAroError(
        err instanceof ApiError ? err.message : t.deposits.aroRunError
      );
      setAroOpen(false);
    } finally {
      setAroSubmitting(false);
    }
  };

  const columns: Column<Deposit>[] = [
    { header: t.deposits.colAccountNumber, accessorKey: "account_number", isMono: true },
    {
      header: t.deposits.customer,
      cell: (row) => customerLabel(row.customer_id, customerNames),
    },
    {
      header: t.deposits.principal,
      type: "money",
      cell: (row) => <MoneyText value={row.placement_amount} />,
    },
    {
      header: t.deposits.colScheme,
      cell: (row) => profitTypeLabel(t.deposits, row.profit_type),
    },
    {
      header: t.deposits.labelMaturity,
      cell: (row) => formatDate(row.maturity_date),
      isMono: true,
    },
    {
      header: t.deposits.labelAccruedProfit,
      type: "money",
      cell: (row) => <MoneyText value={row.accrued_profit} />,
    },
    {
      header: t.common.status,
      cell: (row) => <DepositStatusBadge status={row.status} />,
    },
    {
      header: t.common.actions,
      cell: (row) => (
        <Button
          size="sm"
          variant="secondary"
          onClick={() => setSelectedId(row.id)}
        >
          {t.deposits.detailButton}
        </Button>
      ),
    },
  ];

  return (
    <>
      <PageHeader
        title={t.deposits.title}
        description={t.deposits.description}
      />

      {feedback && (
        <div className="mb-4 rounded-md border border-credit-700/30 bg-credit-50 px-4 py-3">
          <p className="text-title font-medium text-credit-700">
            {feedback.title}
          </p>
          <p className="mt-1 text-body text-ink-900">
            {t.deposits.referenceLabel}{" "}
            <span className="font-mono">{feedback.reference}</span>
          </p>
        </div>
      )}

      {aroError && (
        <div
          className="mb-4 rounded-md border border-debit-700/30 bg-debit-50 px-4 py-3"
          role="alert"
        >
          <p className="text-body text-debit-700">{aroError}</p>
        </div>
      )}

      {formOpen && (
        <DepositPlaceForm
          products={depositProducts}
          customers={customers}
          onPlaced={(deposit) => {
            setFormOpen(false);
            setSelectedId(deposit.id);
            setPage(1);
            setListReloadKey((key) => key + 1);
          }}
          onClose={() => setFormOpen(false)}
        />
      )}

      {error && !loading ? (
        <ErrorState title={t.deposits.listErrorTitle} description={error} />
      ) : (
        <Card>
          <CardHeader>
            <CardTitle>{t.deposits.listTitle}</CardTitle>
            <div className="flex gap-2">
              <Button
                size="sm"
                variant="secondary"
                onClick={() => setAroOpen(true)}
              >
                {t.deposits.runAroButton}
              </Button>
              <Button size="sm" onClick={() => setFormOpen((open) => !open)}>
                {t.deposits.placeButton}
              </Button>
            </div>
          </CardHeader>
          <CardContent className="p-0">
            <DataTable
              columns={columns}
              data={deposits}
              keyExtractor={(row) => row.id}
              loading={loading}
              emptyMessage={t.deposits.empty}
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

      {selectedId && (
        <DepositDetailPanel
          depositId={selectedId}
          products={products}
          customerNames={customerNames}
          onClose={() => setSelectedId(null)}
          onChanged={() => setListReloadKey((key) => key + 1)}
        />
      )}

      <ConfirmDialog
        open={aroOpen}
        title={t.deposits.runAroButton}
        loading={aroSubmitting}
        confirmLabel={t.deposits.runAroButton}
        onCancel={() => setAroOpen(false)}
        onConfirm={runAro}
        description={
          <p>
            {t.deposits.aroConfirmDesc}
          </p>
        }
      />
    </>
  );
}
