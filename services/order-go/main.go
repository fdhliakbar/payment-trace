package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type checkoutRequest struct {
	UserID      string  `json:"user_id"`
	ItemID      string  `json:"item_id"`
	Quantity    int     `json:"quantity"`
	TotalAmount float64 `json:"total_amount"`
}

type checkoutResponse struct {
	OrderID    string `json:"order_id"`
	TraceID    string `json:"trace_id"`
	Status     string `json:"status"`
	PaymentRef string `json:"payment_ref"`
}

type structuredLog struct {
	Level     string  `json:"level"`
	Message   string  `json:"message"`
	Service   string  `json:"service"`
	TraceID   string  `json:"trace_id"`
	SpanID    string  `json:"span_id"`
	UserID    string  `json:"user_id,omitempty"`
	ItemID    string  `json:"item_id,omitempty"`
	Extra     map[string]any `json:"extra,omitempty"`
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func initTracer(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	endpoint := envOr("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")

	exp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	res := resource.NewSchemaless(
		semconv.ServiceName(serviceName),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))
	return tp.Shutdown, nil
}

type jsonLogger struct{ svc string }

func (l jsonLogger) emit(level, msg string, sc trace.SpanContext, fields map[string]any) {
	rec := structuredLog{
		Level:   level,
		Message: msg,
		Service: l.svc,
		TraceID: sc.TraceID().String(),
		SpanID:  sc.SpanID().String(),
		Extra:   fields,
	}
	b, _ := json.Marshal(rec)
	fmt.Println(string(b))
}

func doPost(ctx context.Context, client *http.Client, url string, payload any, out any) (int, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, err
	}
	if out != nil && resp.StatusCode < 400 {
		if err := json.Unmarshal(data, out); err != nil {
			return resp.StatusCode, err
		}
	}
	return resp.StatusCode, nil
}

func doGet(ctx context.Context, client *http.Client, url string, out any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, err
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return resp.StatusCode, err
		}
	}
	return resp.StatusCode, nil
}

const docsHTML = `<!DOCTYPE html>
<html>
<head>
  <title>payment-trace API</title>
  <meta charset="utf-8" />
  <style>body{margin:0;font-family:system-ui,sans-serif}</style>
</head>
<body>
  <script id="api-reference" data-url="/openapi.json"></script>
  <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
</body>
</html>`

const openapiJSON = `{
  "openapi": "3.0.3",
  "info": {
    "title": "payment-trace",
    "version": "1.0.0",
    "description": "Polyglot checkout with distributed tracing"
  },
  "servers": [{"url": "http://localhost:8080"}],
  "paths": {
    "/orders/checkout": {
      "post": {
        "summary": "Checkout order (orchestrates auth, catalog, payment)",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/CheckoutRequest"}
            }
          }
        },
        "responses": {
          "201": {
            "description": "Order completed",
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/CheckoutResponse"}
              }
            }
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "CheckoutRequest": {
        "type": "object",
        "required": ["user_id", "item_id", "quantity", "total_amount"],
        "properties": {
          "user_id": {"type": "string", "example": "usr_9918"},
          "item_id": {"type": "string", "example": "item_guitar_01"},
          "quantity": {"type": "integer", "example": 1},
          "total_amount": {"type": "number", "example": 150.00}
        }
      },
      "CheckoutResponse": {
        "type": "object",
        "properties": {
          "order_id": {"type": "string"},
          "trace_id": {"type": "string"},
          "status": {"type": "string"},
          "payment_ref": {"type": "string"}
        }
      }
    }
  }
}`

func main() {
	serviceName := envOr("OTEL_SERVICE_NAME", "order-go")
	port := envOr("PORT", "8080")

	sugar, err := zap.NewProduction()
	if err != nil {
		log.Fatal(err)
	}
	defer sugar.Sync()
	_ = zapcore.InfoLevel

	shutdown, err := initTracer(context.Background(), serviceName)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = shutdown(context.Background()) }()

	jlog := jsonLogger{svc: serviceName}
	tracer := otel.Tracer(serviceName)
	client := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
		Timeout:   10 * time.Second,
	}

	authURL := envOr("AUTH_SERVICE_URL", "http://localhost:8081")
	catalogURL := envOr("CATALOG_SERVICE_URL", "http://localhost:8082")
	paymentURL := envOr("PAYMENT_SERVICE_URL", "http://localhost:8083")

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok"))
	})
	r.Get("/docs", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(docsHTML))
	})
	r.Get("/openapi.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(openapiJSON))
	})

	r.Post("/orders/checkout", func(w http.ResponseWriter, req *http.Request) {
		ctx, span := tracer.Start(req.Context(), "order.checkout",
			trace.WithAttributes(
				attribute.String("http.route", "/orders/checkout"),
			))
		defer span.End()

		var body checkoutRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, `{"error":"invalid_json"}`, http.StatusBadRequest)
			return
		}
		if body.UserID == "" || body.ItemID == "" || body.Quantity <= 0 {
			http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
			return
		}

		sc := trace.SpanContextFromContext(ctx)
		jlog.emit("info", "Checkout initialized", sc, map[string]any{
			"user_id": body.UserID, "item_id": body.ItemID,
		})

		// 1. Auth
		var authResp struct {
			Valid  bool   `json:"valid"`
			UserID string `json:"user_id"`
		}
		code, err := doPost(ctx, client, authURL+"/auth/verify",
			map[string]any{"user_id": body.UserID, "token": "demo-token"}, &authResp)
		if err != nil || code >= 400 || !authResp.Valid {
			span.RecordError(fmt.Errorf("auth failed code=%d err=%v", code, err))
			jlog.emit("error", "Auth verify failed", sc, map[string]any{"code": code})
			http.Error(w, `{"error":"auth_failed"}`, http.StatusUnauthorized)
			return
		}
		jlog.emit("info", "Auth verified", sc, map[string]any{"user_id": authResp.UserID})

		// 2. Catalog
		var item struct {
			ID      string  `json:"id"`
			Name    string  `json:"name"`
			Price   float64 `json:"price"`
			InStock bool    `json:"in_stock"`
		}
		code, err = doGet(ctx, client, fmt.Sprintf("%s/catalog/items/%s", catalogURL, body.ItemID), &item)
		if err != nil || code >= 400 || !item.InStock {
			span.RecordError(fmt.Errorf("catalog failed code=%d err=%v", code, err))
			jlog.emit("error", "Catalog check failed", sc, map[string]any{"code": code, "item_id": body.ItemID})
			http.Error(w, `{"error":"item_unavailable"}`, http.StatusConflict)
			return
		}
		jlog.emit("info", "Item stock checked", sc, map[string]any{"item_id": item.ID, "price": item.Price})

		// 3. Payment
		var payResp struct {
			TransactionID string `json:"transaction_id"`
			Status        string `json:"status"`
		}
		code, err = doPost(ctx, client, paymentURL+"/payments/charge",
			map[string]any{"user_id": body.UserID, "amount": body.TotalAmount}, &payResp)
		if err != nil || code >= 400 {
			span.RecordError(fmt.Errorf("payment failed code=%d err=%v", code, err))
			jlog.emit("error", "Payment charge failed", sc, map[string]any{"code": code})
			http.Error(w, `{"error":"payment_failed"}`, http.StatusPaymentRequired)
			return
		}

		orderID := fmt.Sprintf("ord_%d", time.Now().Unix()%1000000)
		jlog.emit("info", "Order persisted, status COMPLETED", sc, map[string]any{
			"order_id": orderID, "payment_ref": payResp.TransactionID,
		})

		resp := checkoutResponse{
			OrderID:    orderID,
			TraceID:    sc.TraceID().String(),
			Status:     "COMPLETED",
			PaymentRef: payResp.TransactionID,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(resp)
	})

	addr := "0.0.0.0:" + port
	jlog.emit("info", "order-go listening", trace.SpanContext{}, map[string]any{"addr": addr})
	log.Fatal(http.ListenAndServe(addr, r))
}
