#include "authority_fence.hpp"

#include <array>
#include <cstdint>
#include <limits>
#include <string>
#include <unordered_set>
#include <utility>

#include <nlohmann/json.hpp>
#include <openssl/evp.h>

namespace dipole::realtime {
namespace {

constexpr const char* kFenceSchema = "dipole.realtime.delivery-fence.v1";
constexpr const char* kObservationSchema = "dipole.realtime.delivery-fence-observation.v1";

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

bool IsASCIIAlphanumeric(unsigned char character) {
  return (character >= 'A' && character <= 'Z') || (character >= 'a' && character <= 'z') ||
         (character >= '0' && character <= '9');
}

bool ValidObservationID(const std::string& value) {
  if (value.empty() || value.size() > 64 || !IsASCIIAlphanumeric(value.front())) {
    return false;
  }
  for (const unsigned char character : value) {
    if (!IsASCIIAlphanumeric(character) && character != '.' && character != '_' && character != ':' &&
        character != '@' && character != '-') {
      return false;
    }
  }
  return true;
}

}  // namespace

std::string Sha256Hex(const std::string& value) {
  std::array<unsigned char, EVP_MAX_MD_SIZE> digest{};
  unsigned int digest_size = 0;
  if (EVP_Digest(value.data(), value.size(), digest.data(), &digest_size, EVP_sha256(), nullptr) != 1) {
    return "";
  }
  constexpr char kHex[] = "0123456789abcdef";
  const auto size = static_cast<std::size_t>(digest_size);
  std::string output(size * 2, '0');
  for (std::size_t index = 0; index < size; ++index) {
    output[index * 2] = kHex[digest[index] >> 4U];
    output[index * 2 + 1] = kHex[digest[index] & 0x0FU];
  }
  return output;
}

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
  AuthorityFenceInspection inspection;
  return InspectAuthorityFence(payload, expectation, &inspection);
}

ValidationError InspectAuthorityFence(const std::string& payload,
                                      const AuthorityFenceExpectation& expectation,
                                      AuthorityFenceInspection* inspection) {
  if (inspection == nullptr) return "delivery authority fence inspection destination is required";
  *inspection = {};
  inspection->observed_lease_sha256 = Sha256Hex(payload);
  if (expectation.epoch == 0 || expectation.now_unix_ms <= 0) {
    inspection->reason_code = "invalid_record";
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
    inspection->reason_code = "invalid_record";
    return "decode delivery authority fence";
  }
  if (duplicate || !record.is_object()) {
    inspection->reason_code = "invalid_record";
    return "delivery authority fence shape is invalid";
  }
  static constexpr std::array<const char*, 5> required{
      "schema_version", "epoch", "authority", "phase", "lease_until_unix_ms"};
  if (record.size() != required.size()) {
    inspection->reason_code = "invalid_record";
    return "delivery authority fence fields are invalid";
  }
  for (const char* field : required) {
    if (!record.contains(field)) {
      inspection->reason_code = "invalid_record";
      return "delivery authority fence fields are invalid";
    }
  }
  if (!record["schema_version"].is_string() || record["schema_version"] != kFenceSchema ||
      !record["epoch"].is_number_unsigned() || !record["authority"].is_string() ||
      !record["phase"].is_string() || !record["lease_until_unix_ms"].is_number_integer()) {
    inspection->reason_code = "invalid_record";
    return "delivery authority fence values are invalid";
  }
  FenceAuthority granted;
  if (auto error = ParseFenceAuthority(record["authority"].get<std::string>(), &granted)) {
    inspection->reason_code = "invalid_record";
    return error;
  }
  const auto epoch = record["epoch"].get<std::uint64_t>();
  const auto lease_until = record["lease_until_unix_ms"].get<std::int64_t>();
  const auto phase = record["phase"].get<std::string>();
  if (epoch == 0 || lease_until <= 0 || (phase != "active" && phase != "frozen")) {
    inspection->reason_code = "invalid_record";
    return "delivery authority fence values are invalid";
  }
  inspection->observed_authority = AuthorityName(granted);
  inspection->observed_epoch = epoch;
  inspection->observed_phase = phase;
  inspection->observed_lease_until_unix_ms = lease_until;
  if (epoch != expectation.epoch) {
    inspection->reason_code = "epoch_mismatch";
    return "delivery authority fence epoch mismatch";
  }
  if (inspection->observed_phase != "active") {
    inspection->reason_code = "frozen";
    return "delivery authority fence is frozen";
  }
  if (granted != expectation.authority) {
    inspection->reason_code = "authority_mismatch";
    return "delivery authority fence authority mismatch";
  }
  if (lease_until <= expectation.now_unix_ms) {
    inspection->reason_code = "expired";
    return "delivery authority fence lease expired";
  }
  if (inspection->observed_authority.empty()) {
    inspection->reason_code = "invalid_record";
    return "delivery authority fence authority is invalid";
  }
  inspection->reason_code = "authorized";
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
  AuthorityFenceInspection inspection;
  return Inspect(&inspection);
}

ValidationError RedisAuthorityFenceReader::Inspect(AuthorityFenceInspection* inspection) {
  if (inspection == nullptr) return "delivery authority fence inspection destination is required";
  *inspection = {};
  if (source_ == nullptr || key_.empty() || expectation_.epoch == 0 || !now_unix_ms_) {
    inspection->reason_code = "invalid_record";
    return "delivery authority fence reader configuration is invalid";
  }
  std::string payload;
  if (auto error = source_->ReadString(key_, &payload)) {
    inspection->reason_code = *error == "Redis string value is missing" ? "missing" : "redis_unavailable";
    return error;
  }
  auto current = expectation_;
  current.now_unix_ms = now_unix_ms_();
  return InspectAuthorityFence(payload, current, inspection);
}

ValidationError ValidateFenceObservationConfig(const FenceObservationConfig& config) {
  if (config.key_prefix.empty() || !ValidObservationID(config.component) ||
      !ValidObservationID(config.observer_id)) {
    return "delivery authority observation identity is invalid";
  }
  if (config.ttl_ms < 5'000 || config.ttl_ms > 60'000 || config.interval_ms < 1'000 ||
      config.interval_ms > config.ttl_ms / 2) {
    return "delivery authority observation timing is invalid";
  }
  return std::nullopt;
}

RedisObservedAuthorityFenceReader::RedisObservedAuthorityFenceReader(
    RedisAuthorityFenceReader* reader, StringValueWriter* writer, FenceObservationConfig config,
    std::function<std::int64_t()> now_unix_ms)
    : reader_(reader), writer_(writer), config_(std::move(config)), now_unix_ms_(std::move(now_unix_ms)) {}

ValidationError RedisObservedAuthorityFenceReader::Assert() {
  if (reader_ == nullptr || writer_ == nullptr || !now_unix_ms_) {
    return "delivery authority observation configuration is invalid";
  }
  if (auto error = ValidateFenceObservationConfig(config_)) return error;
  return CheckAndPublish(now_unix_ms_());
}

ValidationError RedisObservedAuthorityFenceReader::Heartbeat() {
  if (reader_ == nullptr || writer_ == nullptr || !now_unix_ms_) {
    return "delivery authority observation configuration is invalid";
  }
  if (auto error = ValidateFenceObservationConfig(config_)) return error;
  const auto now = now_unix_ms_();
  if (now < next_publish_unix_ms_) return std::nullopt;
  return CheckAndPublish(now);
}

ValidationError RedisObservedAuthorityFenceReader::CheckAndPublish(std::int64_t now_unix_ms) {
  if (now_unix_ms <= 0 || now_unix_ms > std::numeric_limits<std::int64_t>::max() - config_.ttl_ms) {
    return "delivery authority observation clock is invalid";
  }
  AuthorityFenceInspection inspection;
  auto fence_error = reader_->Inspect(&inspection);
  if (now_unix_ms >= next_publish_unix_ms_) {
    const nlohmann::json observation{
        {"schema_version", kObservationSchema},
        {"observer_id", config_.observer_id},
        {"component", config_.component},
        {"expected_authority", AuthorityName(reader_->Expectation().authority)},
        {"expected_epoch", reader_->Expectation().epoch},
        {"observed_authority", inspection.observed_authority},
        {"observed_epoch", inspection.observed_epoch},
        {"observed_phase", inspection.observed_phase},
        {"observed_lease_until_unix_ms", inspection.observed_lease_until_unix_ms},
        {"observed_lease_sha256", inspection.observed_lease_sha256},
        {"status", fence_error ? "denied" : "authorized"},
        {"reason_code", inspection.reason_code},
        {"observed_at_unix_ms", now_unix_ms},
        {"expires_at_unix_ms", now_unix_ms + config_.ttl_ms},
    };
    const auto key = config_.key_prefix + config_.component + ":" + config_.observer_id;
    if (auto error = writer_->WriteStringWithTTL(key, observation.dump(), config_.ttl_ms)) {
      return "delivery authority observation write failed";
    }
    next_publish_unix_ms_ = now_unix_ms + config_.interval_ms;
  }
  return fence_error;
}

}  // namespace dipole::realtime
