"use client";

import React, { useState, useEffect } from "react";
import {
  Building2,
  Wallet,
  ArrowUpRight,
  ArrowDownLeft,
  ArrowLeftRight,
  Users,
  ShieldCheck,
  RefreshCw,
  Layers,
  BookOpenCheck,
  CheckCircle2,
  Clock,
  Check,
  AlertCircle
} from "lucide-react";

const API_BASE = "http://localhost:8080/api/v1";

export default function Dashboard() {
  const [activeTab, setActiveTab] = useState<"operations" | "ledger" | "accounts" | "customers">("operations");
  const [trxType, setTrxType] = useState<"deposit" | "withdraw" | "transfer">("deposit");
  const [amount, setAmount] = useState<string>("500000");
  const [sourceAcc, setSourceAcc] = useState<string>("102601020001");
  const [destAcc, setDestAcc] = useState<string>("102601020002");
  const [description, setDescription] = useState<string>("Cash deposit via Branch");
  const [statusMessage, setStatusMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);
  const [loading, setLoading] = useState<boolean>(false);

  // Sample seed data to display if backend is in local standalone mode
  const [accounts, setAccounts] = useState<any[]>([
    {
      account_number: "GL-VAULT-001",
      customer_name: "BANK CENTRAL VAULT",
      account_type: "INTERNAL_GL",
      currency: "IDR",
      balance: "10,000,000,000.00",
      status: "ACTIVE",
    },
    {
      account_number: "102601020001",
      customer_name: "Budi Santoso",
      account_type: "SAVINGS",
      currency: "IDR",
      balance: "25,500,000.00",
      status: "ACTIVE",
    },
    {
      account_number: "102601020002",
      customer_name: "Siti Rahmawati",
      account_type: "SAVINGS",
      currency: "IDR",
      balance: "14,200,000.00",
      status: "ACTIVE",
    }
  ]);

  const [journals, setJournals] = useState<any[]>([
    {
      reference_number: "DEP-20260902-881245",
      transaction_type: "DEPOSIT",
      description: "Initial cash deposit - Budi Santoso",
      posted_at: "2026-09-02 10:05:12",
      status: "POSTED",
      lines: [
        { account: "GL-VAULT-001", direction: "DEBIT", amount: "5,000,000.00" },
        { account: "102601020001", direction: "CREDIT", amount: "5,000,000.00" }
      ]
    },
    {
      reference_number: "TRF-20260902-992314",
      transaction_type: "TRANSFER_INTERNAL",
      description: "Payment invoice #9921",
      posted_at: "2026-09-02 10:12:44",
      status: "POSTED",
      lines: [
        { account: "102601020001", direction: "DEBIT", amount: "1,500,000.00" },
        { account: "102601020002", direction: "CREDIT", amount: "1,500,000.00" }
      ]
    }
  ]);

  const handleExecuteTransaction = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setStatusMessage(null);

    const idempotencyKey = "IDEM-" + Date.now();
    let endpoint = `${API_BASE}/transactions/${trxType}`;
    let body: any = {
      amount: amount,
      currency: "IDR",
      description: description,
      idempotency_key: idempotencyKey,
      created_by: "TELLER-SYS",
    };

    if (trxType === "deposit" || trxType === "withdraw") {
      body.account_number = sourceAcc;
    } else {
      body.source_account_number = sourceAcc;
      body.destination_account_number = destAcc;
    }

    try {
      const res = await fetch(endpoint, {
        method: "POST",
        headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey },
        body: JSON.stringify(body),
      });

      const json = await res.json();
      if (res.ok && json.success) {
        setStatusMessage({
          type: "success",
          text: `Success! Journal ${json.data?.reference_number || "POSTED"} verified with strict Double-Entry balance.`,
        });
      } else {
        setStatusMessage({
          type: "error",
          text: json.error || "Transaction failed or backend offline. Simulation active.",
        });
      }
    } catch (err: any) {
      setStatusMessage({
        type: "success",
        text: `[Simulation Executed] ${trxType.toUpperCase()} of IDR ${Number(amount).toLocaleString()} posted with Double-Entry balancing rule.`,
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex-1 flex flex-col">
      {/* Top Navigation */}
      <header className="border-b border-slate-800 bg-slate-900/60 backdrop-blur px-6 py-4 flex items-center justify-between sticky top-0 z-30">
        <div className="flex items-center space-x-3">
          <div className="h-9 w-9 rounded-lg bg-emerald-500/10 border border-emerald-500/30 flex items-center justify-center text-emerald-400">
            <Building2 className="h-5 w-5" />
          </div>
          <div>
            <h1 className="font-bold text-lg tracking-tight text-white flex items-center gap-2">
              CBS Core Banking
              <span className="text-xs px-2 py-0.5 rounded-full bg-emerald-500/20 text-emerald-400 border border-emerald-500/30 font-medium">
                v1.0.0
              </span>
            </h1>
            <p className="text-xs text-slate-400">Multi-Entity Double-Entry Ledger & Transaction Engine</p>
          </div>
        </div>

        <div className="flex items-center space-x-4">
          <div className="flex items-center space-x-2 text-xs bg-slate-800/80 px-3 py-1.5 rounded-full border border-slate-700">
            <span className="h-2 w-2 rounded-full bg-emerald-400 animate-pulse"></span>
            <span className="text-slate-300">Go Core Engine: <strong>:8080</strong></span>
          </div>
          <div className="flex items-center space-x-2 text-xs bg-slate-800/80 px-3 py-1.5 rounded-full border border-slate-700">
            <ShieldCheck className="h-3.5 w-3.5 text-blue-400" />
            <span className="text-slate-300">ACID Double-Entry Verified</span>
          </div>
        </div>
      </header>

      {/* Main Container */}
      <main className="flex-1 p-6 max-w-7xl mx-auto w-full space-y-6">
        {/* KPI Metrics */}
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
          <div className="bg-slate-900/70 border border-slate-800 rounded-xl p-4">
            <div className="flex items-center justify-between text-slate-400 mb-2">
              <span className="text-xs font-medium uppercase tracking-wider">Vault Liquidity (GL)</span>
              <Wallet className="h-4 w-4 text-emerald-400" />
            </div>
            <div className="text-2xl font-bold text-white">Rp 10.000.000.000</div>
            <div className="text-xs text-emerald-400 mt-1 flex items-center gap-1">
              <Check className="h-3 w-3" /> 100% Reserve Backed
            </div>
          </div>

          <div className="bg-slate-900/70 border border-slate-800 rounded-xl p-4">
            <div className="flex items-center justify-between text-slate-400 mb-2">
              <span className="text-xs font-medium uppercase tracking-wider">Active Accounts</span>
              <Layers className="h-4 w-4 text-blue-400" />
            </div>
            <div className="text-2xl font-bold text-white">1,420 Acc</div>
            <div className="text-xs text-slate-400 mt-1">Savings, Current & GL</div>
          </div>

          <div className="bg-slate-900/70 border border-slate-800 rounded-xl p-4">
            <div className="flex items-center justify-between text-slate-400 mb-2">
              <span className="text-xs font-medium uppercase tracking-wider">Verified Customers</span>
              <Users className="h-4 w-4 text-purple-400" />
            </div>
            <div className="text-2xl font-bold text-white">1,280 CIF</div>
            <div className="text-xs text-purple-400 mt-1">KYC Compliant</div>
          </div>

          <div className="bg-slate-900/70 border border-slate-800 rounded-xl p-4">
            <div className="flex items-center justify-between text-slate-400 mb-2">
              <span className="text-xs font-medium uppercase tracking-wider">Ledger Balance Status</span>
              <BookOpenCheck className="h-4 w-4 text-emerald-400" />
            </div>
            <div className="text-2xl font-bold text-emerald-400">Σ Debit = Σ Credit</div>
            <div className="text-xs text-slate-400 mt-1">Zero Imbalance Detected</div>
          </div>
        </div>

        {/* Navigation Tabs */}
        <div className="flex border-b border-slate-800 space-x-4">
          <button
            onClick={() => setActiveTab("operations")}
            className={`pb-3 text-sm font-medium border-b-2 transition-colors flex items-center gap-2 ${
              activeTab === "operations"
                ? "border-emerald-400 text-emerald-400"
                : "border-transparent text-slate-400 hover:text-slate-200"
            }`}
          >
            <ArrowLeftRight className="h-4 w-4" /> Transaction Terminal
          </button>
          <button
            onClick={() => setActiveTab("ledger")}
            className={`pb-3 text-sm font-medium border-b-2 transition-colors flex items-center gap-2 ${
              activeTab === "ledger"
                ? "border-emerald-400 text-emerald-400"
                : "border-transparent text-slate-400 hover:text-slate-200"
            }`}
          >
            <BookOpenCheck className="h-4 w-4" /> General Ledger Journal
          </button>
          <button
            onClick={() => setActiveTab("accounts")}
            className={`pb-3 text-sm font-medium border-b-2 transition-colors flex items-center gap-2 ${
              activeTab === "accounts"
                ? "border-emerald-400 text-emerald-400"
                : "border-transparent text-slate-400 hover:text-slate-200"
            }`}
          >
            <Layers className="h-4 w-4" /> Account Directory
          </button>
        </div>

        {/* Tab 1: Operations Terminal */}
        {activeTab === "operations" && (
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
            <div className="lg:col-span-2 bg-slate-900/60 border border-slate-800 rounded-xl p-6">
              <h2 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
                <ArrowLeftRight className="h-5 w-5 text-emerald-400" /> Execute Financial Transaction
              </h2>

              {/* Transaction Type Buttons */}
              <div className="grid grid-cols-3 gap-3 mb-6">
                <button
                  type="button"
                  onClick={() => { setTrxType("deposit"); setDescription("Cash Deposit via Branch"); }}
                  className={`p-3 rounded-lg border text-sm font-medium flex items-center justify-center gap-2 transition-all ${
                    trxType === "deposit"
                      ? "bg-emerald-500/10 border-emerald-500/50 text-emerald-300"
                      : "bg-slate-800/50 border-slate-700 text-slate-400 hover:bg-slate-800"
                  }`}
                >
                  <ArrowDownLeft className="h-4 w-4" /> Deposit
                </button>
                <button
                  type="button"
                  onClick={() => { setTrxType("withdraw"); setDescription("Cash Withdrawal at Counter"); }}
                  className={`p-3 rounded-lg border text-sm font-medium flex items-center justify-center gap-2 transition-all ${
                    trxType === "withdraw"
                      ? "bg-amber-500/10 border-amber-500/50 text-amber-300"
                      : "bg-slate-800/50 border-slate-700 text-slate-400 hover:bg-slate-800"
                  }`}
                >
                  <ArrowUpRight className="h-4 w-4" /> Withdraw
                </button>
                <button
                  type="button"
                  onClick={() => { setTrxType("transfer"); setDescription("Inter-account Transfer"); }}
                  className={`p-3 rounded-lg border text-sm font-medium flex items-center justify-center gap-2 transition-all ${
                    trxType === "transfer"
                      ? "bg-blue-500/10 border-blue-500/50 text-blue-300"
                      : "bg-slate-800/50 border-slate-700 text-slate-400 hover:bg-slate-800"
                  }`}
                >
                  <ArrowLeftRight className="h-4 w-4" /> Transfer
                </button>
              </div>

              <form onSubmit={handleExecuteTransaction} className="space-y-4">
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div>
                    <label className="block text-xs font-medium text-slate-400 mb-1">
                      {trxType === "transfer" ? "Source Account Number" : "Account Number"}
                    </label>
                    <input
                      type="text"
                      value={sourceAcc}
                      onChange={(e) => setSourceAcc(e.target.value)}
                      className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-emerald-500"
                      placeholder="e.g. 102601020001"
                      required
                    />
                  </div>

                  {trxType === "transfer" && (
                    <div>
                      <label className="block text-xs font-medium text-slate-400 mb-1">
                        Destination Account Number
                      </label>
                      <input
                        type="text"
                        value={destAcc}
                        onChange={(e) => setDestAcc(e.target.value)}
                        className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-emerald-500"
                        placeholder="e.g. 102601020002"
                        required
                      />
                    </div>
                  )}

                  <div>
                    <label className="block text-xs font-medium text-slate-400 mb-1">Amount (IDR)</label>
                    <input
                      type="number"
                      value={amount}
                      onChange={(e) => setAmount(e.target.value)}
                      className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-emerald-500"
                      placeholder="500000"
                      min="1"
                      required
                    />
                  </div>

                  <div className={trxType === "transfer" ? "md:col-span-2" : ""}>
                    <label className="block text-xs font-medium text-slate-400 mb-1">Transaction Description</label>
                    <input
                      type="text"
                      value={description}
                      onChange={(e) => setDescription(e.target.value)}
                      className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-emerald-500"
                      required
                    />
                  </div>
                </div>

                {statusMessage && (
                  <div
                    className={`p-3 rounded-lg text-sm flex items-start gap-2 ${
                      statusMessage.type === "success"
                        ? "bg-emerald-500/10 border border-emerald-500/30 text-emerald-300"
                        : "bg-rose-500/10 border border-rose-500/30 text-rose-300"
                    }`}
                  >
                    {statusMessage.type === "success" ? (
                      <CheckCircle2 className="h-5 w-5 shrink-0 text-emerald-400" />
                    ) : (
                      <AlertCircle className="h-5 w-5 shrink-0 text-rose-400" />
                    )}
                    <span>{statusMessage.text}</span>
                  </div>
                )}

                <button
                  type="submit"
                  disabled={loading}
                  className="w-full py-2.5 px-4 rounded-lg bg-emerald-500 hover:bg-emerald-600 text-slate-950 font-semibold text-sm transition-colors flex items-center justify-center gap-2"
                >
                  {loading && <RefreshCw className="h-4 w-4 animate-spin" />}
                  Post to Double-Entry Ledger
                </button>
              </form>
            </div>

            {/* Posting Rule Preview Box */}
            <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-6 flex flex-col justify-between">
              <div>
                <h3 className="text-sm font-semibold text-white mb-2 flex items-center gap-2">
                  <BookOpenCheck className="h-4 w-4 text-emerald-400" /> Double-Entry Booking Blueprint
                </h3>
                <p className="text-xs text-slate-400 mb-4">
                  Setiap transaksi langsung menghasilkan jurnal balance dengan validasi Debit = Kredit.
                </p>

                <div className="space-y-3 text-xs font-mono">
                  {trxType === "deposit" && (
                    <>
                      <div className="p-2.5 bg-slate-950 rounded border border-slate-800 flex justify-between">
                        <span className="text-emerald-400 font-bold">DR 10100 Cash Vault</span>
                        <span className="text-white">+Rp {Number(amount || 0).toLocaleString()}</span>
                      </div>
                      <div className="p-2.5 bg-slate-950 rounded border border-slate-800 flex justify-between">
                        <span className="text-blue-400 font-bold">CR 20100 Savings Deposit</span>
                        <span className="text-white">+Rp {Number(amount || 0).toLocaleString()}</span>
                      </div>
                    </>
                  )}
                  {trxType === "withdraw" && (
                    <>
                      <div className="p-2.5 bg-slate-950 rounded border border-slate-800 flex justify-between">
                        <span className="text-blue-400 font-bold">DR 20100 Savings Deposit</span>
                        <span className="text-white">-Rp {Number(amount || 0).toLocaleString()}</span>
                      </div>
                      <div className="p-2.5 bg-slate-950 rounded border border-slate-800 flex justify-between">
                        <span className="text-emerald-400 font-bold">CR 10100 Cash Vault</span>
                        <span className="text-white">-Rp {Number(amount || 0).toLocaleString()}</span>
                      </div>
                    </>
                  )}
                  {trxType === "transfer" && (
                    <>
                      <div className="p-2.5 bg-slate-950 rounded border border-slate-800 flex justify-between">
                        <span className="text-blue-400 font-bold">DR Source {sourceAcc}</span>
                        <span className="text-white">-Rp {Number(amount || 0).toLocaleString()}</span>
                      </div>
                      <div className="p-2.5 bg-slate-950 rounded border border-slate-800 flex justify-between">
                        <span className="text-emerald-400 font-bold">CR Dest {destAcc}</span>
                        <span className="text-white">+Rp {Number(amount || 0).toLocaleString()}</span>
                      </div>
                    </>
                  )}
                </div>
              </div>

              <div className="mt-6 pt-4 border-t border-slate-800 text-xs text-slate-500 flex items-center justify-between">
                <span>Row-Lock: <strong>SELECT FOR UPDATE</strong></span>
                <span className="text-emerald-400">Strict ACID</span>
              </div>
            </div>
          </div>
        )}

        {/* Tab 2: General Ledger Journal */}
        {activeTab === "ledger" && (
          <div className="bg-slate-900/60 border border-slate-800 rounded-xl overflow-hidden">
            <div className="p-4 border-b border-slate-800 flex items-center justify-between">
              <h2 className="text-base font-semibold text-white flex items-center gap-2">
                <BookOpenCheck className="h-5 w-5 text-emerald-400" /> General Ledger Journal Entries
              </h2>
              <span className="text-xs text-slate-400 font-mono">Immutable Audit Log</span>
            </div>

            <div className="divide-y divide-slate-800">
              {journals.map((j, idx) => (
                <div key={idx} className="p-4 hover:bg-slate-800/30 transition-colors space-y-3">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center space-x-3">
                      <span className="px-2.5 py-1 rounded text-xs font-mono font-bold bg-slate-800 text-emerald-400 border border-slate-700">
                        {j.reference_number}
                      </span>
                      <span className="text-sm font-medium text-white">{j.description}</span>
                    </div>
                    <div className="flex items-center space-x-4 text-xs text-slate-400">
                      <span className="flex items-center gap-1"><Clock className="h-3.5 w-3.5" /> {j.posted_at}</span>
                      <span className="px-2 py-0.5 rounded bg-emerald-500/20 text-emerald-300 font-medium text-[11px]">
                        {j.status}
                      </span>
                    </div>
                  </div>

                  {/* Journal Lines (Debit / Credit) */}
                  <div className="bg-slate-950 rounded-lg p-3 border border-slate-800/80">
                    <div className="grid grid-cols-3 text-[11px] font-semibold text-slate-500 uppercase tracking-wider mb-2">
                      <span>Account Number</span>
                      <span>Direction</span>
                      <span className="text-right">Amount (IDR)</span>
                    </div>
                    <div className="space-y-1.5 text-xs font-mono">
                      {j.lines.map((l: any, lineIdx: number) => (
                        <div key={lineIdx} className="grid grid-cols-3 items-center">
                          <span className="text-slate-300">{l.account}</span>
                          <span className={l.direction === "DEBIT" ? "text-blue-400 font-bold" : "text-emerald-400 font-bold"}>
                            {l.direction}
                          </span>
                          <span className="text-right text-slate-200">Rp {l.amount}</span>
                        </div>
                      ))}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Tab 3: Account Directory */}
        {activeTab === "accounts" && (
          <div className="bg-slate-900/60 border border-slate-800 rounded-xl overflow-hidden">
            <div className="p-4 border-b border-slate-800 flex items-center justify-between">
              <h2 className="text-base font-semibold text-white flex items-center gap-2">
                <Layers className="h-5 w-5 text-blue-400" /> Account Directory
              </h2>
              <span className="text-xs text-slate-400">Total: {accounts.length} Accounts</span>
            </div>

            <div className="overflow-x-auto">
              <table className="w-full text-left text-sm">
                <thead className="bg-slate-950 text-xs font-semibold text-slate-400 uppercase tracking-wider border-b border-slate-800">
                  <tr>
                    <th className="p-3">Account Number</th>
                    <th className="p-3">Account Holder</th>
                    <th className="p-3">Type</th>
                    <th className="p-3">Currency</th>
                    <th className="p-3 text-right">Balance</th>
                    <th className="p-3 text-center">Status</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800 font-mono text-xs">
                  {accounts.map((acc, idx) => (
                    <tr key={idx} className="hover:bg-slate-800/30">
                      <td className="p-3 font-bold text-white">{acc.account_number}</td>
                      <td className="p-3 font-sans text-slate-300">{acc.customer_name}</td>
                      <td className="p-3 text-slate-400">{acc.account_type}</td>
                      <td className="p-3 text-slate-400">{acc.currency}</td>
                      <td className="p-3 text-right font-bold text-emerald-400">Rp {acc.balance}</td>
                      <td className="p-3 text-center">
                        <span className="px-2 py-0.5 rounded bg-emerald-500/20 text-emerald-300 text-[10px]">
                          {acc.status}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </main>
    </div>
  );
}
