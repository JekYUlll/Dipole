#include <cstdlib>
#include <deque>
#include <iostream>
#include <optional>
#include <string>
#include <utility>
#include <vector>

#include <nlohmann/json.hpp>

#include "shadow_runner.hpp"

namespace {

int failures = 0;

void Check(bool condition, const std::string& message) {
  if (!condition) {
    std::cerr << "FAIL: " << message << '\n';
    ++failures;
  }
}

std::string DirectEvent() {
  return nlohmann::json({
                            {"event_id", "E100"},
                            {"event_type", "message.direct.created"},
                            {"version", "v1"},
                            {"source", "dipole"},
                            {"occurred_at", "2026-08-28T09:00:00Z"},
                            {"payload",
                             {{"mutation_type", "created"},
                              {"revision", 1},
                              {"actor_uuid", "U1"},
                              {"message_id", "M100"},
                              {"conversation_key", "direct:U1:U2"},
                              {"message_seq", 100},
                              {"sender_uuid", "U1"},
                              {"target_uuid", "U2"},
                              {"target_type", 0},
                              {"message_type", 0},
                              {"content", "body must not enter evidence"},
                              {"sent_at", "2026-08-28T09:00:00Z"}}},
                        })
      .dump();
}

dipole::realtime::KafkaRecord Record(std::string value = DirectEvent()) {
  return {.topic = "dipole.message.direct.created",
          .partition = 1,
          .offset = 99,
          .value = std::move(value)};
}

dipole::realtime::PollResult PolledRecord(std::string value = DirectEvent()) {
  dipole::realtime::PollResult result;
  result.status = dipole::realtime::PollStatus::kRecord;
  result.record = Record(std::move(value));
  return result;
}

dipole::realtime::PollResult PollTimeout() {
  return {};
}

dipole::realtime::PollResult PollError(std::string error) {
  dipole::realtime::PollResult result;
  result.status = dipole::realtime::PollStatus::kError;
  result.error = std::move(error);
  return result;
}

class FakeConsumer final : public dipole::realtime::ShadowRecordConsumer {
 public:
  dipole::realtime::PollResult Poll(int timeout_ms) override {
    last_timeout_ms = timeout_ms;
    if (results.empty()) {
      return PollTimeout();
    }
    auto result = std::move(results.front());
    results.pop_front();
    return result;
  }

  dipole::realtime::ValidationError Commit(const dipole::realtime::KafkaRecord& record) override {
    commit_attempts.push_back(record.offset);
    return commit_error;
  }

  std::size_t AssignmentCount() const override { return assignment_count; }

  std::deque<dipole::realtime::PollResult> results;
  std::vector<std::int64_t> commit_attempts;
  dipole::realtime::ValidationError commit_error;
  std::size_t assignment_count = 2;
  int last_timeout_ms = 0;
};

class FakeEvidenceSink final : public dipole::realtime::ShadowEvidenceSink {
 public:
  dipole::realtime::ValidationError Append(
      const dipole::realtime::ShadowEvidence& evidence) override {
    entries.push_back(evidence);
    return append_error;
  }

  std::vector<dipole::realtime::ShadowEvidence> entries;
  dipole::realtime::ValidationError append_error;
};

class FakePresenceReader final : public dipole::realtime::PresenceReader {
 public:
  dipole::realtime::ValidationError ReadUsers(
      const std::vector<std::string>& user_ids,
      dipole::realtime::PresenceReadResult* result) override {
    requested_users = user_ids;
    if (read_error) return read_error;
    *result = read_result;
    return std::nullopt;
  }

  std::vector<std::string> requested_users;
  dipole::realtime::PresenceReadResult read_result;
  dipole::realtime::ValidationError read_error;
};

class FakeNodeTransport final : public dipole::realtime::NodeBatchTransport {
 public:
  dipole::realtime::ValidationError Observe(
      const std::vector<dipole::delivery::v1::NodeDeliveryBatch>& batches,
      dipole::realtime::NodeTransportStats* stats) override {
    observed_batches = batches;
    *stats = response;
    return error;
  }

  std::vector<dipole::delivery::v1::NodeDeliveryBatch> observed_batches;
  dipole::realtime::NodeTransportStats response;
  dipole::realtime::ValidationError error;
};

void TestProjectedRecordCommitsAfterEvidence() {
  FakeConsumer consumer;
  consumer.results.push_back(PolledRecord());
  FakeEvidenceSink sink;
  dipole::realtime::ShadowRunner runner(&consumer, &sink, 250);

  const auto error = runner.RunOnce({.timeline_notify_shadow = true});
  Check(!error, "projected record succeeds");
  Check(consumer.last_timeout_ms == 250, "runner passes bounded poll timeout");
  Check(consumer.commit_attempts == std::vector<std::int64_t>{99},
        "projected record commits once");
  Check(sink.entries.size() == 1, "projected record writes one evidence entry");
  if (sink.entries.size() == 1) {
    const auto& evidence = sink.entries.front();
    Check(evidence.outcome == dipole::realtime::ShadowOutcome::kProjected &&
              evidence.source_event_id == "E100" && evidence.batch_id == "shadow:E100:1:99" &&
              evidence.message_type == 0 && evidence.item_count == 2,
          "projected evidence contains bounded identifiers and counts");
    Check(evidence.error_code.empty() && evidence.topic == "dipole.message.direct.created" &&
              evidence.partition == 1 && evidence.offset == 99,
          "projected evidence binds Kafka coordinates");
  }
  const auto stats = runner.Stats();
  Check(stats.polled == 1 && stats.projected == 1 && stats.rejected == 0 &&
            stats.evidence_written == 1 && stats.committed == 1,
        "projected stats advance");
  Check(runner.Ready(), "assigned healthy consumer is ready");
}

void TestPoisonRecordWritesEvidenceAndCommits() {
  FakeConsumer consumer;
  consumer.results.push_back(PolledRecord("{"));
  FakeEvidenceSink sink;
  dipole::realtime::ShadowRunner runner(&consumer, &sink, 100);

  const auto error = runner.RunOnce({});
  Check(!error, "poison record is isolated in shadow evidence");
  Check(consumer.commit_attempts == std::vector<std::int64_t>{99},
        "poison record commits after evidence");
  Check(sink.entries.size() == 1 &&
            sink.entries.front().outcome == dipole::realtime::ShadowOutcome::kRejected &&
            sink.entries.front().error_code == "invalid_event" &&
            sink.entries.front().source_event_id.empty() && sink.entries.front().item_count == 0,
        "poison evidence excludes body and unstable diagnostics");
  const auto stats = runner.Stats();
  Check(stats.rejected == 1 && stats.committed == 1, "poison stats advance");
}

void TestPresenceProjectionAddsBoundedEvidence() {
  FakeConsumer consumer;
  consumer.results.push_back(PolledRecord());
  FakeEvidenceSink sink;
  FakePresenceReader presence;
  presence.read_result.by_user["U2"] = {
      {.connection_id = "C1", .user_id = "U2", .node_id = "node-a", .last_seen_unix_ms = 9'900},
      {.connection_id = "C2", .user_id = "U2", .node_id = "node-b", .last_seen_unix_ms = 1'000},
  };
  dipole::realtime::ShadowRunner runner(&consumer, &sink, 100, &presence);

  const auto error = runner.RunOnce({}, dipole::realtime::PresenceProjectionPolicy{
                                            .now_unix_ms = 10'000, .ttl_ms = 1'000});
  Check(!error, "Presence projection succeeds");
  Check(presence.requested_users == std::vector<std::string>{"U2"},
        "Presence reads unique projected recipient");
  Check(sink.entries.size() == 1 && sink.entries[0].node_batch_count == 1 &&
            sink.entries[0].presence_observed == 2 && sink.entries[0].presence_eligible == 1 &&
            sink.entries[0].presence_stale == 1 && sink.entries[0].offline_item_count == 0,
        "Presence evidence contains aggregate routing counts");
  Check(consumer.commit_attempts == std::vector<std::int64_t>{99},
        "Presence evidence precedes commit");
}

void TestPresenceReadFailureKeepsOffsetUncommitted() {
  FakeConsumer consumer;
  consumer.results.push_back(PolledRecord());
  FakeEvidenceSink sink;
  FakePresenceReader presence;
  presence.read_error = "Redis unavailable";
  dipole::realtime::ShadowRunner runner(&consumer, &sink, 100, &presence);

  const auto error = runner.RunOnce({}, dipole::realtime::PresenceProjectionPolicy{
                                            .now_unix_ms = 10'000, .ttl_ms = 1'000});
  Check(error.has_value() && error->find("Presence") != std::string::npos,
        "Presence read error reaches caller");
  Check(sink.entries.empty() && consumer.commit_attempts.empty(),
        "Presence read failure has no evidence or commit");
  Check(runner.Stats().presence_read_errors == 1 && !runner.Ready(),
        "Presence read failure removes readiness");
}

void TestNodeTransportPrecedesEvidenceAndCommit() {
  FakeConsumer consumer;
  consumer.results.push_back(PolledRecord());
  FakeEvidenceSink sink;
  FakePresenceReader presence;
  presence.read_result.by_user["U2"] = {
      {.connection_id = "C1", .user_id = "U2", .node_id = "node-a", .last_seen_unix_ms = 9'900}};
  FakeNodeTransport transport;
  transport.response = {.requested = 1, .observed = 1, .duplicate = 0, .rejected = 0,
                        .backpressured = 0};
  dipole::realtime::ShadowRunner runner(&consumer, &sink, 100, &presence, &transport);

  const auto error = runner.RunOnce({}, dipole::realtime::PresenceProjectionPolicy{
                                            .now_unix_ms = 10'000, .ttl_ms = 1'000});
  Check(!error && transport.observed_batches.size() == 1,
        "node transport observes projected batch");
  Check(sink.entries.size() == 1 && sink.entries[0].transport_requested == 1 &&
            sink.entries[0].transport_observed == 1 && sink.entries[0].error_code.empty(),
        "node transport evidence is bounded");
  Check(consumer.commit_attempts == std::vector<std::int64_t>{99},
        "successful node observation precedes offset commit");
}

void TestNodeTransportFailureWritesDeferredEvidenceWithoutCommit() {
  FakeConsumer consumer;
  consumer.results.push_back(PolledRecord());
  FakeEvidenceSink sink;
  FakePresenceReader presence;
  presence.read_result.by_user["U2"] = {
      {.connection_id = "C1", .user_id = "U2", .node_id = "node-a", .last_seen_unix_ms = 9'900}};
  FakeNodeTransport transport;
  transport.response = {.requested = 1, .observed = 0, .duplicate = 0, .rejected = 0,
                        .backpressured = 1};
  transport.error = "backpressured";
  dipole::realtime::ShadowRunner runner(&consumer, &sink, 100, &presence, &transport);

  const auto error = runner.RunOnce({}, dipole::realtime::PresenceProjectionPolicy{
                                            .now_unix_ms = 10'000, .ttl_ms = 1'000});
  Check(error.has_value() && error->find("transport") != std::string::npos,
        "node transport failure reaches caller");
  Check(sink.entries.size() == 1 &&
            sink.entries[0].outcome == dipole::realtime::ShadowOutcome::kDeferred &&
            sink.entries[0].error_code == "node_transport" &&
            sink.entries[0].transport_backpressured == 1,
        "transport failure writes low-sensitive deferred evidence");
  Check(consumer.commit_attempts.empty() && runner.Stats().transport_errors == 1 && !runner.Ready(),
        "transport failure keeps offset and removes readiness");
}

void TestNodeTransportFailureRetriesPendingRecordWithoutPoll() {
  FakeConsumer consumer;
  consumer.results.push_back(PolledRecord());
  FakeEvidenceSink sink;
  FakePresenceReader presence;
  presence.read_result.by_user["U2"] = {
      {.connection_id = "C1", .user_id = "U2", .node_id = "node-a", .last_seen_unix_ms = 9'900}};
  FakeNodeTransport transport;
  transport.response = {.requested = 1, .observed = 0, .duplicate = 0, .rejected = 0,
                        .backpressured = 1};
  transport.error = "backpressured";
  dipole::realtime::ShadowRunner runner(&consumer, &sink, 100, &presence, &transport);
  const dipole::realtime::PresenceProjectionPolicy policy{
      .now_unix_ms = 10'000, .ttl_ms = 1'000};

  Check(runner.RunOnce({}, policy).has_value(), "first transport attempt is deferred");
  transport.response = {.requested = 1, .observed = 1, .duplicate = 0, .rejected = 0,
                        .backpressured = 0};
  transport.error = std::nullopt;
  const auto retry_error = runner.RunOnce({}, policy);

  Check(!retry_error, "pending record succeeds without another Kafka poll result");
  Check(consumer.results.empty() && consumer.commit_attempts == std::vector<std::int64_t>{99},
        "pending retry commits the original record once");
  Check(sink.entries.size() == 2 &&
            sink.entries[0].outcome == dipole::realtime::ShadowOutcome::kDeferred &&
            sink.entries[1].outcome == dipole::realtime::ShadowOutcome::kProjected &&
            sink.entries[1].transport_observed == 1,
        "pending retry preserves deferred and recovery evidence");
  Check(runner.Stats().polled == 1 && runner.Stats().projected == 1 &&
            runner.Stats().transport_errors == 1 && runner.Stats().committed == 1 &&
            runner.Ready(),
        "pending retry counts one projected record and restores readiness");
}

void TestInvalidPresenceWritesEvidenceAndCommits() {
  FakeConsumer consumer;
  consumer.results.push_back(PolledRecord());
  FakeEvidenceSink sink;
  FakePresenceReader presence;
  presence.read_result.by_user["U2"] = {
      {.connection_id = "C1", .user_id = "U-drift", .node_id = "node-a", .last_seen_unix_ms = 9'900}};
  dipole::realtime::ShadowRunner runner(&consumer, &sink, 100, &presence);

  const auto error = runner.RunOnce({}, dipole::realtime::PresenceProjectionPolicy{
                                            .now_unix_ms = 10'000, .ttl_ms = 1'000});
  Check(!error, "invalid Presence is isolated after evidence");
  Check(sink.entries.size() == 1 && sink.entries[0].error_code == "invalid_presence" &&
            sink.entries[0].outcome == dipole::realtime::ShadowOutcome::kRejected,
        "invalid Presence uses fixed low-sensitive category");
  Check(consumer.commit_attempts == std::vector<std::int64_t>{99},
        "invalid Presence commits after evidence");
}

void TestEvidenceFailureKeepsOffsetUncommitted() {
  FakeConsumer consumer;
  consumer.results.push_back(PolledRecord());
  FakeEvidenceSink sink;
  sink.append_error = "disk full";
  dipole::realtime::ShadowRunner runner(&consumer, &sink, 100);

  const auto error = runner.RunOnce({});
  Check(error.has_value() && error->find("evidence") != std::string::npos,
        "evidence failure reaches caller");
  Check(consumer.commit_attempts.empty(), "evidence failure does not commit");
  Check(!runner.Ready(), "processing failure removes readiness");
}

void TestCommitFailureReachesCaller() {
  FakeConsumer consumer;
  consumer.results.push_back(PolledRecord());
  consumer.commit_error = "broker unavailable";
  FakeEvidenceSink sink;
  dipole::realtime::ShadowRunner runner(&consumer, &sink, 100);

  const auto error = runner.RunOnce({});
  Check(error.has_value() && error->find("commit") != std::string::npos,
        "commit failure reaches caller");
  Check(sink.entries.size() == 1 && consumer.commit_attempts.size() == 1,
        "commit follows evidence exactly once");
  Check(runner.Stats().commit_errors == 1, "commit error stats advance");
}

void TestTimeoutAndPollError() {
  FakeConsumer consumer;
  consumer.results.push_back(PollTimeout());
  consumer.results.push_back(PollError("transport down"));
  FakeEvidenceSink sink;
  dipole::realtime::ShadowRunner runner(&consumer, &sink, 100);

  Check(!runner.RunOnce({}), "timeout is an idle success");
  Check(consumer.commit_attempts.empty() && sink.entries.empty(),
        "timeout has no side effects");
  const auto error = runner.RunOnce({});
  Check(error.has_value() && error->find("poll") != std::string::npos,
        "poll error reaches caller");
  Check(runner.Stats().poll_errors == 1 && !runner.Ready(), "poll error removes readiness");
}

void TestInvalidConstruction() {
  FakeConsumer consumer;
  FakeEvidenceSink sink;
  dipole::realtime::ShadowRunner missing_consumer(nullptr, &sink, 100);
  dipole::realtime::ShadowRunner missing_sink(&consumer, nullptr, 100);
  dipole::realtime::ShadowRunner invalid_timeout(&consumer, &sink, 0);
  Check(missing_consumer.RunOnce({}).has_value(), "runner requires consumer");
  Check(missing_sink.RunOnce({}).has_value(), "runner requires evidence sink");
  Check(invalid_timeout.RunOnce({}).has_value(), "runner requires positive timeout");
}

}  // namespace

int main() {
  try {
    TestProjectedRecordCommitsAfterEvidence();
    TestPoisonRecordWritesEvidenceAndCommits();
    TestPresenceProjectionAddsBoundedEvidence();
    TestPresenceReadFailureKeepsOffsetUncommitted();
    TestNodeTransportPrecedesEvidenceAndCommit();
    TestNodeTransportFailureWritesDeferredEvidenceWithoutCommit();
    TestNodeTransportFailureRetriesPendingRecordWithoutPoll();
    TestInvalidPresenceWritesEvidenceAndCommits();
    TestEvidenceFailureKeepsOffsetUncommitted();
    TestCommitFailureReachesCaller();
    TestTimeoutAndPollError();
    TestInvalidConstruction();
  } catch (const std::exception& error) {
    std::cerr << "FAIL: unexpected exception: " << error.what() << '\n';
    ++failures;
  }
  return failures == 0 ? EXIT_SUCCESS : EXIT_FAILURE;
}
