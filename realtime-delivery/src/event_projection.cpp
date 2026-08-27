#include "event_projection.hpp"

#include <algorithm>
#include <cctype>
#include <cstdint>
#include <limits>
#include <optional>
#include <string>
#include <string_view>
#include <unordered_set>
#include <utility>
#include <vector>

#include <google/protobuf/util/time_util.h>
#include <nlohmann/json.hpp>

namespace dipole::realtime {
namespace {

using Json = nlohmann::json;
using delivery::v1::DeliveryEnvelope;
using delivery::v1::DeliveryItem;
using delivery::v1::DeliveryMode;

constexpr int kDirectTarget = 0;
constexpr int kGroupTarget = 1;
constexpr int kFileMessage = 1;

struct MessageEvent {
  std::string event_id;
  std::string request_id;
  std::string trace_id;
  std::string event_type;
  std::string occurred_at;
  std::string message_id;
  std::string conversation_key;
  std::uint64_t message_seq = 0;
  std::string sender_uuid;
  std::string target_uuid;
  int target_type = 0;
  int message_type = 0;
  std::string content;
  std::string sent_at;
  std::vector<std::string> recipients;
  Json payload;
};

bool IsBlank(std::string_view value) {
  return value.empty() || value.find_first_not_of(" \t\r\n") == std::string_view::npos;
}

bool IsSupportedVersion(std::string_view version) {
  if (!version.starts_with("v1")) {
    return false;
  }
  if (version.size() == 2) {
    return true;
  }
  if (version[2] != '.') {
    return false;
  }
  bool segment_has_digit = false;
  for (std::size_t index = 3; index < version.size(); ++index) {
    const char value = version[index];
    if (value == '.') {
      if (!segment_has_digit) {
        return false;
      }
      segment_has_digit = false;
      continue;
    }
    if (std::isdigit(static_cast<unsigned char>(value)) == 0) {
      return false;
    }
    segment_has_digit = true;
  }
  return segment_has_digit;
}

ValidationError RequiredString(const Json& value, std::string_view key, std::string* output,
                               bool allow_empty = false) {
  const auto iterator = value.find(key);
  if (iterator == value.end() || !iterator->is_string()) {
    return "event field " + std::string(key) + " must be a string";
  }
  *output = iterator->get<std::string>();
  if (!allow_empty && IsBlank(*output)) {
    return "event field " + std::string(key) + " is empty";
  }
  return std::nullopt;
}

ValidationError OptionalString(const Json& value, std::string_view key, std::string* output) {
  const auto iterator = value.find(key);
  if (iterator == value.end()) {
    output->clear();
    return std::nullopt;
  }
  if (!iterator->is_string()) {
    return "event field " + std::string(key) + " must be a string";
  }
  *output = iterator->get<std::string>();
  return std::nullopt;
}

ValidationError RequiredInteger(const Json& value, std::string_view key, std::int64_t minimum,
                                std::int64_t maximum, std::int64_t* output) {
  const auto iterator = value.find(key);
  if (iterator == value.end() || !iterator->is_number_integer()) {
    return "event field " + std::string(key) + " must be an integer";
  }
  std::int64_t parsed = 0;
  if (iterator->is_number_unsigned()) {
    const auto unsigned_value = iterator->get<std::uint64_t>();
    if (unsigned_value > static_cast<std::uint64_t>(maximum)) {
      return "event field " + std::string(key) + " is out of range";
    }
    parsed = static_cast<std::int64_t>(unsigned_value);
  } else {
    parsed = iterator->get<std::int64_t>();
  }
  if (parsed < minimum || parsed > maximum) {
    return "event field " + std::string(key) + " is out of range";
  }
  *output = parsed;
  return std::nullopt;
}

ValidationError ParseTimestamp(std::string_view value, google::protobuf::Timestamp* output,
                               std::string_view field);

ValidationError ValidateOptionalPayloadFields(const Json& payload) {
  for (const std::string_view key : {"client_message_id", "file_id", "file_name", "file_url",
                                     "file_content_type"}) {
    std::string ignored;
    if (auto error = OptionalString(payload, key, &ignored); error) {
      return error;
    }
  }
  if (payload.contains("file_size")) {
    std::int64_t ignored = 0;
    if (auto error = RequiredInteger(payload, "file_size", 0,
                                     std::numeric_limits<std::int64_t>::max(), &ignored);
        error) {
      return error;
    }
  }
  if (payload.contains("file_expires_at")) {
    std::string expires_at;
    if (auto error = RequiredString(payload, "file_expires_at", &expires_at); error) {
      return error;
    }
    google::protobuf::Timestamp timestamp;
    if (auto error = ParseTimestamp(expires_at, &timestamp, "file_expires_at"); error) {
      return error;
    }
  }
  if (const auto sync_fanout = payload.find("sync_fanout");
      sync_fanout != payload.end() && !sync_fanout->is_boolean()) {
    return "event field sync_fanout must be a boolean";
  }
  return std::nullopt;
}

ValidationError ParseTimestamp(std::string_view value, google::protobuf::Timestamp* output,
                               std::string_view field) {
  if (output == nullptr ||
      !google::protobuf::util::TimeUtil::FromString(std::string(value), output)) {
    return "event field " + std::string(field) + " is not RFC3339";
  }
  return std::nullopt;
}

ValidationError DecodeEvent(const KafkaRecord& record, MessageEvent* output) {
  if (output == nullptr) {
    return "message event destination is required";
  }
  if (IsBlank(record.topic) || record.partition < 0 || record.offset < 0) {
    return "Kafka record topic and coordinates are invalid";
  }

  Json root;
  try {
    root = Json::parse(record.value);
  } catch (const Json::exception& error) {
    return "decode event JSON: " + std::string(error.what());
  }
  if (!root.is_object()) {
    return "decode event: root must be an object";
  }

  if (auto error = RequiredString(root, "event_id", &output->event_id); error) {
    return error;
  }
  if (auto error = OptionalString(root, "request_id", &output->request_id); error) {
    return error;
  }
  if (auto error = OptionalString(root, "trace_id", &output->trace_id); error) {
    return error;
  }
  if (auto error = RequiredString(root, "event_type", &output->event_type); error) {
    return error;
  }
  if (output->event_type != "message.direct.created" &&
      output->event_type != "message.group.created") {
    return "message event type is unsupported: " + output->event_type;
  }

  std::string version;
  if (auto error = RequiredString(root, "version", &version); error) {
    return error;
  }
  if (!IsSupportedVersion(version)) {
    return "message event version is unsupported: " + version;
  }
  std::string source;
  if (auto error = RequiredString(root, "source", &source); error) {
    return error;
  }
  if (source != "dipole") {
    return "message event source is unsupported: " + source;
  }
  if (auto error = RequiredString(root, "occurred_at", &output->occurred_at); error) {
    return error;
  }
  google::protobuf::Timestamp occurred_at;
  if (auto error = ParseTimestamp(output->occurred_at, &occurred_at, "occurred_at"); error) {
    return error;
  }

  const auto payload = root.find("payload");
  if (payload == root.end() || !payload->is_object()) {
    return "message event payload must be an object";
  }
  output->payload = *payload;

  std::string mutation;
  if (auto error = OptionalString(*payload, "mutation_type", &mutation); error) {
    return error;
  }
  if (!mutation.empty() && mutation != "created") {
    return "message mutation type does not match created event";
  }
  std::int64_t revision = 1;
  if (payload->contains("revision") &&
      (RequiredInteger(*payload, "revision", 1, 1, &revision).has_value())) {
    return "created message revision must be 1";
  }
  std::string actor;
  if (auto error = OptionalString(*payload, "actor_uuid", &actor); error) {
    return error;
  }

  if (auto error = RequiredString(*payload, "message_id", &output->message_id); error) {
    return error;
  }
  if (auto error = RequiredString(*payload, "conversation_key", &output->conversation_key); error) {
    return error;
  }
  std::int64_t sequence = 0;
  if (payload->contains("message_seq")) {
    if (auto error = RequiredInteger(*payload, "message_seq", 0,
                                     std::numeric_limits<std::int64_t>::max(), &sequence);
        error) {
      return error;
    }
  }
  output->message_seq = static_cast<std::uint64_t>(sequence);
  if (auto error = RequiredString(*payload, "sender_uuid", &output->sender_uuid); error) {
    return error;
  }
  if (actor.empty()) {
    actor = output->sender_uuid;
  }
  if (IsBlank(actor)) {
    return "created message actor is required";
  }
  if (auto error = RequiredString(*payload, "target_uuid", &output->target_uuid); error) {
    return error;
  }
  std::int64_t target_type = 0;
  if (auto error = RequiredInteger(*payload, "target_type", kDirectTarget, kGroupTarget,
                                   &target_type);
      error) {
    return error;
  }
  output->target_type = static_cast<int>(target_type);
  const int expected_target =
      output->event_type == "message.direct.created" ? kDirectTarget : kGroupTarget;
  if (output->target_type != expected_target) {
    return "message event target type does not match event channel";
  }
  std::int64_t message_type = 0;
  if (auto error = RequiredInteger(*payload, "message_type",
                                   std::numeric_limits<std::int8_t>::min(),
                                   std::numeric_limits<std::int8_t>::max(), &message_type);
      error) {
    return error;
  }
  output->message_type = static_cast<int>(message_type);
  if (auto error = RequiredString(*payload, "content", &output->content, true); error) {
    return error;
  }
  if (auto error = RequiredString(*payload, "sent_at", &output->sent_at); error) {
    return error;
  }
  google::protobuf::Timestamp sent_at;
  if (auto error = ParseTimestamp(output->sent_at, &sent_at, "sent_at"); error) {
    return error;
  }
  if (auto error = ValidateOptionalPayloadFields(*payload); error) {
    return error;
  }

  const auto recipients = payload->find("recipient_uuids");
  if (recipients != payload->end()) {
    if (!recipients->is_array()) {
      return "event field recipient_uuids must be an array";
    }
    std::unordered_set<std::string> unique;
    for (const auto& recipient : *recipients) {
      if (!recipient.is_string() || IsBlank(recipient.get_ref<const std::string&>())) {
        return "event recipient is empty or invalid";
      }
      auto uuid = recipient.get<std::string>();
      if (!unique.insert(uuid).second) {
        return "event recipient is duplicated: " + uuid;
      }
      output->recipients.push_back(std::move(uuid));
    }
  }
  if (output->target_type == kGroupTarget && output->recipients.empty()) {
    return "group message recipient_uuids is empty";
  }
  return std::nullopt;
}

Json FilePayload(const MessageEvent& event) {
  const auto string_value = [&event](std::string_view key) {
    const auto iterator = event.payload.find(key);
    return iterator != event.payload.end() && iterator->is_string()
               ? iterator->get<std::string>()
               : std::string();
  };
  const auto integer_value = [&event](std::string_view key) {
    const auto iterator = event.payload.find(key);
    return iterator != event.payload.end() && iterator->is_number_integer()
               ? iterator->get<std::int64_t>()
               : std::int64_t{0};
  };
  const auto file_id = string_value("file_id");
  Json file = {{"file_id", file_id},
               {"file_name", string_value("file_name")},
               {"file_size", integer_value("file_size")},
               {"download_path", "/api/v1/files/" + file_id + "/download"},
               {"content_path", ""},
               {"content_type", string_value("file_content_type")}};
  const auto expires_at = string_value("file_expires_at");
  if (!expires_at.empty()) {
    file["file_expires_at"] = expires_at;
  }
  return file;
}

Json FullMessagePayload(const MessageEvent& event) {
  Json payload = {{"message_id", event.message_id},
                  {"from_uuid", event.sender_uuid},
                  {"target_uuid", event.target_uuid},
                  {"target_type", event.target_type},
                  {"message_type", event.message_type},
                  {"content", event.content},
                  {"sent_at", event.sent_at}};
  if (event.message_seq != 0) {
    payload["message_seq"] = event.message_seq;
  }
  if (event.message_type == kFileMessage) {
    payload["file"] = FilePayload(event);
  }
  return payload;
}

Json TimelinePayload(const MessageEvent& event) {
  return {{"schema_version", "v1"},
          {"event_id", event.event_id},
          {"message_uuid", event.message_id},
          {"conversation_key", event.conversation_key},
          {"message_seq", event.message_seq},
          {"target_type", event.target_type},
          {"target_uuid", event.target_uuid}};
}

Json HotGroupPayload(const MessageEvent& event, int recent_message_count) {
  std::string preview = event.content;
  if (event.message_type == kFileMessage) {
    const auto file_name = event.payload.value("file_name", std::string());
    preview = file_name.empty() ? "[文件]" : "[文件] " + file_name;
  }
  return {{"group_uuid", event.target_uuid},
          {"latest_message_id", event.message_id},
          {"latest_message_seq", event.message_seq},
          {"message_type", event.message_type},
          {"preview", preview},
          {"recent_message_count", recent_message_count},
          {"sent_at", event.sent_at},
          {"sender_uuid", event.sender_uuid}};
}

struct ItemProjection {
  std::string_view suffix;
  std::string_view event_type;
  DeliveryMode mode;
};

void AddItem(DeliveryEnvelope* output, const MessageEvent& event, std::string_view recipient,
             const ItemProjection& projection, const Json& payload) {
  DeliveryItem* item = output->add_items();
  item->set_delivery_id(event.event_id + ":" + std::string(recipient) + ":" +
                        std::string(projection.suffix));
  item->set_recipient_user_id(std::string(recipient));
  item->set_event_type(std::string(projection.event_type));
  item->set_payload_json(payload.dump());
  item->set_ordering_key("user:" + std::string(recipient));
  item->set_mode(projection.mode);
}

ValidationError ProjectDirect(const MessageEvent& event, const ProjectionPolicy& policy,
                              DeliveryEnvelope* output) {
  AddItem(output, event, event.target_uuid,
          {.suffix = "full",
           .event_type = "chat.message",
           .mode = DeliveryMode::DELIVERY_MODE_FULL_EVENT},
          FullMessagePayload(event));
  if (policy.timeline_notify_shadow) {
    if (event.message_seq == 0) {
      return "timeline projection requires message_seq";
    }
    AddItem(output, event, event.target_uuid,
            {.suffix = "timeline",
             .event_type = "sync.item.notify.v1",
             .mode = DeliveryMode::DELIVERY_MODE_TIMELINE_NOTIFY},
            TimelinePayload(event));
  }
  return std::nullopt;
}

ValidationError ProjectGroup(const MessageEvent& event, const ProjectionPolicy& policy,
                             DeliveryEnvelope* output) {
  if (policy.hot_group) {
    if (event.message_seq == 0 || policy.recent_message_count < 0) {
      return "hot-group projection requires message_seq and non-negative recent_message_count";
    }
    const auto payload = HotGroupPayload(event, policy.recent_message_count);
    for (const auto& recipient : event.recipients) {
      AddItem(output, event, recipient,
              {.suffix = "hot",
               .event_type = "group.message.notify",
               .mode = DeliveryMode::DELIVERY_MODE_HOT_GROUP_NOTIFY},
              payload);
    }
    return std::nullopt;
  }

  for (const auto& recipient : event.recipients) {
    if (recipient == event.sender_uuid) {
      continue;
    }
    AddItem(output, event, recipient,
            {.suffix = "full",
             .event_type = "chat.message",
             .mode = DeliveryMode::DELIVERY_MODE_FULL_EVENT},
            FullMessagePayload(event));
    if (policy.timeline_notify_shadow) {
      if (event.message_seq == 0) {
        return "timeline projection requires message_seq";
      }
      AddItem(output, event, recipient,
              {.suffix = "timeline",
               .event_type = "sync.item.notify.v1",
               .mode = DeliveryMode::DELIVERY_MODE_TIMELINE_NOTIFY},
              TimelinePayload(event));
    }
  }
  if (output->items().empty()) {
    return "group projection has no recipient after sender exclusion";
  }
  return std::nullopt;
}

}  // namespace

ValidationError ProjectMessageEvent(const KafkaRecord& record, const ProjectionPolicy& policy,
                                    DeliveryEnvelope* output) {
  if (output == nullptr) {
    return "delivery envelope destination is required";
  }
  output->Clear();

  MessageEvent event;
  if (auto error = DecodeEvent(record, &event); error) {
    return error;
  }
  if (policy.hot_group && event.target_type != kGroupTarget) {
    return "hot-group policy requires a group event";
  }

  output->set_contract_version(kContractVersion);
  output->set_batch_id("shadow:" + event.event_id + ":" + std::to_string(record.partition) + ":" +
                       std::to_string(record.offset));
  output->set_source_event_id(event.event_id);
  output->set_source_topic(record.topic);
  output->set_source_partition(record.partition);
  output->set_source_offset(record.offset);
  output->set_request_id(event.request_id);
  output->set_trace_id(event.trace_id);
  if (auto error = ParseTimestamp(event.occurred_at, output->mutable_created_at(), "occurred_at");
      error) {
    return error;
  }

  ValidationError error;
  if (event.target_type == kDirectTarget) {
    error = ProjectDirect(event, policy, output);
  } else {
    error = ProjectGroup(event, policy, output);
  }
  if (error) {
    output->Clear();
    return error;
  }
  if (error = ValidateEnvelope(*output); error) {
    output->Clear();
    return "projected delivery contract is invalid: " + *error;
  }
  return std::nullopt;
}

}  // namespace dipole::realtime
