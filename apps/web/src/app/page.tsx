"use client";

import React, { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import {
  Wallet,
  Landmark,
  ShieldCheck,
  BookOpenCheck,
  Users,
  CheckCircle2,
  AlertCircle,
  Calculator,
  FileCheck,
  TrendingUp,
  Clock,
  ArrowRight,
} from "lucide-react";
import { useTranslation } from "@/i18n/context";
import { AppHeader } from "@/components/layout/AppHeader";
import { AppSidebar, TabType } from "@/components/layout/AppSidebar";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";
import { Input } from "@/components/ui/Input";
import { CurrencyInput } from "@/components/ui/CurrencyInput";
import { DataTable, Column } from "@/components/ui/DataTable";

export default function Dashboard() {
  const router = useRouter();
  const { t } = useTranslation();

  const [user, setUser] = useState<any>(null);
  const [activeTab, setActiveTab] = useState<TabType>("operations");

  // Operations Form State
  const [trxType, setTrxType] = useState<"deposit" | "withdraw" | "transfer">("deposit");
  const [amount, setAmount] = useState<number>(5000000);
  const [sourceAcc, setSourceAcc] = useState<string>("102601020001");
  const [destAcc, setDestAcc] = useState<string>("102601020002");
  const [description, setDescription] = useState<string>("Setoran Tunai via Kasir Teller");
  const [statusMessage, setStatusMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);

  // Loan Calculator State
  const [loanProduct, setLoanProduct] = useState<"FLAT" | "MURABAHAH">("MURABAHAH");
  const [loanAmount, setLoanAmount] = useState<number>(50000000);
  const [loanTerm, setLoanTerm] = useState<number>(12);
  const [rateOrMargin, setRateOrMargin] = useState<number>(12.0);

  useEffect(() => {
    const storedUser = localStorage.getItem("cbs_user");
    if (storedUser) {
      try {
        setUser(JSON.parse(storedUser));
      } catch (e) {}
    } else {
      setUser({
        username: "superadmin",
        full_name: "System Administrator",
        role: "SUPERADMIN",
        branch_code: "HO",
      });
    }
  }, []);

  const handleLogout = () => {
    localStorage.removeItem("cbs_access_token");
    localStorage.removeItem("cbs_refresh_token");
    localStorage.removeItem("cbs_user");
    router.push("/login");
  };

  // Mock accounts data
  const accounts = [
    { account_number: "GL-VAULT-001", customer_name: "KAS BRANKAS UTAMA", account_type: "INTERNAL_GL", currency: "IDR", balance: 10000000000, status: "ACTIVE" },
    { account_number: "102601020001", customer_name: "Budi Santoso (CIF-00192)", account_type: "SAVINGS", currency: "IDR", balance: 25500000, status: "ACTIVE" },
    { account_number: "102601020002", customer_name: "Siti Rahmawati (CIF-00204)", account_type: "SAVINGS", currency: "IDR", balance: 14200000, status: "ACTIVE" },
    { account_number: "GL-FEE-INCOME-001", customer_name: "PENDAPATAN ADM & MARGIN", account_type: "INTERNAL_GL", currency: "IDR", balance: 85400000, status: "ACTIVE" },
  ];

  // Mock pending maker-checker approvals
  const [pendingApprovals, setPendingApprovals] = useState([
    { id: "MC-901", maker: "Teller-01", type: "WITHDRAWAL", amount: 75000000, desc: "Penarikan Tunai Rekening #102601020001", status: "PENDING", time: "10 min ago" },
    { id: "MC-902", maker: "AO-Budi", type: "LOAN_ORIGINATION", amount: 150000000, desc: "Pembiayaan Murabahah Toko Sembako (CIF-002)", status: "PENDING", time: "25 min ago" },
  ]);

  // Mock journal entries
  const journals = [
    {
      reference_number: "DISB-PMB-2026-00012",
      posted_at: "2026-09-02 11:30:15",
      description: "Pencairan Pembiayaan Murabahah Modal Usaha Nasabah Budi Santoso",
      lines: [
        { account: "GL-LOAN-RECEIVABLE", direction: "DEBIT", amount: 50000000 },
        { account: "102601020001", direction: "CREDIT", amount: 50000000 },
      ],
    },
    {
      reference_number: "DEP-20260902-881245",
      posted_at: "2026-09-02 10:05:12",
      description: "Setoran Tunai Simpanan Nasabah Budi Santoso via Teller-01",
      lines: [
        { account: "GL-VAULT-001", direction: "DEBIT", amount: 5000000 },
        { account: "102601020001", direction: "CREDIT", amount: 5000000 },
      ],
    },
  ];

  // Calculate Loan Simulation
  const calculateLoanSim = () => {
    let totalInterestOrMargin = 0;
    if (loanProduct === "FLAT") {
      totalInterestOrMargin = loanAmount * (rateOrMargin / 100) * (loanTerm / 12);
    } else {
      totalInterestOrMargin = loanAmount * (rateOrMargin / 100);
    }
    const totalPayable = loanAmount + totalInterestOrMargin;
    const monthlyInstallment = totalPayable / loanTerm;

    return {
      totalInterestOrMargin,
      totalPayable,
      monthlyInstallment,
    };
  };

  const sim = calculateLoanSim();

  const handleApprove = (id: string) => {
    setPendingApprovals(pendingApprovals.filter((a) => a.id !== id));
    setStatusMessage({ type: "success", text: t.approvals.approvedMsg });
  };

  const handleReject = (id: string) => {
    setPendingApprovals(pendingApprovals.filter((a) => a.id !== id));
    setStatusMessage({ type: "error", text: t.approvals.rejectedMsg });
  };

  return (
    <div className="min-h-screen bg-slate-50 text-slate-900 font-sans flex flex-col">
      {/* ── Top Header & Operational Status ── */}
      <AppHeader user={user} onLogout={handleLogout} />

      {/* ── Role Navigation Bar ── */}
      <AppSidebar
        activeTab={activeTab}
        setActiveTab={setActiveTab}
        pendingApprovalsCount={pendingApprovals.length}
      />

      {/* ── Main Canvas ── */}
      <main className="flex-1 max-w-7xl w-full mx-auto px-6 py-6 space-y-6">
        {/* Status Alert Banner */}
        {statusMessage && (
          <div
            className={`p-4 rounded-xl border flex items-center justify-between text-xs font-medium shadow-xs transition-all ${
              statusMessage.type === "success"
                ? "bg-emerald-50 border-emerald-200 text-emerald-900"
                : "bg-red-50 border-red-200 text-red-900"
            }`}
          >
            <div className="flex items-center space-x-2.5">
              {statusMessage.type === "success" ? (
                <CheckCircle2 className="w-5 h-5 text-emerald-700 shrink-0" />
              ) : (
                <AlertCircle className="w-5 h-5 text-red-700 shrink-0" />
              )}
              <span>{statusMessage.text}</span>
            </div>
            <button
              onClick={() => setStatusMessage(null)}
              className="text-xs font-semibold hover:underline px-2 py-1"
            >
              {t.common.close}
            </button>
          </div>
        )}

        {/* ── TAB 1: TERMINAL KASIR & TRANSAKSI ── */}
        {activeTab === "operations" && (
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
            <div className="lg:col-span-2 space-y-6">
              <Card>
                <CardHeader>
                  <div>
                    <CardTitle>
                      <Wallet className="w-5 h-5 text-slate-900" />
                      <span>{t.operations.title}</span>
                    </CardTitle>
                    <CardDescription>{t.operations.subtitle}</CardDescription>
                  </div>

                  {/* Transaction Type Buttons */}
                  <div className="flex bg-slate-100 p-1 rounded-lg border border-slate-200">
                    <button
                      onClick={() => setTrxType("deposit")}
                      className={`px-3 py-1.5 rounded-md text-xs font-semibold transition-all ${
                        trxType === "deposit"
                          ? "bg-emerald-700 text-white shadow-xs"
                          : "text-slate-600 hover:text-slate-900"
                      }`}
                    >
                      {t.operations.deposit}
                    </button>
                    <button
                      onClick={() => setTrxType("withdraw")}
                      className={`px-3 py-1.5 rounded-md text-xs font-semibold transition-all ${
                        trxType === "withdraw"
                          ? "bg-amber-600 text-white shadow-xs"
                          : "text-slate-600 hover:text-slate-900"
                      }`}
                    >
                      {t.operations.withdraw}
                    </button>
                    <button
                      onClick={() => setTrxType("transfer")}
                      className={`px-3 py-1.5 rounded-md text-xs font-semibold transition-all ${
                        trxType === "transfer"
                          ? "bg-slate-900 text-white shadow-xs"
                          : "text-slate-600 hover:text-slate-900"
                      }`}
                    >
                      {t.operations.transfer}
                    </button>
                  </div>
                </CardHeader>

                <CardContent>
                  <form
                    onSubmit={(e) => {
                      e.preventDefault();
                      setStatusMessage({ type: "success", text: t.operations.successMsg });
                    }}
                    className="space-y-4"
                  >
                    {trxType !== "deposit" && (
                      <Input
                        label={t.operations.sourceAcc}
                        value={sourceAcc}
                        onChange={(e) => setSourceAcc(e.target.value)}
                        isMono
                      />
                    )}

                    {trxType !== "withdraw" && (
                      <Input
                        label={t.operations.destAcc}
                        value={destAcc}
                        onChange={(e) => setDestAcc(e.target.value)}
                        isMono
                      />
                    )}

                    <CurrencyInput
                      label={t.operations.amount}
                      value={amount}
                      onChange={(val) => setAmount(val)}
                    />

                    <Input
                      label={t.operations.description}
                      value={description}
                      onChange={(e) => setDescription(e.target.value)}
                    />

                    <Button type="submit" variant="emerald" size="lg" className="w-full">
                      <CheckCircle2 className="w-4 h-4" />
                      <span>{t.operations.processBtn}</span>
                    </Button>
                  </form>
                </CardContent>
              </Card>
            </div>

            {/* Side Rules Card */}
            <div className="space-y-6">
              <Card>
                <CardHeader>
                  <CardTitle>
                    <ShieldCheck className="w-5 h-5 text-blue-800" />
                    <span>{t.operations.rulesTitle}</span>
                  </CardTitle>
                </CardHeader>
                <CardContent className="space-y-3">
                  <div className="p-3.5 rounded-lg bg-slate-50 border border-slate-200 space-y-1">
                    <div className="font-semibold text-slate-900 text-xs flex items-center gap-1">
                      <span>✓ {t.operations.rule1Title}</span>
                    </div>
                    <p className="text-[11px] text-slate-600 leading-relaxed">
                      {t.operations.rule1Desc}
                    </p>
                  </div>

                  <div className="p-3.5 rounded-lg bg-slate-50 border border-slate-200 space-y-1">
                    <div className="font-semibold text-slate-900 text-xs flex items-center gap-1">
                      <span>✓ {t.operations.rule2Title}</span>
                    </div>
                    <p className="text-[11px] text-slate-600 leading-relaxed">
                      {t.operations.rule2Desc}
                    </p>
                  </div>
                </CardContent>
              </Card>
            </div>
          </div>
        )}

        {/* ── TAB 2: KREDIT & PEMBIAYAAN BPR/BMT ── */}
        {activeTab === "loans" && (
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
            <div className="lg:col-span-2 space-y-6">
              <Card>
                <CardHeader>
                  <div>
                    <CardTitle>
                      <Calculator className="w-5 h-5 text-slate-900" />
                      <span>{t.loans.title}</span>
                    </CardTitle>
                    <CardDescription>{t.loans.subtitle}</CardDescription>
                  </div>

                  <div className="flex bg-slate-100 p-1 rounded-lg border border-slate-200">
                    <button
                      onClick={() => setLoanProduct("MURABAHAH")}
                      className={`px-3 py-1.5 rounded-md text-xs font-semibold transition-all ${
                        loanProduct === "MURABAHAH"
                          ? "bg-emerald-700 text-white shadow-xs"
                          : "text-slate-600 hover:text-slate-900"
                      }`}
                    >
                      {t.loans.bmtMurabahah}
                    </button>
                    <button
                      onClick={() => setLoanProduct("FLAT")}
                      className={`px-3 py-1.5 rounded-md text-xs font-semibold transition-all ${
                        loanProduct === "FLAT"
                          ? "bg-slate-900 text-white shadow-xs"
                          : "text-slate-600 hover:text-slate-900"
                      }`}
                    >
                      {t.loans.bprFlat}
                    </button>
                  </div>
                </CardHeader>

                <CardContent className="space-y-4">
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <CurrencyInput
                      label={t.loans.principal}
                      value={loanAmount}
                      onChange={(val) => setLoanAmount(val)}
                    />

                    <Input
                      label={t.loans.term}
                      type="number"
                      value={loanTerm}
                      onChange={(e) => setLoanTerm(Number(e.target.value))}
                      isMono
                    />

                    <Input
                      label={t.loans.rateOrMargin}
                      type="number"
                      step="0.1"
                      value={rateOrMargin}
                      onChange={(e) => setRateOrMargin(Number(e.target.value))}
                      isMono
                    />

                    <Input
                      label={t.loans.accountNumber}
                      defaultValue="102601020001"
                      isMono
                    />
                  </div>

                  <Button
                    onClick={() => setStatusMessage({ type: "success", text: t.loans.submittedMsg })}
                    variant="primary"
                    size="lg"
                    className="w-full"
                  >
                    <FileCheck className="w-4 h-4" />
                    <span>{t.loans.submitBtn}</span>
                  </Button>
                </CardContent>
              </Card>
            </div>

            {/* Loan Simulation Preview */}
            <div className="space-y-6">
              <Card>
                <CardHeader>
                  <CardTitle>
                    <TrendingUp className="w-5 h-5 text-emerald-700" />
                    <span>{t.loans.simTitle}</span>
                  </CardTitle>
                </CardHeader>
                <CardContent className="space-y-3">
                  <div className="p-4 rounded-lg bg-slate-50 border border-slate-200 text-xs space-y-2.5">
                    <div className="flex justify-between items-center border-b border-slate-200 pb-2">
                      <span className="text-slate-600 font-medium">{t.loans.totalPrincipal}:</span>
                      <span className="font-mono font-bold text-slate-900">
                        Rp {loanAmount.toLocaleString("id-ID")}
                      </span>
                    </div>

                    <div className="flex justify-between items-center border-b border-slate-200 pb-2">
                      <span className="text-slate-600 font-medium">{t.loans.totalMargin}:</span>
                      <span className="font-mono font-bold text-emerald-700">
                        Rp {sim.totalInterestOrMargin.toLocaleString("id-ID")}
                      </span>
                    </div>

                    <div className="flex justify-between items-center border-b border-slate-200 pb-2">
                      <span className="text-slate-600 font-medium">{t.loans.totalPayable}:</span>
                      <span className="font-mono font-bold text-slate-900">
                        Rp {sim.totalPayable.toLocaleString("id-ID")}
                      </span>
                    </div>

                    <div className="flex justify-between items-center bg-blue-50 border border-blue-200 p-3 rounded-lg mt-3">
                      <span className="font-bold text-blue-900">{t.loans.monthlyInstallment}:</span>
                      <span className="font-mono font-bold text-blue-900 text-base">
                        Rp {Math.round(sim.monthlyInstallment).toLocaleString("id-ID")}
                      </span>
                    </div>
                  </div>
                </CardContent>
              </Card>
            </div>
          </div>
        )}

        {/* ── TAB 3: MAKER-CHECKER QUEUE ── */}
        {activeTab === "approvals" && (
          <Card>
            <CardHeader>
              <div>
                <CardTitle>
                  <ShieldCheck className="w-5 h-5 text-amber-600" />
                  <span>{t.approvals.title}</span>
                </CardTitle>
                <CardDescription>{t.approvals.subtitle}</CardDescription>
              </div>
            </CardHeader>
            <CardContent>
              {pendingApprovals.length === 0 ? (
                <div className="text-center py-12 text-slate-500 text-xs font-medium">
                  🎉 {t.approvals.emptyQueue}
                </div>
              ) : (
                <div className="space-y-3">
                  {pendingApprovals.map((item) => (
                    <div
                      key={item.id}
                      className="p-4 bg-white border border-slate-200 rounded-lg flex flex-col md:flex-row md:items-center justify-between gap-4 hover:border-slate-300 transition-colors shadow-xs"
                    >
                      <div className="space-y-1">
                        <div className="flex items-center space-x-2">
                          <Badge variant="warning">{item.id}</Badge>
                          <Badge variant="outline">{item.type}</Badge>
                          <span className="text-xs text-slate-500 flex items-center gap-1 font-mono">
                            <Clock className="w-3 h-3 text-slate-400" />
                            {t.approvals.initiatedBy}: {item.maker} ({item.time})
                          </span>
                        </div>
                        <div className="text-xs font-semibold text-slate-900">{item.desc}</div>
                        <div className="text-xs font-mono text-emerald-700 font-bold">
                          {t.approvals.nominal}: Rp {item.amount.toLocaleString("id-ID")}
                        </div>
                      </div>

                      <div className="flex items-center space-x-2 shrink-0">
                        <Button
                          onClick={() => handleApprove(item.id)}
                          variant="emerald"
                          size="sm"
                        >
                          {t.approvals.approveAndPost}
                        </Button>
                        <Button
                          onClick={() => handleReject(item.id)}
                          variant="outline"
                          size="sm"
                          className="hover:bg-red-50 hover:text-red-700 hover:border-red-200"
                        >
                          {t.approvals.reject}
                        </Button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        )}

        {/* ── TAB 4: GENERAL LEDGER JURNAL ── */}
        {activeTab === "ledger" && (
          <Card>
            <CardHeader>
              <div>
                <CardTitle>
                  <BookOpenCheck className="w-5 h-5 text-slate-900" />
                  <span>{t.ledger.title}</span>
                </CardTitle>
                <CardDescription>{t.ledger.subtitle}</CardDescription>
              </div>
            </CardHeader>
            <CardContent>
              <div className="overflow-x-auto rounded-lg border border-slate-200 bg-white">
                <table className="w-full text-left text-xs">
                  <thead className="bg-slate-100 text-slate-700 uppercase font-sans font-semibold border-b border-slate-200">
                    <tr>
                      <th className="py-2.5 px-4">{t.ledger.refNumber}</th>
                      <th className="py-2.5 px-4">{t.ledger.time}</th>
                      <th className="py-2.5 px-4">{t.ledger.desc}</th>
                      <th className="py-2.5 px-4">{t.ledger.lines}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-200 font-mono">
                    {journals.map((j, idx) => (
                      <tr key={idx} className="hover:bg-slate-50/80 transition-colors">
                        <td className="py-3 px-4 font-bold text-blue-900">{j.reference_number}</td>
                        <td className="py-3 px-4 text-slate-500">{j.posted_at}</td>
                        <td className="py-3 px-4 text-slate-900 font-sans">{j.description}</td>
                        <td className="py-3 px-4 space-y-1">
                          {j.lines.map((l, i) => (
                            <div key={i} className="flex justify-between gap-4 text-[11px]">
                              <span
                                className={
                                  l.direction === "DEBIT"
                                    ? "text-emerald-700 font-bold"
                                    : "text-amber-700 font-bold pl-3"
                                }
                              >
                                [{l.direction}] {l.account}
                              </span>
                              <span className="font-bold">Rp {l.amount.toLocaleString("id-ID")}</span>
                            </div>
                          ))}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </CardContent>
          </Card>
        )}

        {/* ── TAB 5: DIREKTORI REKENING ── */}
        {activeTab === "accounts" && (
          <Card>
            <CardHeader>
              <div>
                <CardTitle>
                  <Users className="w-5 h-5 text-slate-900" />
                  <span>{t.accounts.title}</span>
                </CardTitle>
                <CardDescription>{t.accounts.subtitle}</CardDescription>
              </div>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {accounts.map((acc, idx) => (
                  <div
                    key={idx}
                    className="p-4 rounded-lg bg-white border border-slate-200 space-y-2 hover:border-slate-300 transition-colors shadow-xs"
                  >
                    <div className="flex justify-between items-center">
                      <span className="font-mono font-bold text-blue-900 text-xs">
                        {acc.account_number}
                      </span>
                      <Badge variant="default" size="sm">
                        {acc.account_type}
                      </Badge>
                    </div>
                    <div className="text-xs font-semibold text-slate-900">{acc.customer_name}</div>
                    <div className="text-xs font-mono text-emerald-700 font-bold text-right pt-2 border-t border-slate-200">
                      {t.accounts.balance}: Rp {acc.balance.toLocaleString("id-ID")}
                    </div>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        )}
      </main>
    </div>
  );
}
