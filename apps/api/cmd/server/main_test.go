package main

import (
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

// newTestServer menjalankan http.Server pada port acak dan mengembalikan servernya
// beserta alamat yang benar-benar dipakai.
func newTestServer(t *testing.T, handler http.Handler) (*http.Server, string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("gagal membuka listener: %v", err)
	}
	server := &http.Server{Handler: handler}
	go func() {
		_ = server.Serve(listener)
	}()
	return server, listener.Addr().String()
}

// newTestClient membuat klien HTTP tanpa keep-alive. Default client memakai pool
// koneksi global; bila dipakai bersama antar test, koneksi ke server test sebelumnya
// (yang sudah mati) bisa dipakai ulang dan memicu handler test lain. Klien terpisah
// tanpa keep-alive menghindari itu.
func newTestClient(timeout time.Duration) *http.Client {
	transport := &http.Transport{DisableKeepAlives: true}
	return &http.Client{Timeout: timeout, Transport: transport}
}

// TestServeWithShutdownStopsServer membuktikan bahwa menutup channel stop benar-benar
// menghentikan server: setelah shutdown, koneksi baru tidak lagi dilayani.
func TestServeWithShutdownStopsServer(t *testing.T) {
	server, addr := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	client := newTestClient(2 * time.Second)

	// Sanity: server melayani sebelum shutdown.
	resp, err := client.Get("http://" + addr)
	if err != nil {
		t.Fatalf("server harus melayani sebelum shutdown: %v", err)
	}
	_ = resp.Body.Close()

	stop := make(chan struct{})
	done := make(chan struct{})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	go func() {
		_ = serveWithShutdown(server, logger, stop, 5*time.Second)
		close(done)
	}()

	close(stop)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("serveWithShutdown tidak selesai dalam batas waktu")
	}

	// Setelah shutdown, permintaan baru harus gagal (koneksi ditolak / server tutup).
	if resp, err := client.Get("http://" + addr); err == nil {
		_ = resp.Body.Close()
		t.Fatal("server seharusnya tidak melayani lagi setelah shutdown")
	}
}

// TestServeWithShutdownWaitsForInflight membuktikan permintaan yang sedang berjalan
// diberi kesempatan selesai (bukan diputus), inti dari graceful shutdown.
func TestServeWithShutdownWaitsForInflight(t *testing.T) {
	release := make(chan struct{})
	handlerStarted := make(chan struct{})
	var once sync.Once

	server, addr := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		once.Do(func() { close(handlerStarted) })
		<-release // tahan sampai test mengizinkan selesai
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("selesai"))
	}))
	client := newTestClient(5 * time.Second)

	stop := make(chan struct{})
	done := make(chan struct{})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	go func() {
		_ = serveWithShutdown(server, logger, stop, 5*time.Second)
		close(done)
	}()

	type result struct {
		body string
		err  error
	}
	resCh := make(chan result, 1)
	go func() {
		resp, err := client.Get("http://" + addr)
		if err != nil {
			resCh <- result{err: err}
			return
		}
		defer func() { _ = resp.Body.Close() }()
		b, _ := io.ReadAll(resp.Body)
		resCh <- result{body: string(b)}
	}()

	<-handlerStarted // handler sudah berjalan
	close(stop)      // minta shutdown saat handler masih berjalan

	// Beri kesempatan handler menyelesaikan diri; jangan buru-buru.
	time.Sleep(100 * time.Millisecond)
	close(release)

	select {
	case r := <-resCh:
		if r.err != nil {
			t.Fatalf("permintaan berjalan seharusnya selesai, bukan gagal: %v", r.err)
		}
		if r.body != "selesai" {
			t.Fatalf("badan respons = %q, mau %q", r.body, "selesai")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("permintaan berjalan tidak selesai")
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("serveWithShutdown tidak selesai setelah permintaan berjalan rampung")
	}
}
