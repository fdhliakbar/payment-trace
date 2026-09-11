using System.Diagnostics;
using Npgsql;
using OpenTelemetry.Resources;
using OpenTelemetry.Trace;

var builder = WebApplication.CreateBuilder(args);

var serviceName = Environment.GetEnvironmentVariable("OTEL_SERVICE_NAME") ?? "payment-dotnet";
var otlp = Environment.GetEnvironmentVariable("OTEL_EXPORTER_OTLP_ENDPOINT") ?? "http://localhost:4317";
if (!otlp.StartsWith("http://") && !otlp.StartsWith("https://"))
{
    otlp = $"http://{otlp}";
}

builder.Services.AddOpenTelemetry()
    .ConfigureResource(r => r.AddService(serviceName))
    .WithTracing(t => t
        .AddAspNetCoreInstrumentation()
        .AddHttpClientInstrumentation()
        .AddSource("payment-dotnet")
        .AddOtlpExporter(o =>
        {
            o.Endpoint = new Uri(otlp);
            o.Protocol = OpenTelemetry.Exporter.OtlpExportProtocol.Grpc;
        }));

builder.Services.AddSingleton(new NpgsqlConnectionStringBuilder(
    Environment.GetEnvironmentVariable("ConnectionStrings__Postgres")
    ?? "Host=localhost;Port=5432;Database=payment_db;Username=devuser;Password=devpassword"));

var app = builder.Build();
var activitySource = new ActivitySource("payment-dotnet");

app.MapGet("/health", () => "ok");

app.MapPost("/payments/charge", async (ChargeRequest req, NpgsqlConnectionStringBuilder cs) =>
{
    using var activity = activitySource.StartActivity("payment.process_charge", ActivityKind.Internal);
    activity?.SetTag("payment.user_id", req.UserId);
    activity?.SetTag("payment.amount", req.Amount);

    var log = new
    {
        level = "info",
        message = "Processing charge",
        service = serviceName,
        user_id = req.UserId,
        amount = req.Amount,
        trace_id = activity?.TraceId.ToString(),
        span_id = activity?.SpanId.ToString(),
    };
    Console.WriteLine(System.Text.Json.JsonSerializer.Serialize(log));

    await using var conn = new NpgsqlConnection(cs.ConnectionString);
    await conn.OpenAsync();

    await using (var cmd = new NpgsqlCommand(
        "INSERT INTO ledger_accounts (account_id, user_id, balance, currency) " +
        "VALUES (@aid, @uid, 1000, 'USD') ON CONFLICT (user_id) DO NOTHING", conn))
    {
        cmd.Parameters.AddWithValue("aid", $"acct_{req.UserId}");
        cmd.Parameters.AddWithValue("uid", req.UserId);
        await cmd.ExecuteNonQueryAsync();
    }

    decimal balance;
    await using (var cmd = new NpgsqlCommand(
        "SELECT balance FROM ledger_accounts WHERE user_id = @uid FOR UPDATE", conn))
    {
        cmd.Parameters.AddWithValue("uid", req.UserId);
        var result = await cmd.ExecuteScalarAsync();
        balance = result is decimal d ? d : 0m;
    }

    if (balance < req.Amount)
    {
        activity?.SetStatus(ActivityStatusCode.Error, "insufficient_funds");
        Console.WriteLine(System.Text.Json.JsonSerializer.Serialize(new
        {
            level = "error",
            message = "Insufficient funds",
            service = serviceName,
            trace_id = activity?.TraceId.ToString(),
        }));
        return Results.Json(new { error = "insufficient_funds", balance }, statusCode: 402);
    }

    await using (var cmd = new NpgsqlCommand(
        "UPDATE ledger_accounts SET balance = balance - @amt, updated_at = now() WHERE user_id = @uid", conn))
    {
        cmd.Parameters.AddWithValue("amt", req.Amount);
        cmd.Parameters.AddWithValue("uid", req.UserId);
        await cmd.ExecuteNonQueryAsync();
    }

    var txId = $"tx_csharp_{Random.Shared.Next(10000, 99999)}";
    Console.WriteLine(System.Text.Json.JsonSerializer.Serialize(new
    {
        level = "info",
        message = "Payment captured successfully",
        service = serviceName,
        transaction_id = txId,
        trace_id = activity?.TraceId.ToString(),
        span_id = activity?.SpanId.ToString(),
    }));

    return Results.Ok(new { transaction_id = txId, status = "APPROVED" });
});

app.Run();

record ChargeRequest(
    [property: System.Text.Json.Serialization.JsonPropertyName("user_id")] string UserId,
    [property: System.Text.Json.Serialization.JsonPropertyName("amount")] decimal Amount);
