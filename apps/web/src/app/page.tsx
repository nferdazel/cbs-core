"use client";

import React, { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
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
  AlertCircle,
  LogOut,
  UserCheck,
  Calculator,
  FileCheck,
  Landmark,
  BadgeCheck,
  TrendingUp,
  FileText
} from "lucide-react";

export default function Dashboard() {
  const router = useRouter();
  const [user, setUser] = useState<any>(null);
  const [activeTab, setActiveTab] = useState<"operations" | "loans" | "approvals" | "ledger" | "accounts">("operations");
  
  // Operations Form State
  const [trxType, setTrxType] = useState<"deposit" | "withdraw" | "transfer">("deposit");
  const [amount, setAmount] = useState<string>("5000000");
  const [sourceAcc, setSourceAcc] = useState<string>("102601020001");
  const [destAcc, setDestAcc] = useState<string>("102601020002");
  const [description, setDescription] = useState<string>("Setoran Tunai via Kasir Teller");
  const [statusMessage, setStatusMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);
  const [loading, setLoading] = useState<boolean>(false);

  // Loan Calculator State (BPR Kredit & BMT Pembiayaan)
  const [loanProduct, setLoanProduct] = useState<"FLAT" | "MURABAHAH">("MURABAHAH");
  const [loanAmount, setLoanAmount] = useState<number>(50000000);
  const [loanTerm, setLoanTerm] = useState<number>(12);
  const [rateOrMargin, setRateOrMargin] = useState<number>(12.0); // 12% p.a. or 12% total margin

  useEffect(() => {
    // Check logged in staff user
    const storedUser = localStorage.getItem("cbs_user");
    if (storedUser) {
      try {
        setUser(JSON.parse(storedUser));
      } catch (e) {}
    } else {
      // Default demo user profile for initial view
      setUser({
        username: "superadmin",
        full_name: "System Super Administrator",
        role: "SUPERADMIN",
        branch_code: "HO"
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
    { account_number: "GL-VAULT-001", customer_name: "KAS BRANKAS UTAMA", account_type: "INTERNAL_GL", currency: "IDR", balance: "10,000,000,000.00", status: "ACTIVE" },
    { account_number: "102601020001", customer_name: "Budi Santoso (CIF-001)", account_type: "SAVINGS", currency: "IDR", balance: "25,500,000.00", status: "ACTIVE" },
    { account_number: "102601020002", customer_name: "Siti Rahmawati (CIF-002)", account_type: "SAVINGS", currency: "IDR", balance: "14,200,000.00", status: "ACTIVE" },
    { account_number: "GL-FEE-INCOME-001", customer_name: "PENDAPATAN ADM & MARGIN", account_type: "INTERNAL_GL", currency: "IDR", balance: "85,400,000.00", status: "ACTIVE" }
  ];

  // Mock pending maker-checker approvals
  const [pendingApprovals, setPendingApprovals] = useState([
    { id: "MC-901", maker: "Teller-01", type: "WITHDRAWAL", amount: "75,000,000.00", desc: "Penarikan Tunai Rekening #102601020001", status: "PENDING", time: "10 menit yang lalu" },
    { id: "MC-902", maker: "AO-Budi", type: "LOAN_ORIGINATION", amount: "150,000,000.00", desc: "Pembiayaan Murabahah Toko Sembako (CIF-002)", status: "PENDING", time: "25 menit yang lalu" }
  ]);

  // Mock journal entries
  const journals = [
    {
      reference_number: "DISB-PMB-2026-00012",
      transaction_type: "LOAN_DISBURSEMENT",
      description: "Pencairan Pembiayaan Murabahah Modal Usaha",
      posted_at: "2026-09-02 11:30:15",
      status: "POSTED",
      lines: [
        { account: "GL-LOAN-RECEIVABLE", direction: "DEBIT", amount: "50,000,000.00" },
        { account: "102601020001", direction: "CREDIT", amount: "50,000,000.00" }
      ]
    },
    {
      reference_number: "DEP-20260902-881245",
      transaction_type: "DEPOSIT",
      description: "Setoran Tunai Simpanan Nasabah Budi Santoso",
      posted_at: "2026-09-02 10:05:12",
      status: "POSTED",
      lines: [
        { account: "GL-VAULT-001", direction: "DEBIT", amount: "5,000,000.00" },
        { account: "102601020001", direction: "CREDIT", amount: "5,000,000.00" }
      ]
    }
  ];

  // Calculate Loan Simulation
  const calculateLoanSim = () => {
    let totalInterestOrMargin = 0;
    if (loanProduct === "FLAT") {
      totalInterestOrMargin = loanAmount * (rateOrMargin / 100) * (loanTerm / 12);
    } else {
      // Murabahah fixed margin
      totalInterestOrMargin = loanAmount * (rateOrMargin / 100);
    }
    const totalPayable = loanAmount + totalInterestOrMargin;
    const monthlyInstallment = totalPayable / loanTerm;

    return {
      totalInterestOrMargin,
      totalPayable,
      monthlyInstallment,
      principalMonthly: loanAmount / loanTerm,
      marginMonthly: totalInterestOrMargin / loanTerm
    };
  };

  const sim = calculateLoanSim();

  const handleApprove = (id: string) => {
    setPendingApprovals(pendingApprovals.filter(a => a.id !== id));
    setStatusMessage({ type: "success", text: `Permintaan ${id} telah disetujui & jurnal diposting secara atomic.` });
  };

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 font-sans selection:bg-blue-600 selection:text-white">
      {/* ── Top Navigation Bar ── */}
      <header className="sticky top-0 z-50 bg-slate-900/80 backdrop-blur-md border-b border-slate-800/80 px-6 py-3.5">
        <div className="max-w-7xl mx-auto flex items-center justify-between">
          <div className="flex items-center space-x-3">
            <div className="bg-gradient-to-tr from-blue-600 to-indigo-500 p-2 rounded-xl shadow-lg shadow-blue-500/20">
              <Building2 className="w-5 h-5 text-white" />
            </div>
            <div>
              <div className="flex items-center space-x-2">
                <span className="font-bold text-lg text-slate-100 tracking-tight">CBS Core Backoffice</span>
                <span className="bg-blue-500/10 border border-blue-500/30 text-blue-400 text-[10px] font-semibold px-2 py-0.5 rounded-full uppercase">
                  BPR / BMT Ready
                </span>
              </div>
              <p className="text-xs text-slate-400">Platform Core Banking & Akuntansi Double-Entry Multi-Akad</p>
            </div>
          </div>

          {/* User Badge & Logout */}
          <div className="flex items-center space-x-4">
            <div className="flex items-center space-x-3 bg-slate-800/60 border border-slate-700/60 rounded-xl px-3.5 py-1.5">
              <div className="w-8 h-8 rounded-lg bg-blue-600/20 border border-blue-500/30 flex items-center justify-center text-blue-400 font-bold text-xs">
                {user?.role?.slice(0, 2) || "SA"}
              </div>
              <div className="text-left">
                <div className="text-xs font-semibold text-slate-200 flex items-center space-x-1">
                  <span>{user?.full_name || "Staff User"}</span>
                  <BadgeCheck className="w-3.5 h-3.5 text-blue-400" />
                </div>
                <div className="text-[10px] text-slate-400 flex items-center space-x-2">
                  <span className="text-emerald-400 font-medium">● Role: {user?.role || "SUPERADMIN"}</span>
                  <span>| Cabang: {user?.branch_code || "HO"}</span>
                </div>
              </div>
            </div>

            <button
              onClick={handleLogout}
              className="p-2 rounded-xl bg-slate-800/60 hover:bg-red-500/10 border border-slate-700/60 hover:border-red-500/30 text-slate-400 hover:text-red-400 transition-all text-xs flex items-center space-x-1.5"
              title="Logout"
            >
              <LogOut className="w-4 h-4" />
              <span className="hidden md:inline">Keluar</span>
            </button>
          </div>
        </div>
      </header>

      {/* ── Main Container ── */}
      <main className="max-w-7xl mx-auto px-6 py-8 space-y-6">
        {/* Status Notification */}
        {statusMessage && (
          <div
            className={`p-4 rounded-xl border flex items-center justify-between text-sm shadow-xl animate-in fade-in ${
              statusMessage.type === "success"
                ? "bg-emerald-500/10 border-emerald-500/30 text-emerald-300"
                : "bg-red-500/10 border-red-500/30 text-red-300"
            }`}
          >
            <div className="flex items-center space-x-3">
              {statusMessage.type === "success" ? <CheckCircle2 className="w-5 h-5 text-emerald-400" /> : <AlertCircle className="w-5 h-5 text-red-400" />}
              <span>{statusMessage.text}</span>
            </div>
            <button onClick={() => setStatusMessage(null)} className="text-xs hover:underline">Tutup</button>
          </div>
        )}

        {/* ── Navigation Tabs ── */}
        <div className="flex space-x-2 border-b border-slate-800 pb-1 overflow-x-auto">
          <button
            onClick={() => setActiveTab("operations")}
            className={`flex items-center space-x-2 px-4 py-2.5 rounded-xl font-medium text-sm transition-all ${
              activeTab === "operations"
                ? "bg-blue-600 text-white shadow-lg shadow-blue-600/30"
                : "text-slate-400 hover:text-slate-200 hover:bg-slate-900"
            }`}
          >
            <Wallet className="w-4 h-4" />
            <span>Kasir & Transaksi</span>
          </button>

          <button
            onClick={() => setActiveTab("loans")}
            className={`flex items-center space-x-2 px-4 py-2.5 rounded-xl font-medium text-sm transition-all ${
              activeTab === "loans"
                ? "bg-blue-600 text-white shadow-lg shadow-blue-600/30"
                : "text-slate-400 hover:text-slate-200 hover:bg-slate-900"
            }`}
          >
            <Landmark className="w-4 h-4" />
            <span>Kredit & Pembiayaan BPR/BMT</span>
          </button>

          <button
            onClick={() => setActiveTab("approvals")}
            className={`flex items-center space-x-2 px-4 py-2.5 rounded-xl font-medium text-sm transition-all ${
              activeTab === "approvals"
                ? "bg-blue-600 text-white shadow-lg shadow-blue-600/30"
                : "text-slate-400 hover:text-slate-200 hover:bg-slate-900"
            }`}
          >
            <ShieldCheck className="w-4 h-4" />
            <span>Maker-Checker Queue</span>
            {pendingApprovals.length > 0 && (
              <span className="bg-amber-500/20 text-amber-300 text-xs px-2 py-0.5 rounded-full border border-amber-500/30 font-bold">
                {pendingApprovals.length}
              </span>
            )}
          </button>

          <button
            onClick={() => setActiveTab("ledger")}
            className={`flex items-center space-x-2 px-4 py-2.5 rounded-xl font-medium text-sm transition-all ${
              activeTab === "ledger"
                ? "bg-blue-600 text-white shadow-lg shadow-blue-600/30"
                : "text-slate-400 hover:text-slate-200 hover:bg-slate-900"
            }`}
          >
            <BookOpenCheck className="w-4 h-4" />
            <span>General Ledger Jurnal</span>
          </button>

          <button
            onClick={() => setActiveTab("accounts")}
            className={`flex items-center space-x-2 px-4 py-2.5 rounded-xl font-medium text-sm transition-all ${
              activeTab === "accounts"
                ? "bg-blue-600 text-white shadow-lg shadow-blue-600/30"
                : "text-slate-400 hover:text-slate-200 hover:bg-slate-900"
            }`}
          >
            <Users className="w-4 h-4" />
            <span>Direktori Rekening</span>
          </button>
        </div>

        {/* ── TAB 1: KASIR & TRANSAKSI ── */}
        {activeTab === "operations" && (
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
            <div className="lg:col-span-2 bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-xl space-y-6">
              <div className="flex items-center justify-between border-b border-slate-800 pb-4">
                <div>
                  <h2 className="text-lg font-bold text-slate-100">Terminal Kasir & Teller</h2>
                  <p className="text-xs text-slate-400">Eksekusi Setoran, Penarikan Tunai & Pemindahbukuan</p>
                </div>
                <div className="flex bg-slate-950 p-1 rounded-xl border border-slate-800">
                  <button
                    onClick={() => setTrxType("deposit")}
                    className={`px-3 py-1.5 rounded-lg text-xs font-semibold transition-all ${
                      trxType === "deposit" ? "bg-emerald-600 text-white shadow" : "text-slate-400 hover:text-slate-200"
                    }`}
                  >
                    Setoran Tunai
                  </button>
                  <button
                    onClick={() => setTrxType("withdraw")}
                    className={`px-3 py-1.5 rounded-lg text-xs font-semibold transition-all ${
                      trxType === "withdraw" ? "bg-amber-600 text-white shadow" : "text-slate-400 hover:text-slate-200"
                    }`}
                  >
                    Penarikan Tunai
                  </button>
                  <button
                    onClick={() => setTrxType("transfer")}
                    className={`px-3 py-1.5 rounded-lg text-xs font-semibold transition-all ${
                      trxType === "transfer" ? "bg-blue-600 text-white shadow" : "text-slate-400 hover:text-slate-200"
                    }`}
                  >
                    Transfer Internal
                  </button>
                </div>
              </div>

              <form onSubmit={(e) => { e.preventDefault(); setStatusMessage({ type: "success", text: "Transaksi diposting ke ledger secara atomic." }); }} className="space-y-4">
                {trxType !== "deposit" && (
                  <div>
                    <label className="block text-xs font-semibold uppercase tracking-wider text-slate-400 mb-1">
                      Rekening Sumber / Debet
                    </label>
                    <input
                      type="text"
                      value={sourceAcc}
                      onChange={(e) => setSourceAcc(e.target.value)}
                      className="w-full bg-slate-950 border border-slate-800 rounded-xl px-4 py-2.5 text-sm text-slate-100 font-mono focus:border-blue-500 outline-none"
                    />
                  </div>
                )}

                {trxType !== "withdraw" && (
                  <div>
                    <label className="block text-xs font-semibold uppercase tracking-wider text-slate-400 mb-1">
                      Rekening Tujuan / Kredit
                    </label>
                    <input
                      type="text"
                      value={destAcc}
                      onChange={(e) => setDestAcc(e.target.value)}
                      className="w-full bg-slate-950 border border-slate-800 rounded-xl px-4 py-2.5 text-sm text-slate-100 font-mono focus:border-blue-500 outline-none"
                    />
                  </div>
                )}

                <div>
                  <label className="block text-xs font-semibold uppercase tracking-wider text-slate-400 mb-1">
                    Nominal Transaksi (IDR)
                  </label>
                  <input
                    type="number"
                    value={amount}
                    onChange={(e) => setAmount(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-xl px-4 py-2.5 text-lg font-bold text-emerald-400 font-mono focus:border-blue-500 outline-none"
                  />
                </div>

                <div>
                  <label className="block text-xs font-semibold uppercase tracking-wider text-slate-400 mb-1">
                    Keterangan Transaksi
                  </label>
                  <input
                    type="text"
                    value={description}
                    onChange={(e) => setDescription(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-xl px-4 py-2.5 text-sm text-slate-100 focus:border-blue-500 outline-none"
                  />
                </div>

                <button
                  type="submit"
                  className="w-full bg-gradient-to-r from-blue-600 to-indigo-600 hover:from-blue-500 hover:to-indigo-500 text-white font-bold py-3 px-4 rounded-xl text-sm shadow-xl shadow-blue-500/20 transition-all flex items-center justify-center space-x-2"
                >
                  <CheckCircle2 className="w-4 h-4" />
                  <span>Proses & Posting Jurnal Double-Entry</span>
                </button>
              </form>
            </div>

            {/* Quick Ledger Rules Card */}
            <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-xl space-y-4">
              <h3 className="font-bold text-slate-200 text-sm flex items-center space-x-2">
                <ShieldCheck className="w-4 h-4 text-blue-400" />
                <span>Aturan Keuangan Atomic</span>
              </h3>
              <div className="space-y-3 text-xs text-slate-400">
                <div className="p-3 rounded-xl bg-slate-950 border border-slate-800">
                  <div className="font-semibold text-slate-200 mb-1">✓ Single DB Transaction</div>
                  <span>Setoran tunai mendebet Kas Brankas & mengkredit Rekening Nasabah secara simultan.</span>
                </div>
                <div className="p-3 rounded-xl bg-slate-950 border border-slate-800">
                  <div className="font-semibold text-slate-200 mb-1">✓ Maker-Checker Threshold</div>
                  <span>Transaksi penarikan/transfer di atas Rp 50 Juta wajib diajukan ke Supervisor.</span>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* ── TAB 2: KREDIT & PEMBIAYAAN BPR/BMT ── */}
        {activeTab === "loans" && (
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
            {/* Calculator & Form */}
            <div className="lg:col-span-2 bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-xl space-y-6">
              <div className="flex items-center justify-between border-b border-slate-800 pb-4">
                <div>
                  <h2 className="text-lg font-bold text-slate-100 flex items-center space-x-2">
                    <Calculator className="w-5 h-5 text-blue-400" />
                    <span>Inisiasi Kredit & Pembiayaan AO</span>
                  </h2>
                  <p className="text-xs text-slate-400">Kalkulator & Pengajuan Kredit BPR / Pembiayaan Murabahah BMT</p>
                </div>

                <div className="flex bg-slate-950 p-1 rounded-xl border border-slate-800">
                  <button
                    onClick={() => setLoanProduct("MURABAHAH")}
                    className={`px-3 py-1.5 rounded-lg text-xs font-semibold transition-all ${
                      loanProduct === "MURABAHAH" ? "bg-emerald-600 text-white shadow" : "text-slate-400"
                    }`}
                  >
                    BMT Murabahah
                  </button>
                  <button
                    onClick={() => setLoanProduct("FLAT")}
                    className={`px-3 py-1.5 rounded-lg text-xs font-semibold transition-all ${
                      loanProduct === "FLAT" ? "bg-blue-600 text-white shadow" : "text-slate-400"
                    }`}
                  >
                    BPR Flat Rate
                  </button>
                </div>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-semibold uppercase tracking-wider text-slate-400 mb-1">
                    Plafond / Nilai Pokok (IDR)
                  </label>
                  <input
                    type="number"
                    value={loanAmount}
                    onChange={(e) => setLoanAmount(Number(e.target.value))}
                    className="w-full bg-slate-950 border border-slate-800 rounded-xl px-4 py-2.5 text-sm text-slate-100 font-mono font-bold focus:border-blue-500 outline-none"
                  />
                </div>

                <div>
                  <label className="block text-xs font-semibold uppercase tracking-wider text-slate-400 mb-1">
                    Jangka Waktu (Bulan)
                  </label>
                  <input
                    type="number"
                    value={loanTerm}
                    onChange={(e) => setLoanTerm(Number(e.target.value))}
                    className="w-full bg-slate-950 border border-slate-800 rounded-xl px-4 py-2.5 text-sm text-slate-100 font-mono focus:border-blue-500 outline-none"
                  />
                </div>

                <div>
                  <label className="block text-xs font-semibold uppercase tracking-wider text-slate-400 mb-1">
                    {loanProduct === "MURABAHAH" ? "Margin Keuntungan (%)" : "Suku Bunga p.a. (%)"}
                  </label>
                  <input
                    type="number"
                    step="0.1"
                    value={rateOrMargin}
                    onChange={(e) => setRateOrMargin(Number(e.target.value))}
                    className="w-full bg-slate-950 border border-slate-800 rounded-xl px-4 py-2.5 text-sm text-slate-100 font-mono focus:border-blue-500 outline-none"
                  />
                </div>

                <div>
                  <label className="block text-xs font-semibold uppercase tracking-wider text-slate-400 mb-1">
                    Rekening Pencairan / Nasabah
                  </label>
                  <input
                    type="text"
                    defaultValue="102601020001"
                    className="w-full bg-slate-950 border border-slate-800 rounded-xl px-4 py-2.5 text-sm text-slate-100 font-mono focus:border-blue-500 outline-none"
                  />
                </div>
              </div>

              <button
                onClick={() => setStatusMessage({ type: "success", text: "Pengajuan Pembiayaan disubmit oleh AO -> Masuk antrean Komite Kredit." })}
                className="w-full bg-blue-600 hover:bg-blue-500 text-white font-bold py-3 px-4 rounded-xl text-sm shadow-xl shadow-blue-500/20 transition-all flex items-center justify-center space-x-2"
              >
                <FileCheck className="w-4 h-4" />
                <span>Ajukan Pembiayaan (Submit ke Supervisor)</span>
              </button>
            </div>

            {/* Simulation Preview Card */}
            <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-xl space-y-4">
              <h3 className="font-bold text-slate-200 text-sm flex items-center space-x-2">
                <TrendingUp className="w-4 h-4 text-emerald-400" />
                <span>Hasil Simulasi Angsuran</span>
              </h3>

              <div className="space-y-3 bg-slate-950 p-4 rounded-xl border border-slate-800 text-xs">
                <div className="flex justify-between py-1 border-b border-slate-800/60">
                  <span className="text-slate-400">Total Pokok:</span>
                  <span className="font-mono text-slate-200">Rp {loanAmount.toLocaleString("id-ID")}</span>
                </div>
                <div className="flex justify-between py-1 border-b border-slate-800/60">
                  <span className="text-slate-400">{loanProduct === "MURABAHAH" ? "Total Margin BMT:" : "Total Bunga BPR:"}</span>
                  <span className="font-mono text-emerald-400 font-semibold">Rp {sim.totalInterestOrMargin.toLocaleString("id-ID")}</span>
                </div>
                <div className="flex justify-between py-1 border-b border-slate-800/60">
                  <span className="text-slate-400">Total Piutang / Harus Dibayar:</span>
                  <span className="font-mono text-slate-100 font-bold">Rp {sim.totalPayable.toLocaleString("id-ID")}</span>
                </div>
                <div className="flex justify-between py-2 bg-blue-500/10 border border-blue-500/30 px-3 rounded-lg mt-2">
                  <span className="font-bold text-blue-300">Angsuran / Bulan:</span>
                  <span className="font-mono font-bold text-blue-400 text-sm">Rp {Math.round(sim.monthlyInstallment).toLocaleString("id-ID")}</span>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* ── TAB 3: MAKER-CHECKER QUEUE ── */}
        {activeTab === "approvals" && (
          <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-xl space-y-4">
            <div className="flex items-center justify-between border-b border-slate-800 pb-4">
              <div>
                <h2 className="text-lg font-bold text-slate-100 flex items-center space-x-2">
                  <ShieldCheck className="w-5 h-5 text-amber-400" />
                  <span>Antrean Approval Supervisor / Checker</span>
                </h2>
                <p className="text-xs text-slate-400">Persetujuan Transaksi Nominal Besar & Inisiasi Kredit</p>
              </div>
            </div>

            {pendingApprovals.length === 0 ? (
              <div className="text-center py-12 text-slate-500 text-sm">
                🎉 Tidak ada antrean transaksi pending. Semua aman!
              </div>
            ) : (
              <div className="space-y-3">
                {pendingApprovals.map((item) => (
                  <div key={item.id} className="p-4 bg-slate-950 border border-slate-800 rounded-xl flex items-center justify-between">
                    <div className="space-y-1">
                      <div className="flex items-center space-x-2">
                        <span className="font-mono text-xs text-amber-400 font-bold">{item.id}</span>
                        <span className="bg-slate-800 text-slate-300 text-[10px] px-2 py-0.5 rounded font-semibold uppercase">{item.type}</span>
                        <span className="text-xs text-slate-500">• Inisiasi oleh: {item.maker} ({item.time})</span>
                      </div>
                      <div className="text-sm font-semibold text-slate-200">{item.desc}</div>
                      <div className="text-xs font-mono text-emerald-400 font-bold">Nominal: Rp {item.amount}</div>
                    </div>

                    <div className="flex items-center space-x-2">
                      <button
                        onClick={() => handleApprove(item.id)}
                        className="bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs px-4 py-2 rounded-lg shadow-md shadow-emerald-600/20 transition-all"
                      >
                        Setujui & Post
                      </button>
                      <button
                        onClick={() => setPendingApprovals(pendingApprovals.filter(a => a.id !== item.id))}
                        className="bg-slate-800 hover:bg-red-500/20 text-slate-300 hover:text-red-400 font-semibold text-xs px-3 py-2 rounded-lg transition-all"
                      >
                        Tolak
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}

        {/* ── TAB 4: GENERAL LEDGER JURNAL ── */}
        {activeTab === "ledger" && (
          <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-xl space-y-4">
            <h2 className="text-lg font-bold text-slate-100">Audit Trail Jurnal Keuangan Double-Entry</h2>
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs text-slate-300">
                <thead className="bg-slate-950 text-slate-400 uppercase font-mono border-b border-slate-800">
                  <tr>
                    <th className="py-3 px-4">Ref Number</th>
                    <th className="py-3 px-4">Waktu</th>
                    <th className="py-3 px-4">Keterangan</th>
                    <th className="py-3 px-4">Rincian Jurnal (Debit / Credit)</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800/60 font-mono">
                  {journals.map((j, idx) => (
                    <tr key={idx} className="hover:bg-slate-800/30 transition-colors">
                      <td className="py-3 px-4 font-bold text-blue-400">{j.reference_number}</td>
                      <td className="py-3 px-4 text-slate-400">{j.posted_at}</td>
                      <td className="py-3 px-4 text-slate-200 font-sans">{j.description}</td>
                      <td className="py-3 px-4 space-y-1">
                        {j.lines.map((l, i) => (
                          <div key={i} className="flex justify-between text-[11px]">
                            <span className={l.direction === "DEBIT" ? "text-emerald-400 font-semibold" : "text-amber-400 font-semibold pl-4"}>
                              [{l.direction}] {l.account}
                            </span>
                            <span className="font-bold">Rp {l.amount}</span>
                          </div>
                        ))}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}

        {/* ── TAB 5: DIREKTORI REKENING ── */}
        {activeTab === "accounts" && (
          <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-xl space-y-4">
            <h2 className="text-lg font-bold text-slate-100">Direktori Rekening Nasabah & GL Internal</h2>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {accounts.map((acc, idx) => (
                <div key={idx} className="p-4 rounded-xl bg-slate-950 border border-slate-800 space-y-2">
                  <div className="flex justify-between items-center">
                    <span className="font-mono font-bold text-blue-400">{acc.account_number}</span>
                    <span className="text-[10px] bg-slate-800 text-slate-300 font-bold px-2 py-0.5 rounded">{acc.account_type}</span>
                  </div>
                  <div className="text-sm font-semibold text-slate-100">{acc.customer_name}</div>
                  <div className="text-xs font-mono text-emerald-400 font-bold text-right pt-2 border-t border-slate-800">
                    Saldo: Rp {acc.balance}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </main>
    </div>
  );
}
