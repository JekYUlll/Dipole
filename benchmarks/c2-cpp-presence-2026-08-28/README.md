# C2 C++ Presence Shadow Evidence

This archive binds the first Kafka plus Redis Presence replay and an isolated Redis Sentinel failover drill to C++ candidate `633491f19795a583aa465ca7cf82d5368bb90f1f`.

The replay used a dedicated earliest-safe Kafka shadow group and one temporary Presence fixture containing one eligible and one stale connection. The NDJSON evidence contains only Kafka coordinates, stable event/batch IDs and aggregate routing counters. The fixture, shadow process, isolated Sentinel project, network and volumes were removed after collection.

The failover drill reused one `HiredisPresenceReader` for 80 reads while stopping the current master. Sentinel promoted the remaining replica; the reader observed a bounded error window and then rediscovered the new master without process restart.

Run `sha256sum --check SHA256SUMS` from this directory to verify the archive.
