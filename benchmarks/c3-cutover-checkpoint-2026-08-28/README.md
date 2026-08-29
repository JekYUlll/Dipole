# C3 Cutover Checkpoint Evidence

This isolated drill validates the checkpoint collector at Git revision `3a8910217998bef1c689d66a26afa1b5e6ad8e64` on 2026-08-28. It did not use or mutate the shared `dipole-node1`, `dipole-node2`, or `dipole-node3` services.

## Topology

- Kafka: `apache/kafka:3.9.0`, image `sha256:2956061ba5d3388af8eb3148200f385ccba13ef75bc0eb6dcca5827ebd2d36d4`, isolated loopback port `19095`.
- Redis: `redis:7.4`, image `sha256:54fc9bbc80cdb3b3d8e3a5197732bff502650b62a8bc86f4c3c36e152db4e1af`, isolated loopback port `16391`.
- Compatibility group: `dipole-gateway-consumer`, one real kafka-go member.
- Cross-language group: `dipole-realtime-shadow-checkpoint-v1`, one real C++ librdkafka member from binary SHA-256 `9614ef018a4f293d7c622b243776f9355d4eaabb742957798e457aeb16f0fcb3`.
- Topics: `dipole.message.direct.created` and `dipole.message.group.created`, one partition and one fixture record each.

The C++ process used shadow authority only to establish a real librdkafka group member and commit malformed low-sensitivity fixture records. This evidence verifies cross-client group metadata and checkpoint collection; it does not claim a C++ primary client-delivery cutover. Prior C3 single-frame evidence covers the local primary path separately.

The two Redis observations were injected from the exact v1 contract with real TTLs after the Go and C++ emitters had already been verified independently. Their payloads contain only component/observer labels, authority state, timestamps and hashes.

## Eligible Result

`checkpoint-bundle.json` passed the Draft 2020-12 bundle schema and was published as mode `0600`. Its SHA-256 is `e6ffa77ee73cb51fc508483b312145bca431f567acd77526b1ee514f172c976a`.

- Both expected observations matched active Go epoch 1 and lease SHA-256 `1910b9276432538708635b7f1deb82b770fe31455d9e3b3cd191a3b05043e16e`.
- Both groups were `Stable` and assigned both topic partitions.
- Every committed/log-end coordinate was `1/1`, with lag 0.
- The raw DescribeGroups adapter decoded kafka-go assignment v1 and librdkafka assignment v0 with its opaque extension tail.
- The two read-committed log-end samples remained unchanged during capture.

## Blocked Results

Blocked commands did not create their requested output file.

- Removing `services/realtime-delivery/cpp-real-a` produced `delivery authority observation services/realtime-delivery/cpp-real-a is expired or missing`.
- Stopping the primary-role group member produced `Kafka checkpoint group dipole-realtime-primary-v1 state must be Stable, got Empty`.
- The first Java console-consumer fixture exposed `Got non-zero number of bytes remaining: 10` in kafka-go's high-level assignment decoder. The final implementation uses a bounded raw ConsumerProtocol prefix parser and was rerun successfully against kafka-go and C++ librdkafka members.

All temporary consumers and isolated containers were stopped after capture. Automatic rollback, authority interruption and shared-topology cutover remain pending under `AD-041`.
