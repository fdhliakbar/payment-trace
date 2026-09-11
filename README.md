# Payment Trace

A polyglot distributed payment microservices architecture `Go`, `Rust`, `C#`, `Bun` containerized with **Podman**, featuring end-to-end distributed tracing via **OpenTelemetry** and **Jaeger**, structured logging, **PostgreSQL** persistence, and interactive OpenAPI docs powered by **Scalar**.

<div align="center">
    <img src="" alt="banner payment trace projects" />
</div>

## Flowchart 
...

## Technology

## Quick Setup

```bash
git clone ...
cd ...

```

## Running Infra Only
Start
```bash
docker compose -f podman-compose.yml up -d
```
Check
```bash
docker compose -f podman-compose.yml ps
curl -s -o /dev/null -w "Jaeger UI %{http_code}\n" http://localhost:16686
docker exec postgres pg_isready -U devuser -d payment_db
```
Jaeger UI (browser on Windows)
http://localhost:16686

Logs

```bash
docker compose -f podman-compose.yml logs -f
# or: docker logs -f jaeger
```
Stop

```bash
docker compose -f podman-compose.yml stop
```

Hard reset (wipe DB volume, re-run schema)
```bash
docker compose -f podman-compose.yml down -v
docker compose -f podman-compose.yml up -d
```

## Output Example

### Order with Go Service
```POST http://localhost:your-port/orders/checkout ```

```json
// data sample
{
    "order_id":"ord_150298",
    "trace_id":"3365650c6374012d0c52b837dc45997b",
    "status":"COMPLETED",
    "payment_ref":"tx_csharp_70073"
}
```

### API Documentation with Scalar
<div align="center">
    <img src="./scalar_dccs.png" width="90%" alt="api documentation image with scalar" />
</div>



### Jaeger UI Dashboard

<div align="center">
    <img src="./jaeger-ui.png" width="90%" alt="jaeger ui banners" />
</div>

### Trace ID

<div align="center">
    <img src="./testing_trace_id.png" width="90%" alt="testing trace id banners" />
</div>

---

| Service | Port | URL |
|---|---|---|
| **order-go** (gateway) | **8080** | http://localhost:8080 |
| auth-rust | 8081 | http://localhost:8081 |
| catalog-bun (products) | 8082 | http://localhost:8082 |
| payment-dotnet | 8083 | http://localhost:8083 |
| Jaeger UI | 16686 | http://localhost:16686 |
| PostgreSQL | 5432 | `localhost:5432` |

## Running with WSL (Ubuntu by Default)

```bash
wsl
cd /payment-trace
podman-compose -f podman-compose.yml up -d --build
```
### Create One Order
```bash
curl -X POST http://localhost:8080/orders/checkout \
  -H "Content-Type: application/json" \
  -d '{"user_id":"usr_9918","item_id":"item_guitar_01","quantity":1,"total_amount":150.00}'
```
### Results
```json
{"order_id":"ord_149746","trace_id":"133f04ea87d2344b9a80f1e91503e561","status":"COMPLETED","payment_ref":"tx_csharp_17444"}
```
### Direct Service Calls
```bash
curl http://localhost:8081/health
curl http://localhost:8082/catalog/items/item_guitar_01
curl -X POST http://localhost:8083/payments/charge \
  -H "Content-Type: application/json" \
  -d '{"user_id":"usr_9918","amount":150.00}'
curl http://localhost:8080/docs          # Scalar OpenAPI UI
```
Jaeger trace (verified)
Open http://localhost:16686 and search trace_id: `133f04ea87d2344b9a80f1e91503e561` or **service order-go**.

Confirmed cascade in one trace:
```bash
order.go order.checkout
  → auth-rust        auth.verify_token
  → catalog-bun      catalog.check_stock
  → payment-dotnet   payment.process_charge
```
Teardown
```bash
podman-compose -f podman-compose.yml down
# wipe DB:
podman-compose -f podman-compose.yml down -v
```

---

## Support Me

.... or

<div align="center">
    <img src="" alt="Qrcode Payment" />
</div>

