#include <chrono>
#include <cstdint>
#include <cstdlib>
#include <iostream>
#include <string>

#include <nlohmann/json.hpp>

#include "event_projection.hpp"

namespace {

constexpr char kEvent[] = R"json({
  "event_id":"E-benchmark",
  "request_id":"R-benchmark",
  "trace_id":"T-benchmark",
  "event_type":"message.direct.created",
  "version":"v1",
  "source":"dipole",
  "occurred_at":"2026-08-29T00:00:00Z",
  "payload":{
    "mutation_type":"created",
    "revision":1,
    "actor_uuid":"U1",
    "message_id":"M-benchmark",
    "conversation_key":"direct:U1:U2",
    "message_seq":42,
    "sender_uuid":"U1",
    "target_uuid":"U2",
    "target_type":0,
    "message_type":0,
    "content":"benchmark body",
    "sent_at":"2026-08-29T00:00:00Z"
  }
})json";

std::uint64_t ParseIterations(int argc, char** argv) {
  if (argc < 2) return 100000;
  const auto parsed = std::strtoull(argv[1], nullptr, 10);
  return parsed == 0 ? 100000 : parsed;
}

}  // namespace

int main(int argc, char** argv) {
  const std::uint64_t iterations = ParseIterations(argc, argv);
  const dipole::realtime::KafkaRecord record{
      .topic = "dipole.message.created", .partition = 0, .offset = 0, .value = kEvent};
  dipole::delivery::v1::DeliveryEnvelope output;
  for (std::uint64_t index = 0; index < 1000; ++index) {
    output.Clear();
    if (dipole::realtime::ProjectMessageEvent(record, {}, &output)) return 2;
  }

  std::uint64_t item_count = 0;
  const auto started = std::chrono::steady_clock::now();
  for (std::uint64_t index = 0; index < iterations; ++index) {
    output.Clear();
    if (dipole::realtime::ProjectMessageEvent(record, {}, &output)) return 2;
    item_count += static_cast<std::uint64_t>(output.items_size());
  }
  const auto elapsed = std::chrono::duration_cast<std::chrono::nanoseconds>(
      std::chrono::steady_clock::now() - started);
  const auto elapsed_ns = static_cast<std::uint64_t>(elapsed.count());
  std::cout << nlohmann::json({
                                 {"schema_version", "dipole.realtime.projection-benchmark.v1"},
                                 {"language", "cpp"},
                                 {"iterations", iterations},
                                 {"item_count", item_count},
                                 {"elapsed_ns", elapsed_ns},
                                 {"ops_per_second", elapsed_ns == 0
                                                          ? 0.0
                                                          : static_cast<double>(iterations) * 1e9 /
                                                                static_cast<double>(elapsed_ns)},
                             })
                .dump()
         << '\n';
  return 0;
}
