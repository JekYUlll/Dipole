#include "contract_validator.hpp"

#include <filesystem>
#include <fstream>
#include <sstream>
#include <string_view>
#include <unordered_set>

#include <google/protobuf/message.h>
#include <google/protobuf/struct.pb.h>
#include <google/protobuf/util/json_util.h>
#include <google/protobuf/util/time_util.h>

namespace dipole::realtime {
namespace {

using delivery::v1::DeliveryAckStatus;
using delivery::v1::DeliveryErrorCode;
using delivery::v1::DeliveryMode;
using delivery::v1::DeliveryResult;
using delivery::v1::DeliveryResultStatus;
using delivery::v1::NodeObservationStatus;

bool IsBlank(std::string_view value) {
  return value.empty() || value.find_first_not_of(" \t\r\n") == std::string_view::npos;
}

bool IsValidTimestamp(const google::protobuf::Timestamp& value) {
  return value.seconds() >= google::protobuf::util::TimeUtil::kTimestampMinSeconds &&
         value.seconds() <= google::protobuf::util::TimeUtil::kTimestampMaxSeconds &&
         value.nanos() >= 0 && value.nanos() <= 999999999;
}

bool IsValidMode(DeliveryMode value) {
  return value >= DeliveryMode::DELIVERY_MODE_FULL_EVENT &&
         value <= DeliveryMode::DELIVERY_MODE_HOT_GROUP_NOTIFY;
}

bool IsValidAckStatus(DeliveryAckStatus value) {
  return value >= DeliveryAckStatus::DELIVERY_ACK_STATUS_ACCEPTED &&
         value <= DeliveryAckStatus::DELIVERY_ACK_STATUS_REJECTED;
}

bool IsValidResultStatus(DeliveryResultStatus value) {
  return value >= DeliveryResultStatus::DELIVERY_RESULT_STATUS_ENQUEUED &&
         value <= DeliveryResultStatus::DELIVERY_RESULT_STATUS_FAILED;
}

ValidationError ValidateResultEvidence(const DeliveryResult& result) {
  switch (result.status()) {
    case DeliveryResultStatus::DELIVERY_RESULT_STATUS_ENQUEUED:
      if (result.accepted_connections() == 0 || result.retry_after_ms() != 0 ||
          result.error_code() != DeliveryErrorCode::DELIVERY_ERROR_CODE_UNSPECIFIED) {
        return "enqueued result has inconsistent evidence";
      }
      break;
    case DeliveryResultStatus::DELIVERY_RESULT_STATUS_OFFLINE:
      if (result.accepted_connections() != 0 || result.retry_after_ms() != 0 ||
          result.error_code() != DeliveryErrorCode::DELIVERY_ERROR_CODE_UNSPECIFIED) {
        return "offline result has inconsistent evidence";
      }
      break;
    case DeliveryResultStatus::DELIVERY_RESULT_STATUS_BACKPRESSURED:
      if (result.retry_after_ms() == 0 ||
          result.error_code() != DeliveryErrorCode::DELIVERY_ERROR_CODE_QUEUE_FULL) {
        return "backpressured result requires retry hint and queue_full";
      }
      break;
    case DeliveryResultStatus::DELIVERY_RESULT_STATUS_REJECTED:
      if (result.error_code() != DeliveryErrorCode::DELIVERY_ERROR_CODE_INVALID_ITEM) {
        return "rejected result requires invalid_item";
      }
      break;
    case DeliveryResultStatus::DELIVERY_RESULT_STATUS_FAILED:
      if (result.error_code() != DeliveryErrorCode::DELIVERY_ERROR_CODE_NODE_UNAVAILABLE &&
          result.error_code() != DeliveryErrorCode::DELIVERY_ERROR_CODE_INTERNAL) {
        return "failed result requires a failure error code";
      }
      break;
    default:
      return "delivery result status is invalid";
  }
  return std::nullopt;
}

ValidationError ParseJsonFile(const std::string& path, google::protobuf::Message* message) {
  if (message == nullptr) {
    return "protobuf destination is required";
  }
  std::ifstream input(path);
  if (!input) {
    return "cannot open " + path;
  }
  std::ostringstream raw;
  raw << input.rdbuf();
  google::protobuf::util::JsonParseOptions options;
  options.ignore_unknown_fields = false;
  const auto status = google::protobuf::util::JsonStringToMessage(raw.str(), message, options);
  if (!status.ok()) {
    return "decode " + path + ": " + status.ToString();
  }
  return std::nullopt;
}

}  // namespace

ValidationError ValidateEnvelope(const delivery::v1::DeliveryEnvelope& envelope) {
  if (envelope.contract_version() != kContractVersion) {
    return "delivery contract version is unsupported";
  }
  if (IsBlank(envelope.batch_id()) || IsBlank(envelope.source_event_id()) ||
      IsBlank(envelope.source_topic())) {
    return "delivery envelope identity and source are required";
  }
  if (envelope.source_partition() < 0 || envelope.source_offset() < 0 ||
      !envelope.has_created_at() || !IsValidTimestamp(envelope.created_at())) {
    return "delivery source coordinates and created_at are invalid";
  }
  if (envelope.items_size() < 1 || envelope.items_size() > kMaxBatchItems) {
    return "delivery envelope item count is invalid";
  }
  std::unordered_set<std::string> delivery_ids;
  for (const auto& item : envelope.items()) {
    if (IsBlank(item.delivery_id()) || IsBlank(item.recipient_user_id()) ||
        IsBlank(item.event_type()) || IsBlank(item.ordering_key())) {
      return "delivery item identity, recipient, event type, and ordering key are required";
    }
    if (!delivery_ids.insert(item.delivery_id()).second) {
      return "delivery id is duplicated";
    }
    google::protobuf::Value payload;
    google::protobuf::util::JsonParseOptions options;
    options.ignore_unknown_fields = false;
    if (!google::protobuf::util::JsonStringToMessage(item.payload_json(), &payload, options).ok()) {
      return "delivery payload_json is invalid";
    }
    if (!IsValidMode(item.mode())) {
      return "delivery mode is invalid";
    }
  }
  return std::nullopt;
}

ValidationError ValidateNodeBatch(const delivery::v1::NodeDeliveryBatch& batch) {
  if (batch.contract_version() != kContractVersion) {
    return "delivery contract version is unsupported";
  }
  if (IsBlank(batch.batch_id()) || IsBlank(batch.target_node_id()) ||
      IsBlank(batch.source_event_id())) {
    return "node batch identity, target node, and source event are required";
  }
  if (!batch.has_created_at() || !IsValidTimestamp(batch.created_at())) {
    return "node batch created_at is invalid";
  }
  if (batch.items_size() < 1 || batch.items_size() > kMaxBatchItems) {
    return "node batch item count is invalid";
  }
  std::unordered_set<std::string> delivery_ids;
  for (const auto& item : batch.items()) {
    if (IsBlank(item.delivery_id()) || IsBlank(item.recipient_user_id()) ||
        IsBlank(item.event_type()) || IsBlank(item.ordering_key()) || item.connection_ids().empty()) {
      return "node delivery item identity, recipient, connections, event type, and ordering key are required";
    }
    if (!delivery_ids.insert(item.delivery_id()).second) {
      return "node delivery id is duplicated";
    }
    std::unordered_set<std::string> connections;
    for (const auto& connection_id : item.connection_ids()) {
      if (IsBlank(connection_id) || !connections.insert(connection_id).second) {
        return "node delivery connection id is empty or duplicated";
      }
    }
    google::protobuf::Value payload;
    if (!google::protobuf::util::JsonStringToMessage(item.payload_json(), &payload).ok()) {
      return "node delivery payload_json is invalid";
    }
    if (!IsValidMode(item.mode())) {
      return "node delivery mode is invalid";
    }
  }
  return std::nullopt;
}

ValidationError ValidateAck(const delivery::v1::DeliveryAck& ack) {
  if (ack.contract_version() != kContractVersion || IsBlank(ack.batch_id()) ||
      !IsValidAckStatus(ack.status()) || !ack.has_acknowledged_at() ||
      !IsValidTimestamp(ack.acknowledged_at()) || ack.results().empty()) {
    return "delivery ack identity, status, timestamp, and results are required";
  }
  bool has_accepted = false;
  bool has_non_accepted = false;
  bool has_backpressure = false;
  std::unordered_set<std::string> delivery_ids;
  for (const auto& result : ack.results()) {
    if (IsBlank(result.delivery_id()) || !IsValidResultStatus(result.status())) {
      return "delivery result identity and status are required";
    }
    if (!delivery_ids.insert(result.delivery_id()).second) {
      return "delivery result id is duplicated";
    }
    if (const auto error = ValidateResultEvidence(result); error.has_value()) {
      return error;
    }
    const bool accepted = result.status() == DeliveryResultStatus::DELIVERY_RESULT_STATUS_ENQUEUED ||
                          result.status() == DeliveryResultStatus::DELIVERY_RESULT_STATUS_OFFLINE;
    has_accepted = has_accepted || accepted;
    has_non_accepted = has_non_accepted || !accepted;
    has_backpressure = has_backpressure ||
                       result.status() == DeliveryResultStatus::DELIVERY_RESULT_STATUS_BACKPRESSURED;
  }
  if (has_backpressure &&
      (!ack.has_pressure() || ack.pressure().capacity() == 0 || ack.pressure().retry_after_ms() == 0 ||
       ack.pressure().depth() < ack.pressure().capacity())) {
    return "backpressured ack requires saturated queue pressure and retry hint";
  }
  if (ack.status() == DeliveryAckStatus::DELIVERY_ACK_STATUS_ACCEPTED && has_non_accepted) {
    return "accepted ack contains a non-accepted result";
  }
  if (ack.status() == DeliveryAckStatus::DELIVERY_ACK_STATUS_PARTIAL && !has_non_accepted) {
    return "partial ack requires a non-accepted result";
  }
  if (ack.status() == DeliveryAckStatus::DELIVERY_ACK_STATUS_REJECTED && has_accepted) {
    return "rejected ack contains an accepted result";
  }
  return std::nullopt;
}

ValidationError ValidateObservation(const delivery::v1::NodeDeliveryObservation& observation) {
  if (observation.contract_version() != kContractVersion || IsBlank(observation.batch_id()) ||
      IsBlank(observation.target_node_id()) || !observation.has_observed_at() ||
      !IsValidTimestamp(observation.observed_at())) {
    return "node observation version, identity, node, and timestamp are required";
  }
  switch (observation.status()) {
    case NodeObservationStatus::NODE_OBSERVATION_STATUS_OBSERVED:
      if (observation.observed_items() == 0 || observation.observed_connections() == 0 ||
          observation.error_code() != DeliveryErrorCode::DELIVERY_ERROR_CODE_UNSPECIFIED) {
        return "observed node delivery requires positive counts and no error";
      }
      break;
    case NodeObservationStatus::NODE_OBSERVATION_STATUS_REJECTED:
      if (observation.observed_items() != 0 || observation.observed_connections() != 0 ||
          observation.has_pressure() ||
          observation.error_code() != DeliveryErrorCode::DELIVERY_ERROR_CODE_INVALID_ITEM) {
        return "rejected node delivery requires invalid_item and no observed work";
      }
      break;
    case NodeObservationStatus::NODE_OBSERVATION_STATUS_BACKPRESSURED:
      if (observation.observed_items() != 0 || observation.observed_connections() != 0 ||
          observation.error_code() != DeliveryErrorCode::DELIVERY_ERROR_CODE_QUEUE_FULL ||
          !observation.has_pressure() || observation.pressure().capacity() == 0 ||
          observation.pressure().depth() < observation.pressure().capacity() ||
          observation.pressure().retry_after_ms() == 0) {
        return "backpressured node delivery requires saturated queue pressure";
      }
      break;
    default:
      return "node observation status is required";
  }
  return std::nullopt;
}

ValidationError ParseEnvelopeJsonFile(const std::string& path,
                                      delivery::v1::DeliveryEnvelope* envelope) {
  return ParseJsonFile(path, envelope);
}

ValidationError ParseNodeBatchJsonFile(const std::string& path,
                                       delivery::v1::NodeDeliveryBatch* batch) {
  return ParseJsonFile(path, batch);
}

ValidationError ParseAckJsonFile(const std::string& path, delivery::v1::DeliveryAck* ack) {
  return ParseJsonFile(path, ack);
}

ValidationError ParseObservationJsonFile(
    const std::string& path, delivery::v1::NodeDeliveryObservation* observation) {
  return ParseJsonFile(path, observation);
}

ValidationError ValidateGoldenDirectory(const std::string& directory) {
  const std::filesystem::path root(directory);
  delivery::v1::DeliveryEnvelope envelope;
  if (const auto error = ParseEnvelopeJsonFile((root / "delivery_batch.v1.json").string(), &envelope);
      error.has_value()) {
    return error;
  }
  if (const auto error = ValidateEnvelope(envelope); error.has_value()) {
    return error;
  }

  delivery::v1::NodeDeliveryBatch node_batch;
  if (const auto error = ParseNodeBatchJsonFile(
          (root / "node_delivery_batch.v1.json").string(), &node_batch);
      error.has_value()) {
    return error;
  }
  if (const auto error = ValidateNodeBatch(node_batch); error.has_value()) {
    return error;
  }

  delivery::v1::DeliveryAck ack;
  if (const auto error = ParseAckJsonFile((root / "delivery_ack.v1.json").string(), &ack);
      error.has_value()) {
    return error;
  }
  if (const auto error = ValidateAck(ack); error.has_value()) {
    return error;
  }

  delivery::v1::NodeDeliveryObservation observation;
  if (const auto error = ParseObservationJsonFile(
          (root / "node_delivery_observation.v1.json").string(), &observation);
      error.has_value()) {
    return error;
  }
  return ValidateObservation(observation);
}

}  // namespace dipole::realtime
