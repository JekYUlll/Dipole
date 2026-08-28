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
    const bool authorized = !dipole::realtime::ValidateAuthorityFence(
        test.at("payload").get<std::string>(), expectation);
    if (authorized != test.at("authorized").get<bool>()) {
      std::cerr << "authority fence result mismatch for " << test.at("name") << '\n';
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
