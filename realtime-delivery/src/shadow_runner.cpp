#include "shadow_runner.hpp"

#include <string>

namespace dipole::realtime {

ShadowRunner::ShadowRunner(ShadowRecordConsumer* consumer, ShadowEvidenceSink* evidence_sink,
                           int poll_timeout_ms)
    : consumer_(consumer), evidence_sink_(evidence_sink), poll_timeout_ms_(poll_timeout_ms) {}

ValidationError ShadowRunner::RunOnce(const ProjectionPolicy& policy) {
  if (consumer_ == nullptr) {
    healthy_.store(false);
    return "shadow consumer is required";
  }
  if (evidence_sink_ == nullptr) {
    healthy_.store(false);
    return "shadow evidence sink is required";
  }
  if (poll_timeout_ms_ <= 0) {
    healthy_.store(false);
    return "shadow poll timeout must be positive";
  }

  PollResult result = consumer_->Poll(poll_timeout_ms_);
  if (result.status == PollStatus::kTimeout) {
    return std::nullopt;
  }
  if (result.status == PollStatus::kError) {
    ++stats_.poll_errors;
    healthy_.store(false);
    return "shadow Kafka poll failed: " + result.error;
  }

  ++stats_.polled;
  delivery::v1::DeliveryEnvelope envelope;
  const auto projection_error = ProjectMessageEvent(result.record, policy, &envelope);
  ShadowEvidence evidence;
  evidence.topic = result.record.topic;
  evidence.partition = result.record.partition;
  evidence.offset = result.record.offset;
  if (projection_error) {
    evidence.outcome = ShadowOutcome::kRejected;
    evidence.error_code = "invalid_event";
    ++stats_.rejected;
  } else {
    evidence.source_event_id = envelope.source_event_id();
    evidence.batch_id = envelope.batch_id();
    evidence.item_count = static_cast<std::size_t>(envelope.items_size());
    evidence.outcome = ShadowOutcome::kProjected;
    ++stats_.projected;
  }

  if (const auto evidence_error = evidence_sink_->Append(evidence); evidence_error) {
    ++stats_.evidence_errors;
    healthy_.store(false);
    return "shadow evidence append failed: " + *evidence_error;
  }
  ++stats_.evidence_written;

  if (const auto commit_error = consumer_->Commit(result.record); commit_error) {
    ++stats_.commit_errors;
    healthy_.store(false);
    return "shadow Kafka commit failed: " + *commit_error;
  }
  ++stats_.committed;
  healthy_.store(true);
  return std::nullopt;
}

bool ShadowRunner::Ready() const {
  return healthy_.load() && consumer_ != nullptr && evidence_sink_ != nullptr && poll_timeout_ms_ > 0 &&
         consumer_->AssignmentCount() > 0;
}

ShadowRunnerStats ShadowRunner::Stats() const { return stats_; }

}  // namespace dipole::realtime
