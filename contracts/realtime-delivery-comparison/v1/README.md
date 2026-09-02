# Realtime Delivery Comparison v1

This contract binds one Go realtime baseline to one C++ shadow evidence stream without copying message, user, connection or batch details.

The gate groups C++ attempts by Kafka topic, partition and offset, then selects the low-sensitivity `message_type` declared by the Go baseline. This keeps setup/system events visible in `observed_kafka_records` while excluding them from workload parity. Deferred attempts may precede one final projected outcome. Eligibility requires the Go workload to have complete acceptance, persistence and receipt counts with zero settled lag, and every selected C++ coordinate to finish with all requested node batches observed, zero final rejection/backpressure, and clean Presence aggregates.

Input files are bound by SHA-256 and each runtime is bound to a full Git revision. Structural or identity errors fail the command with exit code 1; a valid but divergent comparison writes `blocked` and exits 2; `eligible` exits 0.

```bash
python3 scripts/realtime_delivery_comparison.py \
  --go-baseline /path/to/baseline.json \
  --cpp-evidence /path/to/shadow.ndjson \
  --go-revision "$(git rev-parse HEAD)" \
  --cpp-revision "$(git rev-parse HEAD)" \
  --output /path/to/comparison.json
```
