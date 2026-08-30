# C3 Equivalent Projection Benchmark

- Revision: `8a87cc44ecb41af21fd70bb26bf38d6cde9bc005`
- Workload: identical direct event and `DeliveryEnvelope` output projection, 10,000 iterations
- C++: `31,287.85 ops/s`
- Go: `126,533.88 ops/s`
- C++/Go ratio: `0.247269`
- C++ builder CTest: `14/14` passed
- Decision: `blocked` under the `1.0` eligibility threshold
- Remote GPU snapshot: 0 processes before, 2 external Python processes after; neither was modified

The Go benchmark now constructs the generated delivery protobuf envelope and JSON payload. The implementations still use different JSON/protobuf libraries, so this is a fairer contract-level comparison, not proof of identical machine work.
