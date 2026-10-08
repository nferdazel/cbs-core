package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestReadyReturnsReadyWhenHealthy membuktikan /ready membalas 200 ketika pemeriksaan
// kesiapan (mis. ping basis data) berhasil. Probe ini harus terpisah dari /healthz
// supaya load balancer hanya mengirim lalu lintas ke instance yang benar-benar siap.
func TestReadyReturnsReadyWhenHealthy(t *testing.T) {
	called := false
	router := NewRouter(RouterParams{
		Readiness: func(context.Context) error {
			called = true
			return nil
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, mau %d", rec.Code, http.StatusOK)
	}
	if !called {
		t.Fatal("Readiness harus dipanggil oleh /ready")
	}
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("badan respons bukan JSON: %v", err)
	}
	if !body.Success || body.Data.Status != "READY" {
		t.Fatalf("badan respons tak terduga: %s", rec.Body.String())
	}
}

// TestReadyReturns503WhenDependencyFails membuktikan /ready tidak berbohong: bila
// basis data tidak sehat, ia membalas 503, bukan 200.
func TestReadyReturns503WhenDependencyFails(t *testing.T) {
	router := NewRouter(RouterParams{
		Readiness: func(context.Context) error {
			return errors.New("basis data tidak dapat dihubungi")
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, mau %d", rec.Code, http.StatusServiceUnavailable)
	}
	var body struct {
		Success bool `json:"success"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Success {
		t.Fatal("respons gagal tidak boleh menyatakan sukses")
	}
}

// TestReadyWithoutCheckerIsReady menjaga kompatibilitas: router tanpa pemeriksaan
// (mis. pada test lain) tetap membalas 200 agar probe tidak mengganggu.
func TestReadyWithoutCheckerIsReady(t *testing.T) {
	router := NewRouter(RouterParams{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, mau %d", rec.Code, http.StatusOK)
	}
}

// TestHealthzStillAlive membuktikan /healthz tidak ikut berubah menjadi pemeriksaan
// dependensi: ia tetap liveness (proses hidup) walau Readiness gagal.
func TestHealthzStillAliveWhenReadinessFails(t *testing.T) {
	router := NewRouter(RouterParams{
		Readiness: func(context.Context) error { return errors.New("down") },
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, mau %d", rec.Code, http.StatusOK)
	}
}
