import { AppShell } from "@/components/layout/AppShell";

/** Layout dashboard: semua halaman di grup ini melewati guard sesi AppShell. */
export default function DashboardLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return <AppShell>{children}</AppShell>;
}
