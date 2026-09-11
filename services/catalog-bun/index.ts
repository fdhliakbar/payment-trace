import { NodeSDK } from "@opentelemetry/sdk-node";
import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-http";
import { Resource } from "@opentelemetry/resources";
import { ATTR_SERVICE_NAME } from "@opentelemetry/semantic-conventions";
import { W3CTraceContextPropagator } from "@opentelemetry/core";
import { trace, context, propagation, ROOT_CONTEXT } from "@opentelemetry/api";
import { Hono } from "hono";

const otlpEndpoint =
  process.env.OTEL_EXPORTER_OTLP_ENDPOINT ?? "http://localhost:4318";
const serviceName = process.env.OTEL_SERVICE_NAME ?? "catalog-bun";
const port = Number(process.env.PORT ?? 8082);

const exporter = new OTLPTraceExporter({
  url: `${otlpEndpoint.replace(/\/$/, "")}/v1/traces`,
});

const sdk = new NodeSDK({
  resource: new Resource({ [ATTR_SERVICE_NAME]: serviceName }),
  traceExporter: exporter,
  textMapPropagator: new W3CTraceContextPropagator(),
});
sdk.start();
propagation.setGlobalPropagator(new W3CTraceContextPropagator());

process.on("SIGTERM", () => {
  sdk.shutdown().finally(() => process.exit(0));
});

function headersToCarrier(headers: Headers): Record<string, string> {
  const out: Record<string, string> = {};
  headers.forEach((v, k) => {
    out[k] = v;
  });
  return out;
}

type Item = { id: string; name: string; price: number; in_stock: boolean };

const CATALOG: Record<string, Item> = {
  item_guitar_01: { id: "item_guitar_01", name: "Solid Wood Tele", price: 150.0, in_stock: true },
  item_amp_01: { id: "item_amp_01", name: "Tube Amp 40W", price: 299.0, in_stock: true },
  item_pick_01: { id: "item_pick_01", name: "Pack of Picks", price: 9.5, in_stock: false },
};

const app = new Hono();

app.get("/health", (c) => c.text("ok"));

app.get("/catalog/items/:id", async (c) => {
  const tracer = trace.getTracer(serviceName);
  const id = c.req.param("id");
  const parentCtx = propagation.extract(
    ROOT_CONTEXT,
    headersToCarrier(c.req.raw.headers)
  );

  return await context.with(parentCtx, () =>
    tracer.startActiveSpan(
      "catalog.check_stock",
      async (span) => {
      span.setAttribute("catalog.item_id", id);
      const item = CATALOG[id];
      console.log(
        JSON.stringify({
          level: "info",
          message: "Item stock checked",
          service: serviceName,
          item_id: id,
          found: Boolean(item),
          in_stock: item?.in_stock ?? false,
          trace_id: span.spanContext().traceId,
          span_id: span.spanContext().spanId,
        })
      );

      if (!item) {
        span.setStatus({ code: 2 });
        span.end();
        return c.json({ error: "item_not_found", id }, 404);
      }
      span.end();
      return c.json(item);
      }
    )
  );
});

// Expose any incoming W3C context for debugging
app.get("/catalog/debug/headers", (c) => {
  const extracted = propagation.extract(context.active(), c.req.raw.headers as any);
  const active = trace.getSpan(extracted)?.spanContext();
  return c.json({ traceparent: c.req.header("traceparent") ?? null, active: active ?? null });
});

console.log(JSON.stringify({ level: "info", message: `catalog-bun listening on ${port}`, service: serviceName }));
export default {
  port,
  fetch: app.fetch,
};
