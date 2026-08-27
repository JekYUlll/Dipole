#ifndef DIPOLE_REALTIME_DELIVERY_SHADOW_RUNNER_HPP_
#define DIPOLE_REALTIME_DELIVERY_SHADOW_RUNNER_HPP_

#include <cstddef>
#include <cstdint>
#include <string>

#include "event_projection.hpp"

namespace dipole::realtime {

enum class PollStatus : std::uint8_t { kRecord, kTimeout, kError };

struct PollResult {
  PollStatus status = PollStatus::kTimeout;
  KafkaRecord record;
  std::string error;
};

class ShadowRecordConsumer {
 public:
  virtual ~ShadowRecordConsumer() = default;
  virtual PollResult Poll(int timeout_ms) = 0;
  virtual ValidationError Commit(const KafkaRecord& record) = 0;
  [[nodiscard]] virtual std::size_t AssignmentCount() const = 0;
};

enum class ShadowOutcome : std::uint8_t { kProjected, kRejected };

// ShadowEvidence deliberately excludes message payloads, recipient IDs and raw
// errors. Detailed event bodies remain in Kafka and the authoritative Go path.
struct ShadowEvidence {
  std::string topic;
  std::int32_t partition = -1;
  std::int64_t offset = -1;
  std::string source_event_id;
  std::string batch_id;
  std::size_t item_count = 0;
  ShadowOutcome outcome = ShadowOutcome::kRejected;
  std::string error_code;
};

class ShadowEvidenceSink {
 public:
  virtual ~ShadowEvidenceSink() = default;
  virtual ValidationError Append(const ShadowEvidence& evidence) = 0;
};

struct ShadowRunnerStats {
  std::uint64_t polled = 0;
  std::uint64_t projected = 0;
  std::uint64_t rejected = 0;
  std::uint64_t evidence_written = 0;
  std::uint64_t committed = 0;
  std::uint64_t poll_errors = 0;
  std::uint64_t evidence_errors = 0;
  std::uint64_t commit_errors = 0;
};

class ShadowRunner {
 public:
  ShadowRunner(ShadowRecordConsumer* consumer, ShadowEvidenceSink* evidence_sink, int poll_timeout_ms);

  ValidationError RunOnce(const ProjectionPolicy& policy);
  [[nodiscard]] bool Ready() const;
  [[nodiscard]] ShadowRunnerStats Stats() const;

 private:
  ShadowRecordConsumer* consumer_;
  ShadowEvidenceSink* evidence_sink_;
  int poll_timeout_ms_;
  bool healthy_ = true;
  ShadowRunnerStats stats_;
};

}  // namespace dipole::realtime

#endif  // DIPOLE_REALTIME_DELIVERY_SHADOW_RUNNER_HPP_
