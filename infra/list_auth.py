import json
import urllib.request

# Search all auth-rust traces
url = "http://localhost:16686/api/traces?service=auth-rust&limit=20&lookback=1h"
with urllib.request.urlopen(url) as r:
    d = json.load(r)
traces = d.get("data") or []
print(f"auth-rust traces={len(traces)}")
for t in traces:
    print(f"  {t['traceID']} spans={len(t['spans'])} ops={[s['operationName'] for s in t['spans']]}")

# Also try finding by operation
url2 = "http://localhost:16686/api/traces?service=auth-rust&operation=auth.verify_token&limit=10&lookback=1h"
with urllib.request.urlopen(url2) as r:
    d2 = json.load(r)
print(f"auth.verify_token traces={len(d2.get('data') or [])}")
