# C3 C++ Projection Benchmark

## Evidence

- Revision: `7eb11de75054d8088196401266e3a31b6e873ade`
- Runner: Dockerfile `builder` target, `cpp_runner=container`
- Workload: `message.direct.created`, 10,000 iterations
- C++: `31,218.83 ops/s`
- Go: `261,843.03 ops/s`
- C++/Go ratio: `0.119227`
- Decision: `blocked` because the C3 eligibility threshold is `1.0`
- C++ builder CTest: 14/14 passed
- Remote GPU task snapshot: 2 Python processes before and after; no task was stopped or modified

The report is retained as a fail-closed performance result. It does not authorize C++ primary or gray rollout.
