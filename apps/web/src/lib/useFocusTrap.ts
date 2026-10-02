"use client";

import { useEffect, type RefObject } from "react";

const FOCUSABLE =
  'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])';

/**
 * Jebakan fokus untuk dialog modal.
 *
 * Dua hal yang wajib ada pada dialog modal dan tidak cukup dengan autoFocus:
 *  1. Tab/Shift+Tab tidak boleh keluar ke halaman di belakang dialog.
 *  2. Saat dialog ditutup, fokus kembali ke elemen pemicu agar pengguna keyboard
 *     tidak "tersesat" di awal dokumen.
 *
 * `containerRef` menunjuk elemen pembungkus dialog; pemanggil tetap bertanggung
 * jawab merender/menyembunyikan dialog dan menangani Escape.
 */
export function useFocusTrap(
  containerRef: RefObject<HTMLElement | null>,
  active = true,
) {
  useEffect(() => {
    if (!active) return;
    const container = containerRef.current;
    if (!container) return;

    const previouslyFocused = document.activeElement as HTMLElement | null;

    const focusables = () =>
      Array.from(container.querySelectorAll<HTMLElement>(FOCUSABLE)).filter(
        (el) => el.offsetParent !== null,
      );

    // Fokuskan elemen pertama bila dialog belum memindahkan fokus sendiri
    // (mis. lewat autoFocus). Menghindari fokus tetap di halaman belakang.
    if (!container.contains(document.activeElement)) {
      focusables()[0]?.focus();
    }

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== "Tab") return;
      const items = focusables();
      if (items.length === 0) {
        event.preventDefault();
        return;
      }
      const first = items[0];
      const last = items[items.length - 1];
      const current = document.activeElement;
      if (event.shiftKey) {
        if (current === first || !container.contains(current)) {
          event.preventDefault();
          last.focus();
        }
      } else if (current === last || !container.contains(current)) {
        event.preventDefault();
        first.focus();
      }
    };

    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("keydown", onKeyDown);
      previouslyFocused?.focus?.();
    };
  }, [containerRef, active]);
}
