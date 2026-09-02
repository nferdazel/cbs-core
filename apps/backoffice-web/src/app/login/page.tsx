"use client";

import React, { useState } from "react";
import { useRouter } from "next/navigation";

export default function LoginPage() {
  const router = useRouter();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setLoading(true);

    try {
      const apiHost = process.env.NEXT_PUBLIC_API_URL || "https://api.qouver.com/cbs";
      const res = await fetch(`${apiHost}/api/v1/auth/login`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username, password }),
      });

      const data = await res.json();
      if (!res.ok || !data.success) {
        throw new Error(data.error || "Login gagal, silakan periksa username & password.");
      }

      // Save tokens & user profile to localStorage
      localStorage.setItem("cbs_access_token", data.data.access_token);
      localStorage.setItem("cbs_refresh_token", data.data.refresh_token);
      localStorage.setItem("cbs_user", JSON.stringify(data.data.user));

      router.push("/");
    } catch (err: any) {
      setError(err.message || "Terjadi kesalahan jaringan.");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex items-center justify-center p-4">
      <div className="w-full max-w-md bg-slate-900 border border-slate-800 rounded-xl p-8 shadow-2xl space-y-6">
        <div className="text-center space-y-2">
          <div className="inline-flex items-center justify-center w-12 h-12 rounded-xl bg-blue-600/20 text-blue-400 font-bold text-xl mb-2">
            CBS
          </div>
          <h1 className="text-2xl font-bold text-slate-100">Portal Backoffice CBS</h1>
          <p className="text-sm text-slate-400">Core Banking System — BPR/BPRS & BMT/Koperasi</p>
        </div>

        {error && (
          <div className="bg-red-500/10 border border-red-500/30 text-red-400 p-3 rounded-lg text-sm">
            ⚠️ {error}
          </div>
        )}

        <form onSubmit={handleLogin} className="space-y-4">
          <div>
            <label className="block text-xs font-semibold uppercase tracking-wider text-slate-400 mb-1">
              Username Staff
            </label>
            <input
              type="text"
              required
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="e.g. superadmin / teller01"
              className="w-full bg-slate-800 border border-slate-700 rounded-lg px-4 py-2.5 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-blue-500 transition-colors"
            />
          </div>

          <div>
            <label className="block text-xs font-semibold uppercase tracking-wider text-slate-400 mb-1">
              Password
            </label>
            <input
              type="password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="••••••••"
              className="w-full bg-slate-800 border border-slate-700 rounded-lg px-4 py-2.5 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-blue-500 transition-colors"
            />
          </div>

          <button
            type="submit"
            disabled={loading}
            className="w-full bg-blue-600 hover:bg-blue-500 text-white font-semibold py-2.5 px-4 rounded-lg text-sm shadow-lg shadow-blue-600/30 transition-all disabled:opacity-50"
          >
            {loading ? "Memproses Login..." : "Masuk ke Backoffice"}
          </button>
        </form>

        <div className="border-t border-slate-800 pt-4 text-center">
          <p className="text-xs text-slate-500">
            Akun default demo: <span className="text-slate-300 font-mono">superadmin</span> / <span className="text-slate-300 font-mono">Admin@CBS2026!</span>
          </p>
        </div>
      </div>
    </div>
  );
}
