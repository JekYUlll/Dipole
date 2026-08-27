#include "hiredis_presence_reader.hpp"

#include <iostream>
#include <cstdlib>
#include <string>

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
  if (raw_endpoint == nullptr || *raw_endpoint == '\0') return 0;
  dipole::realtime::RedisEndpoint endpoint;
  if (auto error = dipole::realtime::ParseRedisEndpoint(raw_endpoint, &endpoint)) {
    std::cerr << *error << '\n';
    return 1;
  }
  dipole::realtime::HiredisPresenceConfig config;
  config.direct = endpoint;
  dipole::realtime::HiredisPresenceReader reader(config);
  dipole::realtime::PresenceReadResult result;
  if (auto error = reader.ReadUsers({"U-C2-PRESENCE-TEST", "U-C2-OFFLINE"}, &result)) {
    std::cerr << *error << '\n';
    return 1;
  }
  int failures = 0;
  failures += Expect(result.by_user.at("U-C2-PRESENCE-TEST").size() == 2,
                     "expected two real Redis Presence connections");
  failures += Expect(result.by_user.at("U-C2-OFFLINE").empty(), "expected empty offline Presence hash");
  failures += Expect(result.parse_stats.parsed_records == 2 && result.parse_stats.malformed_records == 0,
                     "expected real Redis parse counters");
  return failures;
}

}  // namespace

int main() {
  return TestEndpointParsing() + TestConfigValidation() + TestRealRedisWhenConfigured() == 0 ? 0 : 1;
}
