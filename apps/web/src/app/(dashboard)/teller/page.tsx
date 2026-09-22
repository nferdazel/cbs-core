"use client";

import { useMemo, useState } from "react";
import type { JournalEntry, JournalLine } from "@cbs/shared-types";
import { ApiError, newIdempotencyKey, request, unwrap } from "@/lib/api";
import {
  isPendingApproval,
  type PendingApprovalResult,
} from "@/lib/operations-types";
import { formatDateTime } from "@/lib/format";
import { useTranslation } from "@/i18n/context";
import { PageHeader } from "@/components/ui/PageHeader";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { CurrencyInput } from "@/components/ui/CurrencyInput";
import { Select } from "@/components/ui/Select";
import { MoneyText } from "@/components/ui/MoneyText";
import { DataTable, Column } from "@/components/ui/DataTable";
import { DefinitionList } from "@/components/ui/DefinitionList";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { StatusBadge } from "@/components/ui/StatusBadge";
import { PrintButton } from "@/components/ui/PrintButton";

type TrxType = "deposit" | "withdraw" | "transfer";

function trxPath(type: TrxType): string {
  if (type === "deposit") return "/transactions/deposit";
  if (type === "withdraw") return "/transactions/withdraw";
  return "/transactions/transfer";
}

export default function TellerPage() {
  const { t } = useTranslation();
  const [type, setType] = useState<TrxType>("deposit");
  const [accountNumber, setAccountNumber] = useState("");
  const [sourceAccount, setSourceAccount] = useState("");
  const [destinationAccount, setDestinationAccount] = useState("");
  const [amount, setAmount] = useState(0);
  const [description, setDescription] = useState("");

  const [confirmOpen, setConfirmOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [result, setResult] = useState<JournalEntry | null>(null);
  const [pendingApproval, setPendingApproval] =
    useState<PendingApprovalResult | null>(null);

  // Jenis transaksi adalah kunci teknis (deposit dll.); hanya label tampilannya
  // yang diambil dari kamus.
  const trxOptions = useMemo(
    () => [
      { value: "deposit", label: t.teller.trxDeposit },
      { value: "withdraw", label: t.teller.trxWithdraw },
      { value: "transfer", label: t.teller.trxTransfer },
    ],
    [t]
  );

  const trxLabel = (value: TrxType): string =>
    trxOptions.find((option) => option.value === value)?.label ?? value;

  const validate = (): string | null => {
    if (amount <= 0) return t.teller.amountRequired;
    if (type === "transfer") {
      if (!sourceAccount.trim() || !destinationAccount.trim()) {
        return t.teller.accountsRequired;
      }
    } else if (!accountNumber.trim()) {
      return t.teller.accountRequired;
    }
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
    setPendingApproval(null);
    try {
      const idempotencyKey = newIdempotencyKey();
      const payload =
        type === "transfer"
          ? {
              source_account_number: sourceAccount.trim(),
              destination_account_number: destinationAccount.trim(),
              amount: String(amount),
              currency: "IDR",
              description: description.trim() || undefined,
              idempotency_key: idempotencyKey,
            }
          : {
              account_number: accountNumber.trim(),
              amount: String(amount),
              currency: "IDR",
              description: description.trim() || undefined,
              idempotency_key: idempotencyKey,
            };

      const response = await request<JournalEntry | PendingApprovalResult>(
        trxPath(type),
        {
          method: "POST",
          body: payload,
          idempotencyKey,
        }
      );
      // Transaksi di atas ambang limit dibalas 202: masuk antrean maker-checker dan
      // BELUM diposting, sehingga tidak boleh ditampilkan sebagai bukti posting.
      const data = unwrap(response);
      if (isPendingApproval(data)) {
        setPendingApproval(data);
        setResult(null);
      } else {
        setResult(data);
      }
      setAmount(0);
      setDescription("");
      setConfirmOpen(false);
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : t.teller.submitError);
      setConfirmOpen(false);
    } finally {
      setSubmitting(false);
    }
  };

  const lineColumns: Column<JournalLine>[] = [
    { header: t.teller.colAccount, accessorKey: "account_number", isMono: true },
    { header: t.teller.colDirection, accessorKey: "direction" },
    {
      header: t.teller.colAmount,
      type: "money",
      cell: (row) => (
        <MoneyText
          value={row.amount}
          tone={row.direction === "DEBIT" ? "debit" : "credit"}
        />
      ),
    },
    {
      header: t.teller.colBalanceAfter,
      type: "money",
      cell: (row) => <MoneyText value={row.balance_after} />,
    },
    { header: t.teller.colDescription, accessorKey: "description" },
  ];

  return (
    <>
      <PageHeader
        title={t.teller.title}
        description={t.teller.description}
      />

      <div className="grid grid-cols-3 gap-4">
        <div className="col-span-2">
          <Card>
            <CardHeader>
              <CardTitle>{t.teller.newTrxTitle}</CardTitle>
            </CardHeader>
            <CardContent>
              <form onSubmit={openConfirm} className="space-y-4">
                <Select
                  label={t.teller.trxTypeLabel}
                  value={type}
                  onChange={(e) => setType(e.target.value as TrxType)}
                  options={trxOptions}
                />

                {type === "transfer" ? (
                  <div className="grid grid-cols-2 gap-4">
                    <Input
                      label={t.teller.sourceAccountLabel}
                      value={sourceAccount}
                      onChange={(e) => setSourceAccount(e.target.value)}
                      isMono
                    />
                    <Input
                      label={t.teller.destinationAccountLabel}
                      value={destinationAccount}
                      onChange={(e) => setDestinationAccount(e.target.value)}
                      isMono
                    />
                  </div>
                ) : (
                  <Input
                    label={t.teller.accountNumberLabel}
                    value={accountNumber}
                    onChange={(e) => setAccountNumber(e.target.value)}
                    isMono
                  />
                )}

                <CurrencyInput
                  label={t.teller.amountLabel}
                  value={amount}
                  onChange={setAmount}
                />

                <Input
                  label={t.teller.descriptionLabel}
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                />

                {formError && (
                  <p className="text-meta text-debit-700" role="alert">
                    {formError}
                  </p>
                )}

                <Button type="submit">{t.teller.continueButton}</Button>
              </form>
            </CardContent>
          </Card>
        </div>

        <div>
          {pendingApproval && (
            <Card>
              <CardHeader>
                <CardTitle>{t.teller.pendingTitle}</CardTitle>
                <StatusBadge status={pendingApproval.status} />
              </CardHeader>
              <CardContent>
                <p className="text-body text-ink-600">
                  {t.teller.pendingDesc}
                </p>
                <DefinitionList
                  columns={1}
                  items={[
                    {
                      label: t.teller.requestIdLabel,
                      value: pendingApproval.request_id,
                      isMono: true,
                    },
                    {
                      label: t.teller.actionTypeLabel,
                      value: pendingApproval.action_type,
                    },
                  ]}
                />
              </CardContent>
            </Card>
          )}

          {result && (
            <Card>
              <CardHeader>
                <CardTitle>{t.teller.resultTitle}</CardTitle>
                <StatusBadge status={result.status} />
              </CardHeader>
              <CardContent>
                <DefinitionList
                  columns={1}
                  items={[
                    {
                      label: t.teller.refLabel,
                      value: result.reference_number,
                      isMono: true,
                    },
                    { label: t.teller.typeLabel, value: result.transaction_type },
                    {
                      label: t.teller.timeLabel,
                      value: formatDateTime(result.posted_at),
                      isMono: true,
                    },
                  ]}
                />
                {/* Transfer tidak punya dokumen slip; hanya setoran/penarikan. */}
                {type !== "transfer" && result.reference_number && (
                  <div className="mt-3">
                    <PrintButton
                      url={`/documents/${
                        type === "deposit" ? "deposit-slip" : "withdrawal-slip"
                      }/${encodeURIComponent(result.reference_number)}`}
                      label={
                        type === "deposit"
                          ? t.teller.printDepositSlip
                          : t.teller.printWithdrawalSlip
                      }
                    />
                  </div>
                )}
              </CardContent>
            </Card>
          )}
        </div>
      </div>

      {result && (
        <Card className="mt-4">
          <CardHeader>
            <CardTitle>{t.teller.journalTitle}</CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            <DataTable
              columns={lineColumns}
              data={result.lines ?? []}
              keyExtractor={(row) => row.id}
              emptyMessage={t.teller.emptyLines}
              zebra
            />
          </CardContent>
        </Card>
      )}

      <ConfirmDialog
        open={confirmOpen}
        title={t.teller.confirmTitle}
        destructive={type === "withdraw"}
        loading={submitting}
        confirmLabel={t.teller.confirmButton}
        onCancel={() => setConfirmOpen(false)}
        onConfirm={submit}
        description={
          <>
            <p>
              {t.teller.confirmIntro} <strong>{trxLabel(type)}</strong>{" "}
              {t.teller.confirmIntroSuffix}
            </p>
            <dl className="mt-2 space-y-1">
              {type === "transfer" ? (
                <>
                  <div className="flex justify-between">
                    <dt className="text-ink-600">{t.teller.sourceAccountShort}</dt>
                    <dd className="font-mono">{sourceAccount || "-"}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-ink-600">{t.teller.destAccountShort}</dt>
                    <dd className="font-mono">{destinationAccount || "-"}</dd>
                  </div>
                </>
              ) : (
                <div className="flex justify-between">
                  <dt className="text-ink-600">{t.teller.accountShort}</dt>
                  <dd className="font-mono">{accountNumber || "-"}</dd>
                </div>
              )}
              <div className="flex justify-between">
                <dt className="text-ink-600">{t.teller.amountShort}</dt>
                <dd>
                  <MoneyText value={amount} />
                </dd>
              </div>
            </dl>
            <p className="text-meta">{t.teller.irreversible}</p>
          </>
        }
      />
    </>
  );
}
