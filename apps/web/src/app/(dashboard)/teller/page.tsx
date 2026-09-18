"use client";

import { useState } from "react";
import type { JournalEntry, JournalLine } from "@cbs/shared-types";
import { ApiError, newIdempotencyKey, request, unwrap } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
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

type TrxType = "deposit" | "withdraw" | "transfer";

const TRX_OPTIONS = [
  { value: "deposit", label: "Setoran Tunai" },
  { value: "withdraw", label: "Penarikan Tunai" },
  { value: "transfer", label: "Transfer Internal" },
];

function trxPath(type: TrxType): string {
  if (type === "deposit") return "/transactions/deposit";
  if (type === "withdraw") return "/transactions/withdraw";
  return "/transactions/transfer";
}

function trxLabel(type: TrxType): string {
  return TRX_OPTIONS.find((option) => option.value === type)?.label ?? type;
}

export default function TellerPage() {
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

  const validate = (): string | null => {
    if (amount <= 0) return "Nominal harus lebih besar dari nol.";
    if (type === "transfer") {
      if (!sourceAccount.trim() || !destinationAccount.trim()) {
        return "Rekening sumber dan tujuan wajib diisi.";
      }
    } else if (!accountNumber.trim()) {
      return "Nomor rekening wajib diisi.";
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

      const response = await request<JournalEntry>(trxPath(type), {
        method: "POST",
        body: payload,
        idempotencyKey,
      });
      setResult(unwrap(response));
      setAmount(0);
      setDescription("");
      setConfirmOpen(false);
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : "Transaksi gagal diproses.");
      setConfirmOpen(false);
    } finally {
      setSubmitting(false);
    }
  };

  const lineColumns: Column<JournalLine>[] = [
    { header: "Rekening", accessorKey: "account_number", isMono: true },
    { header: "Arah", accessorKey: "direction" },
    {
      header: "Nominal",
      type: "money",
      cell: (row) => (
        <MoneyText
          value={row.amount}
          tone={row.direction === "DEBIT" ? "debit" : "credit"}
        />
      ),
    },
    {
      header: "Saldo Setelah",
      type: "money",
      cell: (row) => <MoneyText value={row.balance_after} />,
    },
    { header: "Keterangan", accessorKey: "description" },
  ];

  return (
    <>
      <PageHeader
        title="Teller"
        description="Setoran, penarikan, dan transfer internal. Setiap tindakan melewati konfirmasi sebelum diposting."
      />

      <div className="grid grid-cols-3 gap-4">
        <div className="col-span-2">
          <Card>
            <CardHeader>
              <CardTitle>Transaksi Baru</CardTitle>
            </CardHeader>
            <CardContent>
              <form onSubmit={openConfirm} className="space-y-4">
                <Select
                  label="Jenis Transaksi"
                  value={type}
                  onChange={(e) => setType(e.target.value as TrxType)}
                  options={TRX_OPTIONS}
                />

                {type === "transfer" ? (
                  <div className="grid grid-cols-2 gap-4">
                    <Input
                      label="Rekening Sumber (Debit)"
                      value={sourceAccount}
                      onChange={(e) => setSourceAccount(e.target.value)}
                      isMono
                    />
                    <Input
                      label="Rekening Tujuan (Kredit)"
                      value={destinationAccount}
                      onChange={(e) => setDestinationAccount(e.target.value)}
                      isMono
                    />
                  </div>
                ) : (
                  <Input
                    label="Nomor Rekening"
                    value={accountNumber}
                    onChange={(e) => setAccountNumber(e.target.value)}
                    isMono
                  />
                )}

                <CurrencyInput
                  label="Nominal (IDR)"
                  value={amount}
                  onChange={setAmount}
                />

                <Input
                  label="Keterangan"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                />

                {formError && (
                  <p className="text-meta text-debit-700" role="alert">
                    {formError}
                  </p>
                )}

                <Button type="submit">Lanjut Konfirmasi</Button>
              </form>
            </CardContent>
          </Card>
        </div>

        <div>
          {result && (
            <Card>
              <CardHeader>
                <CardTitle>Bukti Posting</CardTitle>
                <StatusBadge status={result.status} />
              </CardHeader>
              <CardContent>
                <DefinitionList
                  columns={1}
                  items={[
                    {
                      label: "Nomor Referensi",
                      value: result.reference_number,
                      isMono: true,
                    },
                    { label: "Jenis", value: result.transaction_type },
                    {
                      label: "Waktu",
                      value: formatDateTime(result.posted_at),
                      isMono: true,
                    },
                  ]}
                />
              </CardContent>
            </Card>
          )}
        </div>
      </div>

      {result && (
        <Card className="mt-4">
          <CardHeader>
            <CardTitle>Rincian Jurnal</CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            <DataTable
              columns={lineColumns}
              data={result.lines ?? []}
              keyExtractor={(row) => row.id}
              emptyMessage="Rincian baris jurnal tidak dikembalikan oleh API."
              zebra
            />
          </CardContent>
        </Card>
      )}

      <ConfirmDialog
        open={confirmOpen}
        title="Konfirmasi Transaksi"
        destructive={type === "withdraw"}
        loading={submitting}
        confirmLabel="Posting Transaksi"
        onCancel={() => setConfirmOpen(false)}
        onConfirm={submit}
        description={
          <>
            <p>
              Anda akan memposting <strong>{trxLabel(type)}</strong> dengan rincian:
            </p>
            <dl className="mt-2 space-y-1">
              {type === "transfer" ? (
                <>
                  <div className="flex justify-between">
                    <dt className="text-ink-600">Rekening sumber</dt>
                    <dd className="font-mono">{sourceAccount || "-"}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-ink-600">Rekening tujuan</dt>
                    <dd className="font-mono">{destinationAccount || "-"}</dd>
                  </div>
                </>
              ) : (
                <div className="flex justify-between">
                  <dt className="text-ink-600">Rekening</dt>
                  <dd className="font-mono">{accountNumber || "-"}</dd>
                </div>
              )}
              <div className="flex justify-between">
                <dt className="text-ink-600">Nominal</dt>
                <dd>
                  <MoneyText value={amount} />
                </dd>
              </div>
            </dl>
            <p className="text-meta">Jurnal yang diposting tidak dapat dibatalkan sembarangan.</p>
          </>
        }
      />
    </>
  );
}
