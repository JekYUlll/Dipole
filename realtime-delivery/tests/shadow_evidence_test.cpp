#include <cstdlib>
#include <iostream>
#include <sstream>
#include <string>

#include <nlohmann/json.hpp>

#include "shadow_evidence.hpp"

namespace {

int failures = 0;

void Check(bool condition, const std::string& message) {
  if (!condition) {
    std::cerr << "FAIL: " << message << '\n';
    ++failures;
  }
}

void TestProjectedEvidence() {
  std::ostringstream output;
  dipole::realtime::JsonLineEvidenceSink sink(&output);
  const dipole::realtime::ShadowEvidence evidence{
      .topic = "dipole.message.direct.created",
      .partition = 2,
      .offset = 41,
      .source_event_id = "E1",
      .batch_id = "shadow:E1:2:41",
      .message_type = 0,
      .item_count = 2,
      .outcome = dipole::realtime::ShadowOutcome::kProjected,
      .error_code = "",
  };
  Check(!sink.Append(evidence), "write projected evidence");
  const auto decoded = nlohmann::json::parse(output.str());
  Check(decoded == nlohmann::json({
                       {"schema_version", "dipole.realtime.shadow-evidence.v3"},
                       {"topic", "dipole.message.direct.created"},
                       {"partition", 2},
                       {"offset", 41},
                       {"outcome", "projected"},
                       {"source_event_id", "E1"},
                       {"batch_id", "shadow:E1:2:41"},
                       {"message_type", 0},
                       {"item_count", 2},
                       {"error_code", ""},
                       {"node_batch_count", 0},
                       {"presence_observed", 0},
                       {"presence_eligible", 0},
                       {"presence_stale", 0},
                       {"presence_malformed", 0},
                       {"offline_item_count", 0},
                       {"transport_requested", 0},
                       {"transport_observed", 0},
                       {"transport_duplicate", 0},
                       {"transport_rejected", 0},
                       {"transport_backpressured", 0},
                   }),
        "projected evidence schema is stable");
  Check(output.str().find("body must not enter evidence") == std::string::npos,
        "evidence excludes message payload fields");
}

void TestRejectedEvidenceAndFailures() {
  std::ostringstream output;
  dipole::realtime::JsonLineEvidenceSink sink(&output);
  dipole::realtime::ShadowEvidence evidence{
      .topic = "dipole.message.group.created",
      .partition = 0,
      .offset = 9,
      .source_event_id = "",
      .batch_id = "",
      .message_type = -1,
      .item_count = 0,
      .outcome = dipole::realtime::ShadowOutcome::kRejected,
      .error_code = "invalid_event",
  };
  Check(!sink.Append(evidence), "write rejected evidence");
  Check(nlohmann::json::parse(output.str()).at("outcome") == "rejected",
        "rejected outcome is explicit");
  Check(nlohmann::json::parse(output.str()).at("message_type").is_null(),
        "invalid events do not invent a message type");

  dipole::realtime::JsonLineEvidenceSink missing(nullptr);
  Check(missing.Append(evidence).has_value(), "missing stream is rejected");
  evidence.offset = -1;
  Check(sink.Append(evidence).has_value(), "invalid coordinates are rejected");
}

}  // namespace

int main() {
  try {
    TestProjectedEvidence();
    TestRejectedEvidenceAndFailures();
  } catch (const std::exception& error) {
    std::cerr << "FAIL: unexpected exception: " << error.what() << '\n';
    ++failures;
  }
  return failures == 0 ? EXIT_SUCCESS : EXIT_FAILURE;
}
