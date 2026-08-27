#include "presence_projection.hpp"

#include <algorithm>
#include <map>
#include <string>
#include <unordered_set>
#include <utility>
#include <vector>

#include <google/protobuf/timestamp.pb.h>
#include <google/protobuf/util/time_util.h>
#include <nlohmann/json.hpp>

namespace dipole::realtime {
namespace {

using delivery::v1::DeliveryItem;
using delivery::v1::NodeDeliveryBatch;

void CopyBatchMetadata(const delivery::v1::DeliveryEnvelope& envelope, const std::string& node_id,
                       NodeDeliveryBatch* batch) {
  batch->set_contract_version(envelope.contract_version());
  batch->set_batch_id(envelope.batch_id() + ":node:" + node_id);
  batch->set_target_node_id(node_id);
  batch->set_source_event_id(envelope.source_event_id());
  batch->set_request_id(envelope.request_id());
  batch->set_trace_id(envelope.trace_id());
  batch->mutable_created_at()->CopyFrom(envelope.created_at());
}

void AddItem(const DeliveryItem& source, std::vector<std::string> connection_ids,
             NodeDeliveryBatch* batch) {
  std::sort(connection_ids.begin(), connection_ids.end());
  auto* item = batch->add_items();
  item->set_delivery_id(source.delivery_id());
  item->set_recipient_user_id(source.recipient_user_id());
  for (auto& connection_id : connection_ids) {
    item->add_connection_ids(std::move(connection_id));
  }
  item->set_event_type(source.event_type());
  item->set_payload_json(source.payload_json());
  item->set_ordering_key(source.ordering_key());
  item->set_mode(source.mode());
}

}  // namespace

ValidationError ParsePresenceHash(
    const std::string& user_id,
    const std::vector<std::pair<std::string, std::string>>& fields,
    std::vector<PresenceConnection>* connections, PresenceHashParseStats* stats) {
  if (user_id.empty() || connections == nullptr || stats == nullptr) {
    return "presence hash user and outputs are required";
  }
  connections->clear();
  *stats = {};
  connections->reserve(fields.size());

  for (const auto& [field_connection_id, raw] : fields) {
    ++stats->observed_records;
    try {
      const auto record = nlohmann::json::parse(raw);
      const auto connection_id = record.value("connection_id", std::string{});
      const auto record_user_id = record.value("user_uuid", std::string{});
      const auto node_id = record.value("node_id", std::string{});
      const auto last_seen_at = record.value("last_seen_at", std::string{});
      google::protobuf::Timestamp timestamp;
      if (field_connection_id.empty() || connection_id != field_connection_id ||
          record_user_id != user_id || node_id.empty() ||
          !google::protobuf::util::TimeUtil::FromString(last_seen_at, &timestamp)) {
        ++stats->malformed_records;
        continue;
      }
      connections->push_back({.connection_id = connection_id,
                              .user_id = record_user_id,
                              .node_id = node_id,
                              .last_seen_unix_ms = timestamp.seconds() * 1000 +
                                                   timestamp.nanos() / 1'000'000});
      ++stats->parsed_records;
    } catch (const nlohmann::json::exception&) {
      ++stats->malformed_records;
    }
  }
  return std::nullopt;
}

ValidationError ProjectPresence(const delivery::v1::DeliveryEnvelope& envelope,
                                const PresenceByUser& presence,
                                const PresenceProjectionPolicy& policy,
                                std::vector<NodeDeliveryBatch>* batches,
                                PresenceProjectionStats* stats) {
  if (batches == nullptr || stats == nullptr) {
    return "presence projection outputs are required";
  }
  batches->clear();
  *stats = {};
  if (auto error = ValidateEnvelope(envelope)) {
    return error;
  }
  if (policy.now_unix_ms <= 0 || policy.ttl_ms <= 0) {
    return "presence projection time and ttl must be positive";
  }

  std::map<std::string, NodeDeliveryBatch> by_node;
  std::unordered_set<std::string> observed_connection_ids;
  for (const auto& source_item : envelope.items()) {
    const auto found = presence.find(source_item.recipient_user_id());
    if (found == presence.end() || found->second.empty()) {
      ++stats->offline_items;
      continue;
    }

    std::map<std::string, std::vector<std::string>> eligible_by_node;
    for (const auto& connection : found->second) {
      ++stats->observed_connections;
      if (connection.connection_id.empty() || connection.node_id.empty() ||
          connection.user_id != source_item.recipient_user_id()) {
        return "presence connection identity is incomplete or mismatched";
      }
      if (!observed_connection_ids.insert(connection.connection_id).second) {
        return "presence connection has duplicate ownership: " + connection.connection_id;
      }
      if (connection.last_seen_unix_ms <= 0 ||
          policy.now_unix_ms - connection.last_seen_unix_ms > policy.ttl_ms) {
        ++stats->stale_connections;
        continue;
      }

      ++stats->eligible_connections;
      eligible_by_node[connection.node_id].push_back(connection.connection_id);
    }

    if (eligible_by_node.empty()) {
      ++stats->offline_items;
      continue;
    }
    for (auto& [node_id, connection_ids] : eligible_by_node) {
      auto [iterator, inserted] = by_node.try_emplace(node_id);
      if (inserted) {
        CopyBatchMetadata(envelope, node_id, &iterator->second);
      }
      AddItem(source_item, std::move(connection_ids), &iterator->second);
    }
  }

  batches->reserve(by_node.size());
  for (auto& [node_id, batch] : by_node) {
    static_cast<void>(node_id);
    if (auto error = ValidateNodeBatch(batch)) {
      return error;
    }
    batches->push_back(std::move(batch));
  }
  return std::nullopt;
}

}  // namespace dipole::realtime
