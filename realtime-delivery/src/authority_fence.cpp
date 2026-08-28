#include "authority_fence.hpp"

#include <array>
#include <cstdint>
#include <string>
#include <unordered_set>
#include <utility>

#include <nlohmann/json.hpp>

namespace dipole::realtime {
namespace {

constexpr const char* kFenceSchema = "dipole.realtime.delivery-fence.v1";

std::string AuthorityName(FenceAuthority authority) {
  switch (authority) {
    case FenceAuthority::kGo:
      return "go";
    case FenceAuthority::kShadow:
      return "shadow";
    case FenceAuthority::kCpp:
      return "cpp";
  }
  return "";
}

}  // namespace

ValidationError ParseFenceAuthority(const std::string& value, FenceAuthority* authority) {
  if (authority == nullptr) return "fence authority destination is required";
  if (value == "go") {
    *authority = FenceAuthority::kGo;
    return std::nullopt;
  }
  if (value == "shadow") {
    *authority = FenceAuthority::kShadow;
    return std::nullopt;
  }
  if (value == "cpp") {
    *authority = FenceAuthority::kCpp;
    return std::nullopt;
  }
  return "fence authority is unsupported";
}

ValidationError ValidateAuthorityFence(const std::string& payload,
                                       const AuthorityFenceExpectation& expectation) {
  if (expectation.epoch == 0 || expectation.now_unix_ms <= 0) {
    return "fence expectation is invalid";
  }
  bool duplicate = false;
  std::unordered_set<std::string> fields;
  const auto callback = [&duplicate, &fields](int, nlohmann::json::parse_event_t event,
                                               nlohmann::json& parsed) {
    if (event != nlohmann::json::parse_event_t::key) return true;
    const auto inserted = fields.insert(parsed.get<std::string>()).second;
    duplicate = duplicate || !inserted;
    return inserted;
  };

  nlohmann::json record;
  try {
    record = nlohmann::json::parse(payload, callback, true, false);
  } catch (const nlohmann::json::exception&) {
    return "decode delivery authority fence";
  }
  if (duplicate || !record.is_object()) return "delivery authority fence shape is invalid";
  static constexpr std::array<const char*, 5> required{
      "schema_version", "epoch", "authority", "phase", "lease_until_unix_ms"};
  if (record.size() != required.size()) return "delivery authority fence fields are invalid";
  for (const char* field : required) {
    if (!record.contains(field)) return "delivery authority fence fields are invalid";
  }
  if (!record["schema_version"].is_string() || record["schema_version"] != kFenceSchema ||
      !record["epoch"].is_number_unsigned() || !record["authority"].is_string() ||
      !record["phase"].is_string() || !record["lease_until_unix_ms"].is_number_integer()) {
    return "delivery authority fence values are invalid";
  }
  FenceAuthority granted;
  if (auto error = ParseFenceAuthority(record["authority"].get<std::string>(), &granted)) return error;
  const auto epoch = record["epoch"].get<std::uint64_t>();
  const auto lease_until = record["lease_until_unix_ms"].get<std::int64_t>();
  if (epoch == 0 || epoch != expectation.epoch) return "delivery authority fence epoch mismatch";
  if (record["phase"].get<std::string>() != "active") return "delivery authority fence is frozen";
  if (granted != expectation.authority) return "delivery authority fence authority mismatch";
  if (lease_until <= expectation.now_unix_ms) return "delivery authority fence lease expired";
  if (AuthorityName(granted).empty()) return "delivery authority fence authority is invalid";
  return std::nullopt;
}

RedisAuthorityFenceReader::RedisAuthorityFenceReader(
    StringValueReader* source, std::string key, AuthorityFenceExpectation expectation,
    std::function<std::int64_t()> now_unix_ms)
    : source_(source),
      key_(std::move(key)),
      expectation_(expectation),
      now_unix_ms_(std::move(now_unix_ms)) {}

ValidationError RedisAuthorityFenceReader::Assert() {
  if (source_ == nullptr || key_.empty() || expectation_.epoch == 0 || !now_unix_ms_) {
    return "delivery authority fence reader configuration is invalid";
  }
  std::string payload;
  if (auto error = source_->ReadString(key_, &payload)) return error;
  auto current = expectation_;
  current.now_unix_ms = now_unix_ms_();
  return ValidateAuthorityFence(payload, current);
}

}  // namespace dipole::realtime
