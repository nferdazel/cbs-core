"use client";

import React, { useState } from "react";
import { useRouter } from "next/navigation";
import { AlertCircle, Building2 } from "lucide-react";
import { useTranslation } from "@/i18n/context";
import { Button } from "@/components/ui/Button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
} from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { ApiError, request, unwrap } from "@/lib/api";
import { saveSession } from "@/lib/auth";
import type { LoginResponse } from "@/lib/types";

export default function LoginPage() {
  const router = useRouter();
  const { language, setLanguage, t } = useTranslation();

  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setLoading(true);

    try {
      const response = await request<LoginResponse>("/auth/login", {
        method: "POST",
        auth: false,
        body: { username, password },
      });
      saveSession(unwrap(response));
      router.replace("/");
    } catch (err) {
      const message =
        err instanceof ApiError ? err.message : "Terjadi kesalahan jaringan.";
      setError(message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-canvas p-4">
      <div className="w-full max-w-sm space-y-4">
        <div className="flex justify-end">
          <div className="flex items-center rounded-md border border-border bg-surface p-0.5 text-meta">
            <button
              type="button"
              onClick={() => setLanguage("id")}
              className={`rounded-sm px-2.5 py-1 transition-colors duration-fast ${
                language === "id" ? "bg-navy-700 font-medium text-white" : "text-ink-600 hover:text-ink-900"
              }`}
            >
              ID
            </button>
            <button
              type="button"
              onClick={() => setLanguage("en")}
              className={`rounded-sm px-2.5 py-1 transition-colors duration-fast ${
                language === "en" ? "bg-navy-700 font-medium text-white" : "text-ink-600 hover:text-ink-900"
              }`}
            >
              EN
            </button>
          </div>
        </div>

        <Card>
          <CardHeader className="flex flex-col items-center justify-center py-6 text-center">
            <div className="mb-2 inline-flex h-10 w-10 items-center justify-center rounded-md bg-navy-700 text-white">
              <Building2 className="h-5 w-5" aria-hidden />
            </div>
            <CardTitle>{t.login.title}</CardTitle>
            <CardDescription>{t.login.subtitle}</CardDescription>
          </CardHeader>

          <CardContent>
            {error && (
              <div
                className="flex items-center gap-2 rounded-md border border-debit-700/30 bg-debit-50 p-3 text-meta text-debit-700"
                role="alert"
              >
                <AlertCircle className="h-4 w-4 shrink-0" aria-hidden />
                <span>{error}</span>
              </div>
            )}

            <form onSubmit={handleLogin} className="space-y-4">
              <Input
                label={t.login.staffUsername}
                type="text"
                required
                autoComplete="username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
              />

              <Input
                label={t.login.password}
                type="password"
                required
                autoComplete="current-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />

              <Button type="submit" loading={loading} className="w-full">
                {loading ? t.login.processing : t.login.submitBtn}
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
