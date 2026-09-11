import json
import urllib.request

for svc in ["auth-rust", "catalog-bun", "order-go", "payment-dotnet"]:
    url = f"http://localhost:16686/api/traces?service={svc}&limit=5"
    with urllib.request.urlopen(url) as r:
        d = json.load(r)
    traces = d.get("data") or []
    print(f"{svc}: {len(traces)} traces")
    for t in traces[:2]:
        print(f"  {t['traceID'][:16]}... spans={len(t['spans'])} ops={[s['operationName'] for s in t['spans'][:6]]}")
