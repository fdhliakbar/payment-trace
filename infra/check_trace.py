import json
import urllib.request

trace_id = "133f04ea87d2344b9a80f1e91503e561"
url = f"http://localhost:16686/api/traces/{trace_id}"
with urllib.request.urlopen(url) as r:
    d = json.load(r)
data = d.get("data") or []
if not data:
    print("no trace data")
    raise SystemExit(1)
spans = data[0].get("spans", [])
procs = data[0].get("processes", {})
print(f"trace={trace_id}")
print(f"spans={len(spans)}")
print("services:", sorted({p.get("serviceName") for p in procs.values()}))
for s in sorted(spans, key=lambda x: x.get("startTime", 0)):
    pid = s.get("processID")
    svc = procs.get(pid, {}).get("serviceName", "?")
    print(f"  {svc:16} {s.get('operationName','?')}")
