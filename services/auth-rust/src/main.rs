use axum::{
    extract::State,
    http::{HeaderMap, StatusCode},
    routing::post,
    Json, Router,
};
use opentelemetry::trace::{Span, TraceContextExt, Tracer as _, TracerProvider as _};
use opentelemetry_otlp::WithExportConfig;
use opentelemetry_sdk::{
    propagation::TraceContextPropagator,
    trace::{RandomIdGenerator, Sampler, TracerProvider},
    Resource,
};
use serde::{Deserialize, Serialize};
use std::{env, sync::Arc};
use tracing::{info, instrument};
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt, EnvFilter};

#[derive(Clone)]
struct AppState {
    provider: Arc<TracerProvider>,
}

#[derive(Deserialize)]
struct VerifyRequest {
    #[allow(dead_code)]
    user_id: Option<String>,
    token: Option<String>,
}

#[derive(Serialize)]
struct VerifyResponse {
    valid: bool,
    user_id: String,
}

fn init_tracer() -> TracerProvider {
    opentelemetry::global::set_text_map_propagator(TraceContextPropagator::new());

    let endpoint = env::var("OTEL_EXPORTER_OTLP_ENDPOINT")
        .unwrap_or_else(|_| "http://localhost:4317".into());
    let endpoint = if endpoint.starts_with("http://") || endpoint.starts_with("https://") {
        endpoint
    } else {
        format!("http://{endpoint}")
    };

    let exporter = opentelemetry_otlp::new_exporter()
        .tonic()
        .with_endpoint(endpoint);

    let provider = opentelemetry_otlp::new_pipeline()
        .tracing()
        .with_exporter(exporter)
        .with_trace_config(
            opentelemetry_sdk::trace::Config::default()
                .with_sampler(Sampler::AlwaysOn)
                .with_id_generator(RandomIdGenerator::default())
                .with_resource(Resource::new(vec![
                    opentelemetry::KeyValue::new(
                        "service.name",
                        env::var("OTEL_SERVICE_NAME").unwrap_or_else(|_| "auth-rust".into()),
                    ),
                ])),
        )
        .install_batch(opentelemetry_sdk::runtime::Tokio)
        .expect("failed to install OTLP tracer");

    // Also register as global so other crates can resolve context
    opentelemetry::global::set_tracer_provider(provider.clone());

    let tracer = provider.tracer("auth-rust");
    let telemetry = tracing_opentelemetry::layer().with_tracer(tracer);

    tracing_subscriber::registry()
        .with(EnvFilter::from_default_env().add_directive("info".parse().unwrap()))
        .with(tracing_subscriber::fmt::layer().json())
        .with(telemetry)
        .init();

    provider
}

#[instrument(skip(state, headers, body), fields(service = "auth-rust"))]
async fn verify(
    State(state): State<Arc<AppState>>,
    headers: HeaderMap,
    Json(body): Json<VerifyRequest>,
) -> Result<Json<VerifyResponse>, StatusCode> {
    let parent_cx = opentelemetry::global::get_text_map_propagator(|p| {
        p.extract(&opentelemetry_http::HeaderExtractor(&headers))
    });
    let tracer = state.provider.tracer("auth-rust");
    let span = tracer.start_with_context("auth.verify_token", &parent_cx);
    let cx = parent_cx.with_span(span);

    let token = body.token.as_deref().unwrap_or("demo-token");
    let user_id = body
        .user_id
        .clone()
        .unwrap_or_else(|| "usr_9918".to_string());

    {
        let span_ref = cx.span();
        span_ref.set_attribute(opentelemetry::KeyValue::new("auth.user_id", user_id.clone()));

        info!(
            traceparent = ?headers.get("traceparent"),
            user_id = %user_id,
            "auth.verify_token: token validated"
        );

        let valid = token != "invalid" && !token.is_empty();
        span_ref.end();
        Ok(Json(VerifyResponse { valid, user_id }))
    }
}

#[tokio::main]
async fn main() {
    let provider = init_tracer();

    let port = env::var("PORT").unwrap_or_else(|_| "8081".into());
    let state = Arc::new(AppState {
        provider: Arc::new(provider),
    });

    let app = Router::new()
        .route("/auth/verify", post(verify))
        .route("/health", axum::routing::get(|| async { "ok" }))
        .with_state(state);

    let addr = format!("0.0.0.0:{port}");
    info!(%addr, "auth-rust listening");
    let listener = tokio::net::TcpListener::bind(&addr).await.unwrap();
    axum::serve(listener, app).await.unwrap();
}
