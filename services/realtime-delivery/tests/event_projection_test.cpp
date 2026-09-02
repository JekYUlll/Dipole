#include <cstdlib>
#include <iostream>
#include <string>

#include <nlohmann/json.hpp>

#include "event_projection.hpp"

namespace {

int failures = 0;

void Check(bool condition, const std::string& message) {
  if (!condition) {
    std::cerr << "FAIL: " << message << '\n';
    ++failures;
  }
}

std::string Event(const std::string& event_type, int target_type,
                  const nlohmann::json& extra = nlohmann::json::object()) {
  nlohmann::json payload = {
      {"mutation_type", "created"},
      {"revision", 1},
      {"actor_uuid", "U1"},
      {"message_id", target_type == 0 ? "M42" : "MG42"},
      {"conversation_key", target_type == 0 ? "direct:U1:U2" : "group:G1"},
      {"message_seq", 42},
      {"sender_uuid", "U1"},
      {"target_uuid", target_type == 0 ? "U2" : "G1"},
      {"target_type", target_type},
      {"message_type", 0},
      {"content", target_type == 0 ? "secret body" : "group body"},
      {"sent_at", "2026-08-28T08:00:01Z"},
  };
  payload.update(extra);
  return nlohmann::json({
                            {"event_id", target_type == 0 ? "E42" : "EG42"},
                            {"request_id", "R42"},
                            {"trace_id", "T42"},
                            {"event_type", event_type},
                            {"version", "v1.2"},
                            {"source", "dipole"},
                            {"occurred_at", "2026-08-28T08:00:02Z"},
                            {"payload", payload},
                        })
      .dump();
}

dipole::realtime::KafkaRecord Record(std::string value) {
  return {.topic = "dipole.message.created",
          .partition = 2,
          .offset = 41,
          .value = std::move(value)};
}

nlohmann::json Payload(const dipole::delivery::v1::DeliveryItem& item) {
  return nlohmann::json::parse(item.payload_json());
}

void TestDirectProjection() {
  dipole::delivery::v1::DeliveryEnvelope output;
  const auto error = dipole::realtime::ProjectMessageEvent(
      Record(Event("message.direct.created", 0)), {}, &output);
  Check(!error, "project direct event");
  Check(output.contract_version() == "v1" && output.batch_id() == "shadow:E42:2:41",
        "direct envelope identity");
  Check(output.source_event_id() == "E42" && output.source_topic() == "dipole.message.created" &&
            output.source_partition() == 2 && output.source_offset() == 41,
        "direct Kafka source coordinates");
  Check(output.request_id() == "R42" && output.trace_id() == "T42", "direct correlation");
  Check(output.items_size() == 1, "direct full event count");
  if (output.items_size() != 1) {
    return;
  }
  const auto& item = output.items(0);
  Check(item.delivery_id() == "E42:U2:full" && item.recipient_user_id() == "U2",
        "direct delivery identity");
  Check(item.event_type() == "chat.message" && item.ordering_key() == "user:U2" &&
            item.mode() == dipole::delivery::v1::DELIVERY_MODE_FULL_EVENT,
        "direct delivery metadata");
  const auto payload = Payload(item);
  Check(payload == nlohmann::json({{"message_id", "M42"},
                                   {"message_seq", 42},
                                   {"from_uuid", "U1"},
                                   {"target_uuid", "U2"},
                                   {"target_type", 0},
                                   {"message_type", 0},
                                   {"content", "secret body"},
                                   {"sent_at", "2026-08-28T08:00:01Z"}}),
        "direct payload matches Go WS shape");
  Check(!dipole::realtime::ValidateEnvelope(output), "direct output satisfies delivery contract");
}

void TestDirectTimelineProjection() {
  dipole::delivery::v1::DeliveryEnvelope output;
  const dipole::realtime::ProjectionPolicy policy{.timeline_notify_shadow = true};
  const auto error = dipole::realtime::ProjectMessageEvent(
      Record(Event("message.direct.created", 0)), policy, &output);
  Check(!error && output.items_size() == 2, "project direct timeline shadow");
  if (output.items_size() != 2) {
    return;
  }
  const auto& item = output.items(1);
  Check(item.delivery_id() == "E42:U2:timeline" && item.event_type() == "sync.item.notify.v1" &&
            item.mode() == dipole::delivery::v1::DELIVERY_MODE_TIMELINE_NOTIFY,
        "timeline delivery metadata");
  Check(Payload(item) == nlohmann::json({{"schema_version", "v1"},
                                         {"event_id", "E42"},
                                         {"message_uuid", "M42"},
                                         {"conversation_key", "direct:U1:U2"},
                                         {"message_seq", 42},
                                         {"target_type", 0},
                                         {"target_uuid", "U2"}}),
        "timeline payload matches Go WS shape");
}

void TestPrimaryTimelineProjection() {
  dipole::delivery::v1::DeliveryEnvelope direct;
  const dipole::realtime::ProjectionPolicy policy{.timeline_notify_primary = true};
  const auto direct_error = dipole::realtime::ProjectMessageEvent(
      Record(Event("message.direct.created", 0)), policy, &direct);
  Check(!direct_error && direct.items_size() == 1, "project direct primary timeline notification");
  if (direct.items_size() == 1) {
    Check(direct.items(0).event_type() == "sync.item.notify.v1" &&
              direct.items(0).mode() == dipole::delivery::v1::DELIVERY_MODE_TIMELINE_NOTIFY,
          "direct primary omits full message");
  }

  const nlohmann::json recipients = {{"recipient_uuids", {"U1", "U2", "U3"}}};
  dipole::delivery::v1::DeliveryEnvelope group;
  const auto group_error = dipole::realtime::ProjectMessageEvent(
      Record(Event("message.group.created", 1, recipients)), policy, &group);
  Check(!group_error && group.items_size() == 2, "project group primary timeline notifications");
  for (const auto& item : group.items()) {
    Check(item.event_type() == "sync.item.notify.v1" &&
              item.mode() == dipole::delivery::v1::DELIVERY_MODE_TIMELINE_NOTIFY,
          "group primary omits full message");
  }
}

void TestGroupProjection() {
  const nlohmann::json recipients = {{"recipient_uuids", {"U1", "U2", "U3"}}};
  dipole::delivery::v1::DeliveryEnvelope output;
  const dipole::realtime::ProjectionPolicy policy{.timeline_notify_shadow = true};
  const auto error = dipole::realtime::ProjectMessageEvent(
      Record(Event("message.group.created", 1, recipients)), policy, &output);
  Check(!error && output.items_size() == 4, "normal group excludes sender and adds timeline items");
  if (output.items_size() != 4) {
    return;
  }
  Check(output.items(0).recipient_user_id() == "U2" &&
            output.items(0).mode() == dipole::delivery::v1::DELIVERY_MODE_FULL_EVENT &&
            output.items(1).recipient_user_id() == "U2" &&
            output.items(1).mode() == dipole::delivery::v1::DELIVERY_MODE_TIMELINE_NOTIFY &&
            output.items(2).recipient_user_id() == "U3" &&
            output.items(3).recipient_user_id() == "U3",
        "normal group projection preserves recipient order");
  Check(!dipole::realtime::ValidateEnvelope(output), "normal group output satisfies contract");
}

void TestHotGroupProjection() {
  const nlohmann::json recipients = {{"recipient_uuids", {"U1", "U2"}}};
  dipole::delivery::v1::DeliveryEnvelope output;
  const dipole::realtime::ProjectionPolicy policy{
      .timeline_notify_shadow = true, .hot_group = true, .recent_message_count = 7};
  const auto error = dipole::realtime::ProjectMessageEvent(
      Record(Event("message.group.created", 1, recipients)), policy, &output);
  Check(!error && output.items_size() == 2, "hot group includes sender and suppresses timeline");
  if (output.items_size() != 2) {
    return;
  }
  Check(output.items(0).recipient_user_id() == "U1" &&
            output.items(1).recipient_user_id() == "U2",
        "hot group recipients match Go aggregator");
  for (const auto& item : output.items()) {
    Check(item.event_type() == "group.message.notify" &&
              item.mode() == dipole::delivery::v1::DELIVERY_MODE_HOT_GROUP_NOTIFY,
          "hot group delivery mode");
  }
  Check(Payload(output.items(0)) ==
            nlohmann::json({{"group_uuid", "G1"},
                            {"latest_message_id", "MG42"},
                            {"latest_message_seq", 42},
                            {"message_type", 0},
                            {"preview", "group body"},
                            {"recent_message_count", 7},
                            {"sent_at", "2026-08-28T08:00:01Z"},
                            {"sender_uuid", "U1"}}),
        "hot group payload matches Go WS shape");
}

void TestHotGroupProjectionFromDurableFanoutFact() {
  const nlohmann::json payload = {{"recipient_uuids", {"U1", "U2"}},
                                  {"sync_fanout", false}};
  dipole::delivery::v1::DeliveryEnvelope output;
  const auto error = dipole::realtime::ProjectMessageEvent(
      Record(Event("message.group.created", 1, payload)), {}, &output);
  Check(!error && output.items_size() == 2, "sync_fanout=false selects hot-group projection");
  if (output.items_size() == 2) {
    Check(output.items(0).mode() ==
                  dipole::delivery::v1::DELIVERY_MODE_HOT_GROUP_NOTIFY &&
              output.items(1).mode() ==
                  dipole::delivery::v1::DELIVERY_MODE_HOT_GROUP_NOTIFY,
          "durable fanout fact suppresses full messages");
    Check(Payload(output.items(0)).at("recent_message_count") == 0,
          "Redis-free hot projection uses bounded unknown count");
  }
}

void TestFileProjection() {
  const nlohmann::json file = {{"message_type", 1},
                               {"content", ""},
                               {"file_id", "F1"},
                               {"file_name", "report.pdf"},
                               {"file_size", 512},
                               {"file_content_type", "application/pdf"},
                               {"file_expires_at", "2026-08-29T08:00:00Z"}};
  dipole::delivery::v1::DeliveryEnvelope output;
  const auto error = dipole::realtime::ProjectMessageEvent(
      Record(Event("message.direct.created", 0, file)), {}, &output);
  Check(!error, "project file message");
  const auto payload = Payload(output.items(0));
  Check(payload.at("file") == nlohmann::json({{"file_id", "F1"},
                                               {"file_name", "report.pdf"},
                                               {"file_size", 512},
                                               {"download_path", "/api/v1/files/F1/download"},
                                               {"content_path", ""},
                                               {"content_type", "application/pdf"},
                                               {"file_expires_at", "2026-08-29T08:00:00Z"}}),
        "file payload matches Go WS shape");
}

void TestLegacyCreatedProjection() {
  auto legacy = nlohmann::json::parse(Event("message.direct.created", 0));
  legacy["payload"].erase("mutation_type");
  legacy["payload"].erase("revision");
  legacy["payload"].erase("actor_uuid");
  legacy["payload"].erase("message_seq");
  legacy["legacy_additive_field"] = "accepted";

  dipole::delivery::v1::DeliveryEnvelope output;
  const auto error = dipole::realtime::ProjectMessageEvent(Record(legacy.dump()), {}, &output);
  Check(!error && output.items_size() == 1, "legacy created event remains readable");
  if (output.items_size() == 1) {
    Check(!Payload(output.items(0)).contains("message_seq"),
          "legacy full payload omits unavailable sequence like Go");
  }
}

void TestInvalidEvents() {
  const auto expect_error = [](const dipole::realtime::KafkaRecord& record,
                               const dipole::realtime::ProjectionPolicy& policy,
                               const std::string& expected) {
    dipole::delivery::v1::DeliveryEnvelope output;
    const auto error = dipole::realtime::ProjectMessageEvent(record, policy, &output);
    Check(error.has_value() && error->find(expected) != std::string::npos, expected);
  };

  expect_error(Record("{"), {}, "decode event");

  auto unsupported = nlohmann::json::parse(Event("message.direct.created", 0));
  unsupported["version"] = "v2";
  expect_error(Record(unsupported.dump()), {}, "version");

  expect_error(Record(Event("message.direct.edited", 0)), {}, "event type");
  expect_error(Record(Event("message.group.created", 0)), {}, "target type");
  expect_error(Record(Event("message.direct.created", 1)), {}, "target type");

  const nlohmann::json duplicates = {{"recipient_uuids", {"U1", "U2", "U2"}}};
  expect_error(Record(Event("message.group.created", 1, duplicates)), {}, "duplicated");

  auto missing_sequence = nlohmann::json::parse(Event("message.direct.created", 0));
  missing_sequence["payload"]["message_seq"] = 0;
  expect_error(Record(missing_sequence.dump()), {.timeline_notify_shadow = true}, "message_seq");

  expect_error(Record(Event("message.direct.created", 0)),
               {.timeline_notify_shadow = true, .timeline_notify_primary = true}, "mutually exclusive");

  auto wrong_source = nlohmann::json::parse(Event("message.direct.created", 0));
  wrong_source["source"] = "foreign";
  expect_error(Record(wrong_source.dump()), {}, "source");

  auto invalid_record = Record(Event("message.direct.created", 0));
  invalid_record.offset = -1;
  expect_error(invalid_record, {}, "coordinates");

  auto huge_sequence = nlohmann::json::parse(Event("message.direct.created", 0));
  huge_sequence["payload"]["message_seq"] =
      nlohmann::json::parse("18446744073709551615");
  expect_error(Record(huge_sequence.dump()), {}, "message_seq");

  auto invalid_file = nlohmann::json::parse(Event("message.direct.created", 0));
  invalid_file["payload"]["message_type"] = 1;
  invalid_file["payload"]["file_name"] = 42;
  expect_error(Record(invalid_file.dump()), {}, "file_name");
}

}  // namespace

int main() {
  try {
    TestDirectProjection();
    TestDirectTimelineProjection();
    TestPrimaryTimelineProjection();
    TestGroupProjection();
    TestHotGroupProjection();
    TestHotGroupProjectionFromDurableFanoutFact();
    TestFileProjection();
    TestLegacyCreatedProjection();
    TestInvalidEvents();
  } catch (const std::exception& error) {
    std::cerr << "FAIL: unexpected exception: " << error.what() << '\n';
    ++failures;
  }
  return failures == 0 ? EXIT_SUCCESS : EXIT_FAILURE;
}
