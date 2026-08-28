#include "shadow_runner.hpp"

#include <nlohmann/json.hpp>
#include <string>
#include <unordered_set>
#include <vector>

namespace dipole::realtime {
namespace {

int ProjectedMessageType(const delivery::v1::DeliveryEnvelope& envelope) {
  if (envelope.items().empty()) return -1;
  const auto payload = nlohmann::json::parse(envelope.items(0).payload_json(), nullptr, false);
  if (payload.is_discarded()) return -1;
  const auto message_type = payload.find("message_type");
  return message_type != payload.end() && message_type->is_number_integer() ? message_type->get<int>() : -1;
}

}  // namespace

ShadowRunner::ShadowRunner(ShadowRecordConsumer* consumer, ShadowEvidenceSink* evidence_sink, int poll_timeout_ms,
                           PresenceReader* presence_reader, NodeBatchTransport* node_transport,
                           NodeTransportMode node_transport_mode, AuthorityFenceReader* authority_fence)
    : consumer_(consumer),
      evidence_sink_(evidence_sink),
      poll_timeout_ms_(poll_timeout_ms),
      presence_reader_(presence_reader),
      node_transport_(node_transport),
      node_transport_mode_(node_transport_mode),
      authority_fence_(authority_fence) {
}

ValidationError ShadowRunner::RunOnce(const ProjectionPolicy& policy,
                                      const std::optional<PresenceProjectionPolicy>& presence_policy) {
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
  if (node_transport_ != nullptr && presence_reader_ == nullptr) {
    healthy_.store(false);
    return "node transport requires Presence routing";
  }
  if (node_transport_mode_ == NodeTransportMode::kPrimary && node_transport_ == nullptr) {
    healthy_.store(false);
    return "primary node transport is required";
  }

  PollResult result;
  if (pending_record_) {
    result = {.status = PollStatus::kRecord, .record = *pending_record_, .error = {}};
  } else {
    result = consumer_->Poll(poll_timeout_ms_);
    if (result.status == PollStatus::kTimeout) {
      if (authority_fence_ != nullptr) {
        if (const auto fence_error = authority_fence_->Heartbeat(); fence_error) {
          ++stats_.fence_errors;
          fence_healthy_.store(false);
          return "delivery authority fence heartbeat denied: " + *fence_error;
        }
        fence_healthy_.store(true);
      }
      return std::nullopt;
    }
    if (result.status == PollStatus::kError) {
      ++stats_.poll_errors;
      healthy_.store(false);
      return "shadow Kafka poll failed: " + result.error;
    }
    ++stats_.polled;
    pending_record_ = result.record;
  }

  if (authority_fence_ != nullptr) {
    if (const auto fence_error = authority_fence_->Assert(); fence_error) {
      ++stats_.fence_errors;
      fence_healthy_.store(false);
      return "delivery authority fence denied: " + *fence_error;
    }
    fence_healthy_.store(true);
  }

  delivery::v1::DeliveryEnvelope envelope;
  const auto projection_error = ProjectMessageEvent(result.record, policy, &envelope);
  ShadowEvidence evidence;
  ValidationError deferred_error;
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
    evidence.message_type = ProjectedMessageType(envelope);
    evidence.item_count = static_cast<std::size_t>(envelope.items_size());
    if (presence_reader_ != nullptr) {
      if (!presence_policy) {
        healthy_.store(false);
        return "presence projection policy is required";
      }
      std::unordered_set<std::string> unique_recipients;
      for (const auto& item : envelope.items()) unique_recipients.insert(item.recipient_user_id());
      std::vector<std::string> recipients(unique_recipients.begin(), unique_recipients.end());
      PresenceReadResult read_result;
      if (const auto read_error = presence_reader_->ReadUsers(recipients, &read_result); read_error) {
        ++stats_.presence_read_errors;
        healthy_.store(false);
        return "shadow Presence read failed: " + *read_error;
      }
      std::vector<delivery::v1::NodeDeliveryBatch> batches;
      PresenceProjectionStats presence_stats;
      const auto presence_error =
          ProjectPresence(envelope, read_result.by_user, *presence_policy, &batches, &presence_stats);
      evidence.node_batch_count = batches.size();
      evidence.presence_observed = presence_stats.observed_connections;
      evidence.presence_eligible = presence_stats.eligible_connections;
      evidence.presence_stale = presence_stats.stale_connections;
      evidence.presence_malformed = read_result.parse_stats.malformed_records;
      evidence.offline_item_count = presence_stats.offline_items;
      if (presence_error) {
        evidence.outcome = ShadowOutcome::kRejected;
        evidence.error_code = "invalid_presence";
        ++stats_.rejected;
      } else {
        evidence.outcome = ShadowOutcome::kProjected;
        if (node_transport_ != nullptr) {
          if (node_transport_mode_ == NodeTransportMode::kPrimary) {
            evidence.transport_primary = true;
            PrimaryDeliveryStats transport_stats;
            if (batches.empty()) {
              transport_stats.offline = presence_stats.offline_items;
              transport_stats.decision = PrimaryOffsetDecision::kCommit;
            } else {
              deferred_error = node_transport_->Deliver(batches, &transport_stats);
            }
            evidence.transport_requested = transport_stats.requested;
            evidence.transport_enqueued = transport_stats.enqueued;
            evidence.transport_offline = transport_stats.offline;
            evidence.transport_rejected = transport_stats.rejected;
            evidence.transport_backpressured = transport_stats.backpressured;
            evidence.transport_failed = transport_stats.failed;
            evidence.primary_offset_commit =
                !deferred_error && transport_stats.decision == PrimaryOffsetDecision::kCommit;
            if (!deferred_error && !evidence.primary_offset_commit) {
              deferred_error = "primary acknowledgement requires retry";
              evidence.error_code = "primary_ack_retain";
            }
          } else {
            NodeTransportStats transport_stats;
            deferred_error = node_transport_->Observe(batches, &transport_stats);
            evidence.transport_requested = transport_stats.requested;
            evidence.transport_observed = transport_stats.observed;
            evidence.transport_duplicate = transport_stats.duplicate;
            evidence.transport_rejected = transport_stats.rejected;
            evidence.transport_backpressured = transport_stats.backpressured;
          }
          if (deferred_error) {
            evidence.outcome = ShadowOutcome::kDeferred;
            if (evidence.error_code.empty()) evidence.error_code = "node_transport";
            ++stats_.transport_errors;
          }
        }
        if (!deferred_error) {
          ++stats_.projected;
        }
      }
    } else {
      evidence.outcome = ShadowOutcome::kProjected;
      ++stats_.projected;
    }
  }

  if (const auto evidence_error = evidence_sink_->Append(evidence); evidence_error) {
    ++stats_.evidence_errors;
    healthy_.store(false);
    return "shadow evidence append failed: " + *evidence_error;
  }
  ++stats_.evidence_written;

  if (deferred_error) {
    healthy_.store(false);
    return "shadow node transport failed: " + *deferred_error;
  }

  if (const auto commit_error = consumer_->Commit(result.record); commit_error) {
    ++stats_.commit_errors;
    healthy_.store(false);
    return "shadow Kafka commit failed: " + *commit_error;
  }
  ++stats_.committed;
  pending_record_.reset();
  healthy_.store(true);
  return std::nullopt;
}

bool ShadowRunner::Ready() const {
  return healthy_.load() && fence_healthy_.load() && consumer_ != nullptr && evidence_sink_ != nullptr &&
         poll_timeout_ms_ > 0 && consumer_->AssignmentCount() > 0;
}

ShadowRunnerStats ShadowRunner::Stats() const {
  return stats_;
}

}  // namespace dipole::realtime
