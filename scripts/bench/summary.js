// Keep setup data (including login credentials) out of exported reports.
export function handleSummary(data) {
  const metrics = {};
  for (const [name, metric] of Object.entries(data.metrics || {})) {
    metrics[name] = { ...metric.values };
    if (metric.thresholds) {
      metrics[name].thresholds = Object.fromEntries(
        Object.entries(metric.thresholds).map(([expression, result]) => [expression, result.ok])
      );
    }
  }
  const report = JSON.stringify({ metrics }, null, 2);
  return __ENV.SUMMARY_JSON ? { [__ENV.SUMMARY_JSON]: report } : { stdout: report + "\n" };
}
