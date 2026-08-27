#include "presence_projection.hpp"

#include <cstdint>
#include <iostream>
#include <string>
#include <utility>
#include <unordered_map>
#include <vector>

namespace {

using dipole::delivery::v1::DeliveryEnvelope;
using dipole::delivery::v1::DELIVERY_MODE_FULL_EVENT;
using dipole::realtime::PresenceConnection;
using dipole::realtime::PresenceProjectionPolicy;
using dipole::realtime::PresenceProjectionStats;

DeliveryEnvelope Envelope() {
  DeliveryEnvelope envelope;
  envelope.set_contract_version("v1");
  envelope.set_batch_id("batch-1");
  envelope.set_source_event_id("event-1");
  envelope.set_source_topic("dipole.message.direct.created");
  envelope.set_source_partition(1);
  envelope.set_source_offset(9);
  envelope.mutable_created_at()->set_seconds(1'700'000'000);

  auto* item = envelope.add_items();
  item->set_delivery_id("delivery-u1");
  item->set_recipient_user_id("U1");
  item->set_event_type("message.direct.created");
  item->set_payload_json("{\"message_id\":\"M1\"}");
  item->set_ordering_key("direct:U1:U2");
  item->set_mode(DELIVERY_MODE_FULL_EVENT);
  return envelope;
}

int Expect(bool condition, const std::string& message) {
  if (condition) {
    return 0;
  }
  std::cerr << message << '\n';
  return 1;
}

int TestGroupsEligibleConnectionsDeterministically() {
  auto envelope = Envelope();
  std::unordered_map<std::string, std::vector<PresenceConnection>> presence{
      {"U1",
       {
           {.connection_id = "C2", .user_id = "U1", .node_id = "node-b", .last_seen_unix_ms = 9'950},
           {.connection_id = "C1", .user_id = "U1", .node_id = "node-a", .last_seen_unix_ms = 9'900},
           {.connection_id = "C3", .user_id = "U1", .node_id = "node-a", .last_seen_unix_ms = 1'000},
       }},
  };
  PresenceProjectionPolicy policy{.now_unix_ms = 10'000, .ttl_ms = 1'000};
  std::vector<dipole::delivery::v1::NodeDeliveryBatch> batches;
  PresenceProjectionStats stats;

  if (auto error = dipole::realtime::ProjectPresence(envelope, presence, policy, &batches, &stats)) {
    std::cerr << *error << '\n';
    return 1;
  }

  int failures = 0;
  failures += Expect(batches.size() == 2, "expected two node batches");
  failures += Expect(batches[0].target_node_id() == "node-a", "expected stable node ordering");
  failures += Expect(batches[0].items_size() == 1 && batches[0].items(0).connection_ids_size() == 1 &&
                         batches[0].items(0).connection_ids(0) == "C1",
                     "expected only the live node-a connection");
  failures += Expect(batches[1].target_node_id() == "node-b", "expected node-b second");
  failures += Expect(stats.observed_connections == 3 && stats.eligible_connections == 2 &&
                         stats.stale_connections == 1 && stats.offline_items == 0,
                     "expected explicit projection counters");
  return failures;
}

int TestRejectsIdentityAndConnectionDrift() {
  auto envelope = Envelope();
  PresenceProjectionPolicy policy{.now_unix_ms = 10'000, .ttl_ms = 1'000};
  std::vector<dipole::delivery::v1::NodeDeliveryBatch> batches;
  PresenceProjectionStats stats;

  std::unordered_map<std::string, std::vector<PresenceConnection>> wrong_user{
      {"U1", {{.connection_id = "C1", .user_id = "U2", .node_id = "node-a", .last_seen_unix_ms = 9'900}}},
  };
  int failures = 0;
  failures += Expect(dipole::realtime::ProjectPresence(envelope, wrong_user, policy, &batches, &stats).has_value(),
                     "expected mismatched user to fail closed");

  std::unordered_map<std::string, std::vector<PresenceConnection>> duplicate{
      {"U1",
       {
           {.connection_id = "C1", .user_id = "U1", .node_id = "node-a", .last_seen_unix_ms = 9'900},
           {.connection_id = "C1", .user_id = "U1", .node_id = "node-b", .last_seen_unix_ms = 9'950},
       }},
  };
  failures += Expect(dipole::realtime::ProjectPresence(envelope, duplicate, policy, &batches, &stats).has_value(),
                     "expected duplicate connection ownership to fail closed");
  return failures;
}

int TestParsesGoPresenceHashAndCountsMalformedRecords() {
  const std::vector<std::pair<std::string, std::string>> fields{
      {"C1",
       R"({"connection_id":"C1","user_uuid":"U1","node_id":"node-a","last_seen_at":"2023-11-14T22:13:19.900Z"})"},
      {"C2", R"({"connection_id":"drift","user_uuid":"U1","node_id":"node-b","last_seen_at":"2023-11-14T22:13:19Z"})"},
      {"C3", "{"},
  };
  std::vector<PresenceConnection> connections;
  dipole::realtime::PresenceHashParseStats stats;
  const auto error = dipole::realtime::ParsePresenceHash("U1", fields, &connections, &stats);

  int failures = 0;
  failures += Expect(!error.has_value(), "expected malformed records to be isolated");
  failures += Expect(connections.size() == 1 && connections[0].connection_id == "C1" &&
                         connections[0].last_seen_unix_ms == 1'699'999'999'900,
                     "expected Go presence JSON and RFC3339 timestamp parsing");
  failures += Expect(stats.observed_records == 3 && stats.parsed_records == 1 &&
                         stats.malformed_records == 2,
                     "expected malformed Presence counters");
  return failures;
}

}  // namespace

int main() {
  int failures = 0;
  failures += TestGroupsEligibleConnectionsDeterministically();
  failures += TestRejectsIdentityAndConnectionDrift();
  failures += TestParsesGoPresenceHashAndCountsMalformedRecords();
  return failures == 0 ? 0 : 1;
}
