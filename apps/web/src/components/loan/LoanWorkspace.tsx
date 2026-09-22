"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import type { Account, Customer } from "@cbs/shared-types";
import { ApiError, newIdempotencyKey, request, unwrap } from "@/lib/api";
import { formatDate, formatRate } from "@/lib/format";
import type {
  BankingProduct,
  COABook,
  Loan,
  LoanSchedule,
  OJKCollectibility,
  ProfitType,
} from "@/lib/operations-types";
import { useTranslation } from "@/i18n/context";
import type { Dictionary } from "@/i18n/dictionaries/id";
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
import { PrintButton } from "@/components/ui/PrintButton";

const PAGE_SIZE = 20;

interface Meta {
  page: number;
  page_size: number;
  total_items: number;
  total_pages: number;
}

// Kolektibilitas adalah kunci teknis (1_LANCAR dll.); label & warnanya dari sini,
// teksnya dari kamus.
const COLLECTIBILITY_VARIANT: Record<
  OJKCollectibility,
  "credit" | "accent" | "debit"
> = {
  "1_LANCAR": "credit",
  "2_DPK": "accent",
  "3_KURANG_LANCAR": "debit",
  "4_DIRAGUKAN": "debit",
  "5_MACET": "debit",
};

function collectibilityLabel(
  t: Dictionary["loans"],
  value: OJKCollectibility
): string {
  switch (value) {
    case "1_LANCAR":
      return t.collect1;
    case "2_DPK":
      return t.collect2;
    case "3_KURANG_LANCAR":
      return t.collect3;
    case "4_DIRAGUKAN":
      return t.collect4;
    case "5_MACET":
      return t.collect5;
    default:
      return value;
  }
}

function CollectibilityBadge({ value }: { value: OJKCollectibility }) {
  const { t } = useTranslation();
  const variant = COLLECTIBILITY_VARIANT[value];
  const label = collectibilityLabel(t.loans, value);
  if (!variant) return <Badge variant="outline">{label}</Badge>;
  return <Badge variant={variant}>{label}</Badge>;
}

// Skema imbal hasil produk adalah kunci teknis (MURABAHAH dll.); labelnya dari kamus.
function profitLabel(t: Dictionary["loans"], type: ProfitType): string {
  if (type === "MARGIN") return t.profitMargin;
  if (type === "BAGI_HASIL") return t.profitBagiHasil;
  return t.profitInterest;
}

function schemeLabel(
  t: Dictionary["loans"],
  product?: BankingProduct
): string {
  if (!product) return t.unknown;
  switch (product.profit_scheme) {
    case "MURABAHAH":
      return t.schemeMurabahah;
    case "MUDHARABAH":
      return t.schemeMudharabah;
    case "MUSYARAKAH":
      return t.schemeMusyarakah;
    case "IJARAH":
      return t.schemeIjarah;
    case "WADIAH":
      return t.schemeWadiah;
    default:
      return t.schemeInterest;
  }
}

function customerLabel(
  customerId: string,
  names: Record<string, string>
): React.ReactNode {
  const name = names[customerId];
  if (name) return name;
  return <span className="font-mono">{customerId}</span>;
}

function ActionFeedback({
  feedback,
}: {
  feedback: { title: string; reference: string; description?: string };
}) {
  const { t } = useTranslation();
  return (
    <div className="rounded-md border border-credit-700/30 bg-credit-50 px-4 py-3">
      <p className="text-title font-medium text-credit-700">{feedback.title}</p>
      <p className="mt-1 text-body text-ink-900">
        {t.loans.referenceLabel}{" "}
        <span className="font-mono">{feedback.reference}</span>
      </p>
      {feedback.description && (
        <p className="mt-0.5 text-meta text-ink-600">{feedback.description}</p>
      )}
    </div>
  );
}

interface ApplyFormProps {
  book: COABook;
  products: BankingProduct[];
  customers: Customer[];
  onApplied: (loan: Loan) => void;
  onClose: () => void;
}

function LoanApplyForm({
  book,
  products,
  customers,
  onApplied,
  onClose,
}: ApplyFormProps) {
  const { t } = useTranslation();
  const syariah = book === "SYARIAH";
  const [customerId, setCustomerId] = useState("");
  const [accountId, setAccountId] = useState("");
  const [productId, setProductId] = useState("");
  const [principal, setPrincipal] = useState(0);
  const [termMonths, setTermMonths] = useState(12);
  const [margin, setMargin] = useState(0);
  const [purpose, setPurpose] = useState("");

  const [accounts, setAccounts] = useState<Account[]>([]);
  const [accountsLoading, setAccountsLoading] = useState(false);
  const [accountsError, setAccountsError] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!customerId) {
      setAccounts([]);
      setAccountId("");
      return;
    }
    let cancelled = false;
    setAccountsLoading(true);
    setAccountsError(false);
    request<Account[]>("/accounts?page=1&page_size=100")
      .then((res) => {
        if (cancelled) return;
        setAccounts(
          (res.data ?? []).filter((acc) => acc.customer_id === customerId)
        );
        setAccountId("");
      })
      .catch(() => {
        if (cancelled) return;
        setAccounts([]);
        setAccountsError(true);
      })
      .finally(() => {
        if (!cancelled) setAccountsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [customerId]);

  const selectedProduct = products.find((p) => p.id === productId);

  const validate = (): string | null => {
    if (!customerId) return t.loans.requiredCustomer;
    if (!accountId) return t.loans.requiredAccount;
    if (!productId) return t.loans.requiredProduct;
    if (principal <= 0) return t.loans.requiredPrincipal;
    if (termMonths <= 0) return t.loans.requiredTerm;
    return null;
  };

  const openConfirm = (event: React.FormEvent) => {
    event.preventDefault();
    const error = validate();
    if (error) {
      setFormError(error);
      return;
    }
    setFormError(null);
    setConfirmOpen(true);
  };

  const submit = async () => {
    setSubmitting(true);
    setFormError(null);
    try {
      const idempotencyKey = newIdempotencyKey();
      const response = await request<Loan>("/loans/apply", {
        method: "POST",
        idempotencyKey,
        body: {
          customer_id: customerId,
          product_id: productId,
          disbursement_account_id: accountId,
          principal_amount: String(principal),
          term_months: termMonths,
          margin_amount: String(margin || 0),
          purpose: purpose.trim(),
        },
      });
      onApplied(unwrap(response));
      setConfirmOpen(false);
    } catch (err) {
      setFormError(
        err instanceof ApiError ? err.message : t.loans.applyError
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
  const accountOptions = accounts.map((a) => ({
    value: a.id,
    label: a.account_number,
  }));
  const productOptions = products.map((p) => ({
    value: p.id,
    label: `${p.code} - ${p.name}`,
  }));

  return (
    <Card className="mb-4">
      <CardHeader>
        <CardTitle>
          {syariah ? t.loans.applyTitleFinancing : t.loans.applyTitleLoan}
        </CardTitle>
        <Button variant="ghost" size="sm" onClick={onClose}>
          {t.common.close}
        </Button>
      </CardHeader>
      <CardContent>
        <form onSubmit={openConfirm} className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <Select
              label={t.loans.customer}
              value={customerId}
              onChange={(e) => setCustomerId(e.target.value)}
              options={customerOptions}
              placeholder={t.loans.selectCustomer}
            />
            <Select
              label={t.loans.disbursementAccount}
              value={accountId}
              onChange={(e) => setAccountId(e.target.value)}
              options={accountOptions}
              placeholder={
                !customerId
                  ? t.loans.selectCustomerFirst
                  : accountsLoading
                    ? t.loans.loadingAccounts
                    : accountsError
                      ? t.loans.accountsLoadError
                      : accountOptions.length === 0
                        ? t.loans.noAccounts
                        : t.loans.selectAccount
              }
              disabled={!customerId || accountsLoading}
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <Select
              label={t.loans.product}
              value={productId}
              onChange={(e) => setProductId(e.target.value)}
              options={productOptions}
              placeholder={
                productOptions.length === 0
                  ? t.loans.noActiveProducts
                  : t.loans.selectProduct
              }
            />
            <Input
              label={t.loans.termMonths}
              type="number"
              min={1}
              value={termMonths}
              onChange={(e) => setTermMonths(Number(e.target.value))}
              isMono
            />
          </div>

          {selectedProduct && (
            <p className="text-meta text-ink-600">
              {t.loans.productRange}{" "}
              <MoneyText value={selectedProduct.min_amount} />{" "}
              {t.loans.toRange}{" "}
              <MoneyText value={selectedProduct.max_amount} />,{" "}
              {t.loans.termLabel}{" "}
              <span className="font-mono">
                {selectedProduct.min_term_months}-{selectedProduct.max_term_months}
              </span>{" "}
              {t.loans.termSuffix}.
            </p>
          )}

          <div className="grid grid-cols-2 gap-4">
            <CurrencyInput
              label={t.loans.principal}
              value={principal}
              onChange={setPrincipal}
            />
            <CurrencyInput
              label={syariah ? t.loans.marginSyariah : t.loans.marginOptional}
              value={margin}
              onChange={setMargin}
            />
          </div>

          <Input
            label={t.loans.purpose}
            value={purpose}
            onChange={(e) => setPurpose(e.target.value)}
          />

          {formError && (
            <p className="text-meta text-debit-700" role="alert">
              {formError}
            </p>
          )}

          <Button type="submit">{t.loans.continueButton}</Button>
        </form>
      </CardContent>

      <ConfirmDialog
        open={confirmOpen}
        title={t.loans.confirmApplyTitle}
        loading={submitting}
        confirmLabel={t.loans.confirmApplyButton}
        onCancel={() => setConfirmOpen(false)}
        onConfirm={submit}
        description={
          <>
            <p>{t.loans.confirmApplyDesc}</p>
            <dl className="mt-2 space-y-1">
              <div className="flex justify-between">
                <dt className="text-ink-600">{t.loans.labelProduct}</dt>
                <dd>{selectedProduct?.name ?? "-"}</dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-ink-600">{t.loans.labelPrincipal}</dt>
                <dd>
                  <MoneyText value={principal} />
                </dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-ink-600">{t.loans.labelTerm}</dt>
                <dd className="font-mono">
                  {termMonths} {t.loans.termSuffix}
                </dd>
              </div>
            </dl>
          </>
        }
      />
    </Card>
  );
}

interface DetailPanelProps {
  loanId: string;
  book: COABook;
  products: BankingProduct[];
  customerNames: Record<string, string>;
  onClose: () => void;
  onChanged: () => void;
}

type ActionKind = "approve" | "reject" | "disburse" | "pay";

function actionLabel(t: Dictionary["loans"], kind: ActionKind): string {
  switch (kind) {
    case "approve":
      return t.actionApprove;
    case "reject":
      return t.actionReject;
    case "disburse":
      return t.actionDisburse;
    default:
      return t.actionPay;
  }
}

function LoanDetailPanel({
  loanId,
  book,
  products,
  customerNames,
  onClose,
  onChanged,
}: DetailPanelProps) {
  const { t } = useTranslation();
  const syariah = book === "SYARIAH";
  const [loan, setLoan] = useState<Loan | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);
  const [pending, setPending] = useState<ActionKind | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<{
    title: string;
    reference: string;
    description?: string;
  } | null>(null);
  const [installmentNo, setInstallmentNo] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await request<Loan>(`/loans/${loanId}`);
      setLoan(unwrap(response));
    } catch (err) {
      setError(
        err instanceof ApiError ? err.message : t.loans.detailLoadError
      );
    } finally {
      setLoading(false);
    }
  }, [loanId, t]);

  useEffect(() => {
    load();
  }, [load, reloadKey]);

  const productById = useMemo(
    () => Object.fromEntries(products.map((p) => [p.id, p])),
    [products]
  );
  const product = loan?.product_id ? productById[loan.product_id] : undefined;

  const unpaidSchedules = (loan?.schedules ?? []).filter(
    (s) => s.status !== "PAID"
  );

  const openAction = (kind: ActionKind) => {
    setActionError(null);
    if (kind === "pay" && !installmentNo && unpaidSchedules.length > 0) {
      setInstallmentNo(String(unpaidSchedules[0].installment_no));
    }
    setPending(kind);
  };

  const runAction = async () => {
    if (!pending || !loan) return;
    setSubmitting(true);
    setActionError(null);
    try {
      const idempotencyKey = newIdempotencyKey();
      if (pending === "pay") {
        const response = await request<LoanSchedule>(
          `/loans/${loan.id}/pay-installment`,
          {
            method: "POST",
            idempotencyKey,
            body: { installment_no: Number(installmentNo) },
          }
        );
        const schedule = unwrap(response);
        setFeedback({
          title: t.loans.paymentRecordedTitle,
          reference: `${t.loans.installmentRefPrefix}${schedule.installment_no}`,
          description: `${t.loans.statusDescPrefix} ${schedule.status}${t.loans.statusDescSuffix}`,
        });
      } else {
        const response = await request<Loan>(`/loans/${loan.id}/${pending}`, {
          method: "POST",
          idempotencyKey,
        });
        const updated = unwrap(response);
        setFeedback({
          title: actionLabel(t.loans, pending),
          reference: updated.loan_number,
          description: `${t.loans.currentStatusPrefix} ${updated.status}${t.loans.statusDescSuffix}`,
        });
      }
      setPending(null);
      setReloadKey((key) => key + 1);
      onChanged();
    } catch (err) {
      setActionError(
        err instanceof ApiError ? err.message : t.loans.actionFailed
      );
      setPending(null);
    } finally {
      setSubmitting(false);
    }
  };

  const scheduleColumns: Column<LoanSchedule>[] = [
    { header: t.loans.colInstallmentNo, accessorKey: "installment_no", isMono: true },
    {
      header: t.loans.colDueDate,
      cell: (row) => formatDate(row.due_date),
      isMono: true,
    },
    {
      header: t.loans.principal,
      type: "money",
      cell: (row) => <MoneyText value={row.principal_amount} />,
    },
    {
      header: loan?.schedules?.[0]
        ? profitLabel(t.loans, loan.schedules[0].profit_type)
        : syariah
          ? t.loans.colProfitSyariah
          : t.loans.colProfitConventional,
      type: "money",
      cell: (row) => <MoneyText value={row.profit_amount} />,
    },
    {
      header: t.loans.colTotal,
      type: "money",
      cell: (row) => <MoneyText value={row.total_installment} />,
    },
    {
      header: t.loans.colPaidPrincipal,
      type: "money",
      cell: (row) => <MoneyText value={row.paid_principal} />,
    },
    {
      header: t.loans.colPaidProfit,
      type: "money",
      cell: (row) => <MoneyText value={row.paid_profit} />,
    },
    { header: t.common.status, accessorKey: "status", type: "status" },
  ];

  if (loading) {
    return (
      <Card className="mt-4">
        <CardContent>
          <LoadingState label={t.loans.detailLoading} />
        </CardContent>
      </Card>
    );
  }

  if (error || !loan) {
    return (
      <div className="mt-4">
        <ErrorState
          title={t.loans.detailErrorTitle}
          description={error ?? t.loans.notFound}
          action={
            <Button variant="secondary" onClick={onClose}>
              {t.common.close}
            </Button>
          }
        />
      </div>
    );
  }

  const canPay = loan.status === "DISBURSED";

  return (
    <Card className="mt-4">
      <CardHeader>
        <CardTitle>
          {t.loans.detailTitle}{" "}
          <span className="font-mono">{loan.loan_number}</span>
        </CardTitle>
        <Button variant="ghost" size="sm" onClick={onClose}>
          {t.common.close}
        </Button>
      </CardHeader>
      <CardContent>
        {feedback && <ActionFeedback feedback={feedback} />}
        {actionError && (
          <p className="text-body text-debit-700" role="alert">
            {actionError}
          </p>
        )}

        <DefinitionList
          items={[
            { label: t.loans.customer, value: customerLabel(loan.customer_id, customerNames) },
            { label: t.loans.labelProduct, value: product ? `${product.code} - ${product.name}` : "-" },
            { label: t.common.status, value: loan.status },
            {
              label: t.loans.labelScheme,
              value: schemeLabel(t.loans, product),
            },
            { label: t.loans.principal, value: <MoneyText value={loan.principal_amount} /> },
            {
              label: t.loans.labelRemainingPrincipal,
              value: <MoneyText value={loan.outstanding_principal} />,
            },
            {
              label: t.loans.labelTotalPayable,
              value: <MoneyText value={loan.total_payable} />,
            },
            {
              label: t.loans.labelMonthlyInstallment,
              value: <MoneyText value={loan.monthly_installment} />,
            },
            {
              label: t.loans.labelTerm,
              value: `${loan.term_months} ${t.loans.termSuffix}`,
              isMono: true,
            },
            syariah
              ? {
                  label: t.loans.labelMargin,
                  value: <MoneyText value={loan.margin_amount} />,
                }
              : {
                  label: t.loans.labelInterestAnnual,
                  value: formatRate(loan.interest_rate_annual),
                  isMono: true,
                },
            syariah && Number(loan.profit_sharing_ratio) > 0
              ? {
                  label: t.loans.labelProfitSharing,
                  value: formatRate(
                    String(Number(loan.profit_sharing_ratio) * 100)
                  ),
                  isMono: true,
                }
              : null,
            {
              label: t.loans.labelCollectibility,
              value: <CollectibilityBadge value={loan.collectibility} />,
            },
            { label: t.loans.labelDpd, value: `${loan.dpd} ${t.loans.dpdSuffix}`, isMono: true },
            { label: t.loans.labelPurpose, value: loan.purpose || "-" },
          ].filter((item) => item !== null)}
        />

        <div className="flex flex-wrap gap-2">
          {loan.status === "PENDING_APPROVAL" && (
            <>
              <Button size="sm" onClick={() => openAction("approve")}>
                {t.loans.approveButton}
              </Button>
              <Button
                size="sm"
                variant="danger"
                onClick={() => openAction("reject")}
              >
                {t.loans.rejectButton}
              </Button>
            </>
          )}
          {loan.status === "APPROVED" && (
            <Button size="sm" onClick={() => openAction("disburse")}>
              {t.loans.disburseButton}
            </Button>
          )}
          {/* Perjanjian kredit dapat dicetak kapan pun selama kredit ada di sistem. */}
          <PrintButton
            url={`/documents/loan-agreement/${encodeURIComponent(loan.id)}`}
            label={t.loans.printAgreement}
          />
          {canPay && (
            <>
              <div className="w-72">
                <Select
                  label={t.loans.paidInstallmentLabel}
                  value={installmentNo}
                  onChange={(e) => setInstallmentNo(e.target.value)}
                  options={unpaidSchedules.map((s) => ({
                    value: String(s.installment_no),
                    label: `${t.loans.installmentOptionPrefix}${s.installment_no}${t.loans.installmentDuePrefix}${formatDate(s.due_date)}`,
                  }))}
                  placeholder={
                    unpaidSchedules.length === 0
                      ? t.loans.allPaid
                      : t.loans.selectInstallment
                  }
                />
              </div>
              <div className="flex items-end">
                <Button
                  size="sm"
                  onClick={() => openAction("pay")}
                  disabled={!installmentNo}
                >
                  {t.loans.payButton}
                </Button>
              </div>
            </>
          )}
        </div>

        <div>
          <h4 className="mb-2 text-title font-medium text-ink-900">
            {t.loans.scheduleTitle}
          </h4>
          <DataTable
            columns={scheduleColumns}
            data={loan.schedules ?? []}
            keyExtractor={(row) => row.id}
            emptyMessage={t.loans.emptySchedules}
            zebra
          />
        </div>
      </CardContent>

      <ConfirmDialog
        open={pending !== null}
        title={pending ? actionLabel(t.loans, pending) : t.common.confirm}
        destructive={pending === "reject"}
        loading={submitting}
        confirmLabel={pending ? actionLabel(t.loans, pending) : t.common.confirm}
        onCancel={() => setPending(null)}
        onConfirm={runAction}
        description={
          pending === "disburse" ? (
            <>
              <p>
                {t.loans.disburseDescPrefix}{" "}
                <MoneyText value={loan.principal_amount} />{" "}
                {t.loans.disburseDescSuffix}
              </p>
              <p className="text-meta">{t.loans.irreversible}</p>
            </>
          ) : pending === "pay" ? (
            <>
              <p>
                {t.loans.payDescPrefix}
                {installmentNo || "-"} {t.loans.payDescSuffix}
              </p>
              <p className="text-meta">{t.loans.payConfirmCheck}</p>
            </>
          ) : pending === "reject" ? (
            <p>
              {t.loans.rejectDescPrefix} {loan.loan_number}{" "}
              {t.loans.rejectDescSuffix}
            </p>
          ) : (
            <p>
              {t.loans.approveDescPrefix}{" "}
              <span className="font-mono">{loan.loan_number}</span>{" "}
              {t.loans.approveDescSuffix}
            </p>
          )
        }
      />
    </Card>
  );
}

export interface LoanWorkspaceProps {
  book: COABook;
}

/**
 * Daftar + detail kredit/pembiayaan. /kredit memakai buku CONVENTIONAL,
 * /pembiayaan memakai buku SYARIAH. Endpoint sama: /loans.
 */
export function LoanWorkspace({ book }: LoanWorkspaceProps) {
  const { t } = useTranslation();
  const syariah = book === "SYARIAH";
  const [loans, setLoans] = useState<Loan[]>([]);
  const [meta, setMeta] = useState<Meta | null>(null);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [products, setProducts] = useState<BankingProduct[]>([]);
  const [customers, setCustomers] = useState<Customer[]>([]);
  const customerNames = useMemo(
    () => Object.fromEntries(customers.map((c) => [c.id, c.full_name])),
    [customers]
  );
  const [formOpen, setFormOpen] = useState(false);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [listReloadKey, setListReloadKey] = useState(0);

  const loadLoans = useCallback(async (targetPage: number) => {
    setLoading(true);
    setError(null);
    try {
      const response = await request<Loan[]>(
        `/loans?page=${targetPage}&page_size=${PAGE_SIZE}`
      );
      setLoans(response.data ?? []);
      if (response.meta) setMeta(response.meta as Meta);
    } catch (err) {
      setError(
        err instanceof ApiError ? err.message : t.loans.listLoadError
      );
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    loadLoans(page);
  }, [page, loadLoans, listReloadKey]);

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

  const productById = useMemo(
    () => Object.fromEntries(products.map((p) => [p.id, p])),
    [products]
  );

  const applyProducts = useMemo(
    () =>
      products.filter(
        (p) => p.family === "LOAN" && p.book === book && p.is_active
      ),
    [products, book]
  );

  const productsReady = products.length > 0;
  const visibleLoans = useMemo(() => {
    if (!productsReady) return loans;
    return loans.filter(
      (loan) => loan.product_id && productById[loan.product_id]?.book === book
    );
  }, [loans, productsReady, productById, book]);

  const columns: Column<Loan>[] = [
    { header: t.loans.colLoanNumber, accessorKey: "loan_number", isMono: true },
    {
      header: t.loans.customer,
      cell: (row) => customerLabel(row.customer_id, customerNames),
    },
    {
      header: t.loans.principal,
      type: "money",
      cell: (row) => <MoneyText value={row.principal_amount} />,
    },
    {
      header: t.loans.labelRemainingPrincipal,
      type: "money",
      cell: (row) => <MoneyText value={row.outstanding_principal} />,
    },
    { header: t.common.status, accessorKey: "status", type: "status" },
    ...(syariah
      ? [
          {
            header: t.loans.labelScheme,
            cell: (row: Loan) =>
              schemeLabel(t.loans, row.product_id ? productById[row.product_id] : undefined),
          } as Column<Loan>,
        ]
      : []),
    {
      header: t.loans.labelCollectibility,
      cell: (row) => <CollectibilityBadge value={row.collectibility} />,
    },
    {
      header: "",
      cell: (row) => (
        <Button
          size="sm"
          variant="secondary"
          onClick={() => setSelectedId(row.id)}
        >
          {t.loans.detailButton}
        </Button>
      ),
    },
  ];

  return (
    <>
      {formOpen && (
        <LoanApplyForm
          book={book}
          products={applyProducts}
          customers={customers}
          onApplied={(loan) => {
            setFormOpen(false);
            setSelectedId(loan.id);
            setPage(1);
            setListReloadKey((key) => key + 1);
          }}
          onClose={() => setFormOpen(false)}
        />
      )}

      {error && !loading ? (
        <ErrorState title={t.loans.errorTitle} description={error} />
      ) : (
        <Card>
          <CardHeader>
            <CardTitle>
              {syariah ? t.loans.listTitleSyariah : t.loans.listTitleConventional}
            </CardTitle>
            <Button size="sm" onClick={() => setFormOpen((open) => !open)}>
              {t.loans.applyButton}
            </Button>
          </CardHeader>
          <CardContent className="p-0">
            {syariah && productsReady && (
              <p className="border-b border-border px-4 py-2 text-meta text-ink-600">
                {t.loans.syariahFilterNote}
              </p>
            )}
            <DataTable
              columns={columns}
              data={visibleLoans}
              keyExtractor={(row) => row.id}
              loading={loading}
              emptyMessage={
                syariah
                  ? t.loans.emptySyariah
                  : t.loans.emptyConventional
              }
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
        <LoanDetailPanel
          loanId={selectedId}
          book={book}
          products={products}
          customerNames={customerNames}
          onClose={() => setSelectedId(null)}
          onChanged={() => setListReloadKey((key) => key + 1)}
        />
      )}
    </>
  );
}
