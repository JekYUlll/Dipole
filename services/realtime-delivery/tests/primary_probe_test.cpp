#include <cstdlib>
#include <filesystem>
#include <fstream>
#include <iostream>
#include <string>
#include <string_view>

#include "primary_probe.hpp"

namespace {

int failures = 0;

void Check(bool condition, const std::string& message) {
  if (!condition) {
    std::cerr << "FAIL: " << message << '\n';
    ++failures;
  }
}

std::filesystem::path WriteFixture(const char* name, std::string_view content) {
  const auto path = std::filesystem::temp_directory_path() / name;
  std::ofstream output(path, std::ios::binary | std::ios::trunc);
  output << content;
  return path;
}

void TestBatchJsonParsing() {
  const auto valid = WriteFixture("dipole-primary-probe-valid.json", R"({
    "contractVersion":"v1",
    "batchId":"NB1",
    "targetNodeId":"gateway-1",
    "sourceEventId":"E1",
    "requestId":"R1",
    "traceId":"T1",
    "createdAt":"2026-08-28T00:00:00Z",
    "items":[{
      "deliveryId":"D1",
      "recipientUserId":"U1",
      "connectionIds":["C1"],
      "eventType":"chat.message",
      "payloadJson":"eyJtZXNzYWdlX2lkIjoiTTEifQ==",
      "orderingKey":"direct:U1:U2",
      "mode":"DELIVERY_MODE_FULL_EVENT"
    }]
  })");
  dipole::delivery::v1::NodeDeliveryBatch batch;
  Check(!dipole::realtime::LoadPrimaryProbeBatch(valid.string(), &batch) &&
            batch.batch_id() == "NB1" && batch.items_size() == 1 &&
            batch.items(0).payload_json() == R"({"message_id":"M1"})",
        "primary probe parses and validates canonical batch JSON");

  const auto unknown = WriteFixture("dipole-primary-probe-unknown.json", R"({"unknown":true})");
  Check(dipole::realtime::LoadPrimaryProbeBatch(unknown.string(), &batch).has_value(),
        "primary probe rejects unknown JSON fields");
  const auto empty = WriteFixture("dipole-primary-probe-empty.json", "");
  Check(dipole::realtime::LoadPrimaryProbeBatch(empty.string(), &batch).has_value(),
        "primary probe rejects empty input");

  std::filesystem::remove(valid);
  std::filesystem::remove(unknown);
  std::filesystem::remove(empty);
}

}  // namespace

int main() {
  TestBatchJsonParsing();
  return failures == 0 ? EXIT_SUCCESS : EXIT_FAILURE;
}
