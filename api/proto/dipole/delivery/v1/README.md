# Realtime Delivery Contract v1

`dipole.delivery.v1` is the language-neutral boundary between IM event projection and the realtime data plane.

- `DeliveryEnvelope` binds every dispatch to its Kafka source coordinates and correlation identifiers.
- `NodeDeliveryBatch` is emitted after Presence resolution and contains one target Gateway node plus only the connection IDs owned by that node.
- `delivery_id` is unique within a batch and remains stable across retries.
- `ordering_key` preserves recipient or conversation ordering when workers execute a batch concurrently.
- `payload_json` contains only the WebSocket event data. The edge adds the versioned WebSocket envelope.
- `DeliveryMode` distinguishes full events, Timeline notifications, and hot-group pull notifications.
- `DeliveryAck` reports each item independently. `BACKPRESSURED` includes a bounded retry hint and `QUEUE_FULL`; `OFFLINE` is a successful routing decision and does not require immediate retry. Error codes use a finite protobuf enum to keep metrics and retry policy consistent across languages.
- A v1 envelope or node batch contains at most 4096 items. Producers split larger fanout sets into independently replayable batches.

The Go legacy adapter is a compatibility implementation. C++ shadow and primary implementations must pass the same golden-vector and validation gates before traffic promotion.
