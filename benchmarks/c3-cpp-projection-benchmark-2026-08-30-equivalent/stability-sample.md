# Stability Sample

The sample reused the C++ builder image tagged `dipole-realtime-delivery:benchmark-fa319312`. `services/realtime-delivery` and the delivery proto have no changes between `fa319312` and `6450d04c`, so the binary is valid for the current C++ source. The Go executable came from the current `master` checkout with the pinned remote Go 1.27 toolchain.

All runs used 10,000 iterations of the same direct-event `DeliveryEnvelope` projection:

| Run | C++ ops/s | Go ops/s | Ratio |
| --- | ---: | ---: | ---: |
| 1 | 31,228.68 | 123,671.07 | 0.2525 |
| 2 | 31,423.02 | 124,278.96 | 0.2528 |
| 3 | 31,566.86 | 122,116.15 | 0.2585 |
| 4 | 30,758.90 | 122,944.95 | 0.2502 |
| 5 | 31,577.02 | 125,592.76 | 0.2514 |

The range remains below the `1.0` eligibility threshold. Remote GPU processes were the same two external Python tasks before and after the sample; no Dipole containers remained.
