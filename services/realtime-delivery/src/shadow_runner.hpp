#ifndef DIPOLE_REALTIME_DELIVERY_SHADOW_RUNNER_HPP_
#define DIPOLE_REALTIME_DELIVERY_SHADOW_RUNNER_HPP_

#include <atomic>
#include <cstddef>
#include <cstdint>
#include <optional>
#include <string>

#include "event_projection.hpp"
#include "authority_fence.hpp"
#include "node_delivery_transport.hpp"
#include "presence_projection.hpp"

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

enum class ShadowOutcome : std::uint8_t { kProjected, kRejected, kDeferred };
enum class NodeTransportMode : std::uint8_t { kObserve, kPrimary };

// ShadowEvidence deliberately excludes message payloads, recipient IDs and raw
// errors. Detailed event bodies remain in Kafka and the authoritative Go path.
struct ShadowEvidence {
  std::string topic;
  std::int32_t partition = -1;
  std::int64_t offset = -1;
  std::string source_event_id;
  std::string batch_id;
  int message_type = -1;
  std::size_t item_count = 0;
  ShadowOutcome outcome = ShadowOutcome::kRejected;
  std::string error_code;
  std::size_t node_batch_count = 0;
  std::size_t presence_observed = 0;
  std::size_t presence_eligible = 0;
  std::size_t presence_stale = 0;
  std::size_t presence_malformed = 0;
  std::size_t offline_item_count = 0;
  std::size_t transport_requested = 0;
  std::size_t transport_observed = 0;
  std::size_t transport_duplicate = 0;
  std::size_t transport_rejected = 0;
  std::size_t transport_backpressured = 0;
  bool transport_primary = false;
  std::size_t transport_enqueued = 0;
  std::size_t transport_offline = 0;
  std::size_t transport_failed = 0;
  bool primary_offset_commit = false;
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
  std::uint64_t presence_read_errors = 0;
  std::uint64_t transport_errors = 0;
  std::uint64_t fence_errors = 0;
};

class ShadowRunner {
 public:
  ShadowRunner(ShadowRecordConsumer* consumer, ShadowEvidenceSink* evidence_sink, int poll_timeout_ms,
               PresenceReader* presence_reader = nullptr, NodeBatchTransport* node_transport = nullptr,
               NodeTransportMode node_transport_mode = NodeTransportMode::kObserve,
               AuthorityFenceReader* authority_fence = nullptr);

  ValidationError RunOnce(const ProjectionPolicy& policy,
                          const std::optional<PresenceProjectionPolicy>& presence_policy = std::nullopt);
  [[nodiscard]] bool Ready() const;
  [[nodiscard]] ShadowRunnerStats Stats() const;

 private:
  ShadowRecordConsumer* consumer_;
  ShadowEvidenceSink* evidence_sink_;
  int poll_timeout_ms_;
  PresenceReader* presence_reader_;
  NodeBatchTransport* node_transport_;
  NodeTransportMode node_transport_mode_;
  AuthorityFenceReader* authority_fence_;
  std::optional<KafkaRecord> pending_record_;
  std::atomic_bool healthy_ = true;
  std::atomic_bool fence_healthy_ = true;
  ShadowRunnerStats stats_;
};

}  // namespace dipole::realtime

#endif  // DIPOLE_REALTIME_DELIVERY_SHADOW_RUNNER_HPP_
