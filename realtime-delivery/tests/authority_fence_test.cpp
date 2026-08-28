#include <cstdint>
#include <fstream>
#include <iostream>
#include <string>

#include <nlohmann/json.hpp>

#include "authority_fence.hpp"

namespace {

class FakeStringReader final : public dipole::realtime::StringValueReader {
 public:
  dipole::realtime::ValidationError ReadString(const std::string& requested_key,
                                               std::string* value) override {
    key = requested_key;
    if (error) return error;
    *value = payload;
    return std::nullopt;
  }

  std::string key;
  std::string payload;
  dipole::realtime::ValidationError error;
};

class FakeStringWriter final : public dipole::realtime::StringValueWriter {
 public:
  dipole::realtime::ValidationError WriteStringWithTTL(const std::string& requested_key,
                                                       const std::string& requested_value,
                                                       std::int64_t requested_ttl_ms) override {
    ++writes;
    key = requested_key;
    value = requested_value;
    ttl_ms = requested_ttl_ms;
    return error;
  }

  int writes = 0;
  std::string key;
  std::string value;
  std::int64_t ttl_ms = 0;
  dipole::realtime::ValidationError error;
};

}  // namespace

int RunTests() {
  std::ifstream input(DIPOLE_FENCE_TESTDATA_FILE);
  const auto vectors = nlohmann::json::parse(input);
  for (const auto& test : vectors.at("cases")) {
    dipole::realtime::FenceAuthority authority;
    if (dipole::realtime::ParseFenceAuthority(test.at("expected_authority").get<std::string>(),
                                               &authority)) {
      std::cerr << "invalid expected authority in " << test.at("name") << '\n';
      return 1;
    }
    const dipole::realtime::AuthorityFenceExpectation expectation{
        .authority = authority,
        .epoch = test.at("expected_epoch").get<std::uint64_t>(),
        .now_unix_ms = test.at("now_unix_ms").get<std::int64_t>(),
    };
    dipole::realtime::AuthorityFenceInspection inspection;
    const bool authorized = !dipole::realtime::InspectAuthorityFence(
        test.at("payload").get<std::string>(), expectation, &inspection);
    if (authorized != test.at("authorized").get<bool>()) {
      std::cerr << "authority fence result mismatch for " << test.at("name") << '\n';
      return 1;
    }
    if (inspection.reason_code != test.at("expected_reason").get<std::string>()) {
      std::cerr << "authority fence reason mismatch for " << test.at("name") << '\n';
      return 1;
    }
  }
  FakeStringReader source;
  source.payload = vectors.at("cases").at(1).at("payload").get<std::string>();
  std::int64_t now = 1'787'875'200'000;
  dipole::realtime::RedisAuthorityFenceReader reader(
      &source, "dipole:test:fence",
      {.authority = dipole::realtime::FenceAuthority::kCpp, .epoch = 18}, [&now]() { return now; });
  if (reader.Assert() || source.key != "dipole:test:fence") {
    std::cerr << "Redis authority fence reader rejected valid payload\n";
    return 1;
  }
  now = 1'787'875'260'000;
  if (!reader.Assert()) {
    std::cerr << "Redis authority fence reader accepted expired payload\n";
    return 1;
  }

  now = 1'787'875'200'000;
  FakeStringWriter writer;
  dipole::realtime::RedisObservedAuthorityFenceReader observed(
      &reader, &writer,
      {.key_prefix = "dipole:test:fence:observation:",
       .component = "realtime-delivery",
       .observer_id = "cpp-a",
       .ttl_ms = 15'000,
       .interval_ms = 5'000},
      [&now]() { return now; });
  if (observed.Assert() || writer.writes != 1 || writer.ttl_ms != 15'000 ||
      writer.key != "dipole:test:fence:observation:realtime-delivery:cpp-a") {
    std::cerr << "observed fence did not publish the initial TTL record\n";
    return 1;
  }
  const auto observation = nlohmann::json::parse(writer.value);
  if (observation.at("schema_version") != "dipole.realtime.delivery-fence-observation.v1" ||
      observation.at("expected_authority") != "cpp" || observation.at("expected_epoch") != 18 ||
      observation.at("observed_authority") != "cpp" || observation.at("observed_epoch") != 18 ||
      observation.at("observed_phase") != "active" || observation.at("status") != "authorized" ||
      observation.at("reason_code") != "authorized" ||
      observation.at("observed_lease_sha256") != dipole::realtime::Sha256Hex(source.payload)) {
    std::cerr << "observed fence payload drifted\n";
    return 1;
  }
  now += 4'999;
  if (observed.Heartbeat() || writer.writes != 1) {
    std::cerr << "observed fence heartbeat ignored its interval\n";
    return 1;
  }
  now += 1;
  if (observed.Heartbeat() || writer.writes != 2) {
    std::cerr << "observed fence heartbeat did not refresh at interval\n";
    return 1;
  }
  source.payload = vectors.at("cases").at(2).at("payload").get<std::string>();
  now += 5'000;
  if (!observed.Assert() || writer.writes != 3 ||
      nlohmann::json::parse(writer.value).at("reason_code") != "frozen") {
    std::cerr << "observed fence did not publish denial\n";
    return 1;
  }
  writer.error = "Redis observation write failed";
  now += 5'000;
  if (!observed.Assert()) {
    std::cerr << "observed fence accepted a failed observation write\n";
    return 1;
  }
  dipole::realtime::RedisObservedAuthorityFenceReader invalid_observer(
      &reader, &writer,
      {.key_prefix = "prefix:", .component = "realtime-delivery", .observer_id = "bad observer",
       .ttl_ms = 15'000, .interval_ms = 5'000},
      [&now]() { return now; });
  if (!invalid_observer.Assert()) {
    std::cerr << "observed fence accepted invalid observer identity\n";
    return 1;
  }
  return 0;
}

int main() {
  try {
    return RunTests();
  } catch (const std::exception& error) {
    std::cerr << "unexpected authority fence test exception: " << error.what() << '\n';
    return 1;
  }
}
