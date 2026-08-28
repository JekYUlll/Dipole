#ifndef DIPOLE_REALTIME_DELIVERY_AUTHORITY_FENCE_HPP_
#define DIPOLE_REALTIME_DELIVERY_AUTHORITY_FENCE_HPP_

#include <cstdint>
#include <functional>
#include <string>

#include "presence_projection.hpp"

namespace dipole::realtime {

enum class FenceAuthority : std::uint8_t { kGo, kShadow, kCpp };

struct AuthorityFenceExpectation {
  FenceAuthority authority = FenceAuthority::kGo;
  std::uint64_t epoch = 0;
  std::int64_t now_unix_ms = 0;
};

struct AuthorityFenceInspection {
  std::string observed_authority;
  std::uint64_t observed_epoch = 0;
  std::string observed_phase;
  std::int64_t observed_lease_until_unix_ms = 0;
  std::string observed_lease_sha256;
  std::string reason_code;
};

ValidationError ValidateAuthorityFence(const std::string& payload,
                                       const AuthorityFenceExpectation& expectation);
ValidationError InspectAuthorityFence(const std::string& payload,
                                      const AuthorityFenceExpectation& expectation,
                                      AuthorityFenceInspection* inspection);
ValidationError ParseFenceAuthority(const std::string& value, FenceAuthority* authority);
std::string Sha256Hex(const std::string& value);

class StringValueReader {
 public:
  virtual ~StringValueReader() = default;
  virtual ValidationError ReadString(const std::string& key, std::string* value) = 0;
};

class StringValueWriter {
 public:
  virtual ~StringValueWriter() = default;
  virtual ValidationError WriteStringWithTTL(const std::string& key, const std::string& value,
                                             std::int64_t ttl_ms) = 0;
};

class AuthorityFenceReader {
 public:
  virtual ~AuthorityFenceReader() = default;
  virtual ValidationError Assert() = 0;
  virtual ValidationError Heartbeat() { return Assert(); }
};

class RedisAuthorityFenceReader final : public AuthorityFenceReader {
 public:
  RedisAuthorityFenceReader(StringValueReader* source, std::string key,
                            AuthorityFenceExpectation expectation,
                            std::function<std::int64_t()> now_unix_ms);
  ValidationError Assert() override;
  ValidationError Inspect(AuthorityFenceInspection* inspection);
  [[nodiscard]] const AuthorityFenceExpectation& Expectation() const { return expectation_; }

 private:
  StringValueReader* source_;
  std::string key_;
  AuthorityFenceExpectation expectation_;
  std::function<std::int64_t()> now_unix_ms_;
};

struct FenceObservationConfig {
  std::string key_prefix;
  std::string component;
  std::string observer_id;
  std::int64_t ttl_ms = 15'000;
  std::int64_t interval_ms = 5'000;
};

ValidationError ValidateFenceObservationConfig(const FenceObservationConfig& config);

class RedisObservedAuthorityFenceReader final : public AuthorityFenceReader {
 public:
  RedisObservedAuthorityFenceReader(RedisAuthorityFenceReader* reader, StringValueWriter* writer,
                                    FenceObservationConfig config,
                                    std::function<std::int64_t()> now_unix_ms);
  ValidationError Assert() override;
  ValidationError Heartbeat() override;

 private:
  ValidationError CheckAndPublish(std::int64_t now_unix_ms);

  RedisAuthorityFenceReader* reader_;
  StringValueWriter* writer_;
  FenceObservationConfig config_;
  std::function<std::int64_t()> now_unix_ms_;
  std::int64_t next_publish_unix_ms_ = 0;
};

}  // namespace dipole::realtime

#endif  // DIPOLE_REALTIME_DELIVERY_AUTHORITY_FENCE_HPP_
