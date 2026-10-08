package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// TracerConfig mengatur penyalaan tracing.
//
// Aturan yang mengikat:
//   - Endpoint kosong berarti tracing MATI: tidak ada exporter, tidak ada overhead,
//     dan program berjalan seperti sebelumnya. Fitur ini opt-in, bukan memaksa infra.
//   - Rasio sampling kecil di produksi agar biaya tetap terkendali.
//   - Propagasi memakai W3C Trace Context, menyatu dengan `request_id` yang sudah ada
//     (request_id tetap dinamis dan tidak digantikan).
type TracerConfig struct {
	ServiceName string
	Endpoint    string
	Environment string
	SampleRatio float64
}

// SetupTracing menyalakan tracing OTLP/HTTP bila Endpoint diisi. Mengembalikan fungsi
// shutdown yang WAJIB dipanggil saat proses berhenti agar span terakhir ter-flush.
//
// Bila Endpoint kosong, mengembalikan fungsi no-op dan tidak mengubah provider global,
// sehingga aplikasi berjalan tanpa dependensi runtime.
func SetupTracing(ctx context.Context, cfg TracerConfig) (func(context.Context) error, error) {
	if cfg.Endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}

	ratio := cfg.SampleRatio
	if ratio < 0 || ratio > 1 {
		ratio = 1.0
	}

	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(cfg.Endpoint),
		// Collector dalam jaringan internal biasanya tanpa TLS; TLS diaktifkan lewat
		// endpoint https bila memang ada. Keputusan ini dicatat agar tidak diam-diam
		// mengirim trace terenkripsi/gagal.
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("gagal membuat exporter OTLP: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("gagal menyusun resource OTel: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			// Batch pendek: span cepat naik ke collector tanpa menunggu lama.
			sdktrace.WithBatchTimeout(5*time.Second),
		),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}
