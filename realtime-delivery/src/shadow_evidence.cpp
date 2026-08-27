#include "shadow_evidence.hpp"

#include <string_view>

#include <nlohmann/json.hpp>

namespace dipole::realtime {
namespace {

std::string_view OutcomeName(ShadowOutcome outcome) {
  switch (outcome) {
    case ShadowOutcome::kProjected:
      return "projected";
    case ShadowOutcome::kRejected:
      return "rejected";
    case ShadowOutcome::kDeferred:
      return "deferred";
  }
  return "unknown";
}

}  // namespace

JsonLineEvidenceSink::JsonLineEvidenceSink(std::ostream* output) : output_(output) {}

ValidationError JsonLineEvidenceSink::Append(const ShadowEvidence& evidence) {
  if (output_ == nullptr) {
    return "shadow evidence output is required";
  }
  if (evidence.topic.empty() || evidence.partition < 0 || evidence.offset < 0) {
    return "shadow evidence Kafka coordinates are invalid";
  }
  nlohmann::json record = {
      {"schema_version", "dipole.realtime.shadow-evidence.v3"},
      {"topic", evidence.topic},
      {"partition", evidence.partition},
      {"offset", evidence.offset},
      {"outcome", OutcomeName(evidence.outcome)},
      {"source_event_id", evidence.source_event_id},
      {"batch_id", evidence.batch_id},
      {"message_type", nullptr},
      {"item_count", evidence.item_count},
      {"error_code", evidence.error_code},
      {"node_batch_count", evidence.node_batch_count},
      {"presence_observed", evidence.presence_observed},
      {"presence_eligible", evidence.presence_eligible},
      {"presence_stale", evidence.presence_stale},
      {"presence_malformed", evidence.presence_malformed},
      {"offline_item_count", evidence.offline_item_count},
      {"transport_requested", evidence.transport_requested},
      {"transport_observed", evidence.transport_observed},
      {"transport_duplicate", evidence.transport_duplicate},
      {"transport_rejected", evidence.transport_rejected},
      {"transport_backpressured", evidence.transport_backpressured},
  };
  if (evidence.message_type >= 0) {
    record["message_type"] = evidence.message_type;
  }
  *output_ << record.dump() << '\n';
  output_->flush();
  if (!output_->good()) {
    return "write shadow evidence stream";
  }
  return std::nullopt;
}

}  // namespace dipole::realtime
