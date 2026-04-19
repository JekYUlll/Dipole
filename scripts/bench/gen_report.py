#!/usr/bin/env python3
"""
Parse k6 JSON output and generate a self-contained HTML benchmark report.
Usage: python3 gen_report.py <raw.json> <report.html>
"""

import json
import sys
import math
from collections import defaultdict
from datetime import datetime, timezone

def load_metrics(path):
    metrics = defaultdict(list)
    meta = {}
    with open(path) as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                obj = json.loads(line)
            except json.JSONDecodeError:
                continue
            if obj.get("type") == "Metric":
                meta[obj["metric"]] = obj.get("data", {})
            elif obj.get("type") == "Point":
                name = obj.get("metric")
                val  = obj.get("data", {}).get("value")
                tags = obj.get("data", {}).get("tags", {})
                if name and val is not None:
                    metrics[name].append((val, tags))
    return metrics, meta

def percentile(values, p):
    if not values:
        return 0
    s = sorted(values)
    idx = math.ceil(p / 100 * len(s)) - 1
    return s[max(0, idx)]

def stats(values):
    if not values:
        return {"min": 0, "avg": 0, "p50": 0, "p95": 0, "p99": 0, "max": 0, "count": 0}
    return {
        "min":   round(min(values), 2),
        "avg":   round(sum(values) / len(values), 2),
        "p50":   round(percentile(values, 50), 2),
        "p95":   round(percentile(values, 95), 2),
        "p99":   round(percentile(values, 99), 2),
        "max":   round(max(values), 2),
        "count": len(values),
    }

def counter_total(points):
    return sum(v for v, _ in points)

def rate_avg(points):
    if not points:
        return 0
    return round(sum(v for v, _ in points) / len(points) * 100, 2)

def build_report(raw_path):
    metrics, _ = load_metrics(raw_path)

    # E2E latency
    latency_all = [v for v, _ in metrics.get("msg_e2e_latency_ms", [])]
    latency_by_scenario = defaultdict(list)
    for v, tags in metrics.get("msg_e2e_latency_ms", []):
        latency_by_scenario[tags.get("scenario", "unknown")].append(v)

    # HTTP latency
    http_latency = [v * 1000 for v, _ in metrics.get("http_req_duration", [])]  # s -> ms
    http_by_name = defaultdict(list)
    for v, tags in metrics.get("http_req_duration", []):
        http_by_name[tags.get("name", "unknown")].append(v * 1000)

    # Counters
    sent     = int(counter_total(metrics.get("msg_sent_total", [])))
    received = int(counter_total(metrics.get("msg_received_total", [])))
    delivery = rate_avg(metrics.get("msg_delivery_rate", []))
    http_fail_rate = rate_avg(metrics.get("http_req_failed", []))

    # VUs
    vus_vals = [v for v, _ in metrics.get("vus", [])]
    max_vus  = int(max(vus_vals)) if vus_vals else 0

    now = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S UTC")

    # Build scenario rows
    scenario_rows = ""
    for sc, vals in sorted(latency_by_scenario.items()):
        s = stats(vals)
        scenario_rows += f"""
        <tr>
          <td>{sc}</td>
          <td>{s['count']}</td>
          <td>{s['min']} ms</td>
          <td>{s['avg']} ms</td>
          <td>{s['p50']} ms</td>
          <td>{s['p95']} ms</td>
          <td>{s['p99']} ms</td>
          <td>{s['max']} ms</td>
        </tr>"""

    # HTTP rows
    http_rows = ""
    for name, vals in sorted(http_by_name.items()):
        s = stats(vals)
        http_rows += f"""
        <tr>
          <td>{name}</td>
          <td>{s['count']}</td>
          <td>{s['avg']} ms</td>
          <td>{s['p95']} ms</td>
          <td>{s['p99']} ms</td>
          <td>{s['max']} ms</td>
        </tr>"""

    lat_all = stats(latency_all)
    lat_http = stats(http_latency)

    # Threshold pass/fail
    p95_ok  = "✅ PASS" if lat_all["p95"] < 500  else "❌ FAIL"
    p99_ok  = "✅ PASS" if lat_all["p99"] < 1000 else "❌ FAIL"
    del_ok  = "✅ PASS" if delivery > 95          else "❌ FAIL"
    http_ok = "✅ PASS" if http_fail_rate < 1     else "❌ FAIL"

    html = f"""<!DOCTYPE html>
<html lang="zh">
<head>
<meta charset="UTF-8">
<title>Dipole Benchmark Report</title>
<style>
  body {{ font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 40px; color: #222; background: #f9f9f9; }}
  h1   {{ color: #1a1a2e; }}
  h2   {{ color: #16213e; border-bottom: 2px solid #e0e0e0; padding-bottom: 6px; margin-top: 40px; }}
  .meta {{ color: #666; font-size: 0.9em; margin-bottom: 30px; }}
  .cards {{ display: flex; gap: 20px; flex-wrap: wrap; margin-bottom: 30px; }}
  .card {{ background: #fff; border-radius: 8px; padding: 20px 28px; box-shadow: 0 2px 8px rgba(0,0,0,0.08); min-width: 160px; }}
  .card .label {{ font-size: 0.8em; color: #888; text-transform: uppercase; letter-spacing: 0.05em; }}
  .card .value {{ font-size: 2em; font-weight: 700; color: #1a1a2e; margin-top: 4px; }}
  table {{ border-collapse: collapse; width: 100%; background: #fff; border-radius: 8px; overflow: hidden; box-shadow: 0 2px 8px rgba(0,0,0,0.06); }}
  th    {{ background: #1a1a2e; color: #fff; padding: 10px 14px; text-align: left; font-size: 0.85em; }}
  td    {{ padding: 9px 14px; border-bottom: 1px solid #f0f0f0; font-size: 0.9em; }}
  tr:last-child td {{ border-bottom: none; }}
  tr:hover td {{ background: #f5f7ff; }}
  .thresholds {{ display: flex; gap: 16px; flex-wrap: wrap; margin-bottom: 30px; }}
  .thr {{ background: #fff; border-radius: 8px; padding: 14px 20px; box-shadow: 0 2px 8px rgba(0,0,0,0.06); font-size: 0.95em; }}
</style>
</head>
<body>
<h1>Dipole IM — Benchmark Report</h1>
<div class="meta">Generated: {now} &nbsp;|&nbsp; Source: {raw_path}</div>

<h2>Overview</h2>
<div class="cards">
  <div class="card"><div class="label">Max VUs</div><div class="value">{max_vus}</div></div>
  <div class="card"><div class="label">Messages Sent</div><div class="value">{sent}</div></div>
  <div class="card"><div class="label">Messages Received</div><div class="value">{received}</div></div>
  <div class="card"><div class="label">Delivery Rate</div><div class="value">{delivery}%</div></div>
  <div class="card"><div class="label">HTTP Fail Rate</div><div class="value">{http_fail_rate}%</div></div>
  <div class="card"><div class="label">E2E P95</div><div class="value">{lat_all['p95']} ms</div></div>
  <div class="card"><div class="label">E2E P99</div><div class="value">{lat_all['p99']} ms</div></div>
</div>

<h2>Thresholds</h2>
<div class="thresholds">
  <div class="thr">{p95_ok} &nbsp; E2E P95 &lt; 500ms &nbsp; (actual: {lat_all['p95']} ms)</div>
  <div class="thr">{p99_ok} &nbsp; E2E P99 &lt; 1000ms &nbsp; (actual: {lat_all['p99']} ms)</div>
  <div class="thr">{del_ok} &nbsp; Delivery Rate &gt; 95% &nbsp; (actual: {delivery}%)</div>
  <div class="thr">{http_ok} &nbsp; HTTP Fail Rate &lt; 1% &nbsp; (actual: {http_fail_rate}%)</div>
</div>

<h2>E2E Message Latency by Scenario</h2>
<table>
  <thead><tr>
    <th>Scenario</th><th>Count</th><th>Min</th><th>Avg</th>
    <th>P50</th><th>P95</th><th>P99</th><th>Max</th>
  </tr></thead>
  <tbody>{scenario_rows}</tbody>
</table>

<h2>HTTP Request Latency by Endpoint</h2>
<table>
  <thead><tr>
    <th>Endpoint</th><th>Count</th><th>Avg</th><th>P95</th><th>P99</th><th>Max</th>
  </tr></thead>
  <tbody>{http_rows}</tbody>
</table>

<h2>Overall E2E Latency Summary</h2>
<table>
  <thead><tr><th>Min</th><th>Avg</th><th>P50</th><th>P95</th><th>P99</th><th>Max</th><th>Samples</th></tr></thead>
  <tbody>
    <tr>
      <td>{lat_all['min']} ms</td><td>{lat_all['avg']} ms</td><td>{lat_all['p50']} ms</td>
      <td>{lat_all['p95']} ms</td><td>{lat_all['p99']} ms</td><td>{lat_all['max']} ms</td>
      <td>{lat_all['count']}</td>
    </tr>
  </tbody>
</table>

</body>
</html>"""
    return html

if __name__ == "__main__":
    if len(sys.argv) < 3:
        print(f"Usage: {sys.argv[0]} <raw.json> <report.html>")
        sys.exit(1)
    raw_path    = sys.argv[1]
    report_path = sys.argv[2]
    html = build_report(raw_path)
    with open(report_path, "w") as f:
        f.write(html)
    print(f"Report written to {report_path}")
