#include "hiredis_presence_reader.hpp"

#include <iostream>
#include <cstdlib>
#include <chrono>
#include <string>
#include <thread>

namespace {

int Expect(bool condition, const std::string& message) {
  if (condition) return 0;
  std::cerr << message << '\n';
  return 1;
}

int TestEndpointParsing() {
  dipole::realtime::RedisEndpoint endpoint;
  int failures = 0;
  failures += Expect(!dipole::realtime::ParseRedisEndpoint("redis:6379", &endpoint).has_value() &&
                         endpoint.host == "redis" && endpoint.port == 6379,
                     "expected hostname endpoint");
  failures += Expect(!dipole::realtime::ParseRedisEndpoint("[::1]:26379", &endpoint).has_value() &&
                         endpoint.host == "::1" && endpoint.port == 26379,
                     "expected bracketed IPv6 endpoint");
  failures += Expect(dipole::realtime::ParseRedisEndpoint("redis", &endpoint).has_value(),
                     "expected missing port rejection");
  failures += Expect(dipole::realtime::ParseRedisEndpoint("redis:0", &endpoint).has_value(),
                     "expected invalid port rejection");
  return failures;
}

int TestConfigValidation() {
  using dipole::realtime::HiredisPresenceConfig;
  int failures = 0;
  HiredisPresenceConfig direct;
  direct.direct = {.host = "redis", .port = 6379};
  direct.timeout_ms = 500;
  failures += Expect(!dipole::realtime::ValidateHiredisPresenceConfig(direct).has_value(),
                     "expected direct config");
  HiredisPresenceConfig sentinel;
  sentinel.sentinels = {{.host = "s1", .port = 26379}};
  sentinel.sentinel_master_name = "dipole-master";
  sentinel.timeout_ms = 500;
  failures += Expect(!dipole::realtime::ValidateHiredisPresenceConfig(sentinel).has_value(),
                     "expected sentinel config");
  sentinel.sentinel_master_name.clear();
  failures += Expect(dipole::realtime::ValidateHiredisPresenceConfig(sentinel).has_value(),
                     "expected sentinel master requirement");
  direct.db = 16;
  failures += Expect(dipole::realtime::ValidateHiredisPresenceConfig(direct).has_value(),
                     "expected bounded Redis db");
  return failures;
}

int TestRealRedisWhenConfigured() {
  const char* raw_endpoint = std::getenv("DIPOLE_TEST_REDIS_ENDPOINT");
  const char* raw_sentinel = std::getenv("DIPOLE_TEST_REDIS_SENTINEL");
  if ((raw_endpoint == nullptr || *raw_endpoint == '\0') &&
      (raw_sentinel == nullptr || *raw_sentinel == '\0')) {
    return 0;
  }
  dipole::realtime::RedisEndpoint endpoint;
  const char* selected = raw_endpoint != nullptr && *raw_endpoint != '\0' ? raw_endpoint : raw_sentinel;
  if (auto error = dipole::realtime::ParseRedisEndpoint(selected, &endpoint)) {
    std::cerr << *error << '\n';
    return 1;
  }
  dipole::realtime::HiredisPresenceConfig config;
  if (raw_endpoint != nullptr && *raw_endpoint != '\0') {
    config.direct = endpoint;
  } else {
    config.sentinels = {endpoint};
    const char* master = std::getenv("DIPOLE_TEST_REDIS_MASTER_NAME");
    config.sentinel_master_name = master == nullptr ? "" : master;
  }
  dipole::realtime::HiredisPresenceReader reader(config);
  int iterations = 1;
  if (const char* raw_iterations = std::getenv("DIPOLE_TEST_REDIS_ITERATIONS"); raw_iterations != nullptr) {
    iterations = std::stoi(raw_iterations);
  }
  int successes = 0;
  int errors = 0;
  int failures = 0;
  for (int iteration = 0; iteration < iterations; ++iteration) {
    dipole::realtime::PresenceReadResult result;
    if (auto error = reader.ReadUsers({"U-C2-PRESENCE-TEST", "U-C2-OFFLINE"}, &result)) {
      ++errors;
    } else {
      ++successes;
      failures += Expect(result.by_user.at("U-C2-PRESENCE-TEST").size() == 2,
                         "expected two real Redis Presence connections");
      failures += Expect(result.by_user.at("U-C2-OFFLINE").empty(),
                         "expected empty offline Presence hash");
      failures += Expect(result.parse_stats.parsed_records == 2 &&
                             result.parse_stats.malformed_records == 0,
                         "expected real Redis parse counters");
    }
    if (iterations > 1) std::this_thread::sleep_for(std::chrono::milliseconds(250));
  }
  failures += Expect(successes >= (iterations > 1 ? 2 : 1), "expected successful Redis reads");
  if (std::getenv("DIPOLE_TEST_REDIS_EXPECT_ERRORS") != nullptr) {
    failures += Expect(errors > 0, "expected a bounded Redis failover error window");
  } else {
    failures += Expect(errors == 0, "expected no Redis read errors");
  }
  if (iterations > 1) std::cout << "successes=" << successes << " errors=" << errors << '\n';
  return failures;
}

}  // namespace

int main() {
  return TestEndpointParsing() + TestConfigValidation() + TestRealRedisWhenConfigured() == 0 ? 0 : 1;
}
