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

ValidationError ValidateAuthorityFence(const std::string& payload,
                                       const AuthorityFenceExpectation& expectation);
ValidationError ParseFenceAuthority(const std::string& value, FenceAuthority* authority);

class StringValueReader {
 public:
  virtual ~StringValueReader() = default;
  virtual ValidationError ReadString(const std::string& key, std::string* value) = 0;
};

class AuthorityFenceReader {
 public:
  virtual ~AuthorityFenceReader() = default;
  virtual ValidationError Assert() = 0;
};

class RedisAuthorityFenceReader final : public AuthorityFenceReader {
 public:
  RedisAuthorityFenceReader(StringValueReader* source, std::string key,
                            AuthorityFenceExpectation expectation,
                            std::function<std::int64_t()> now_unix_ms);
  ValidationError Assert() override;

 private:
  StringValueReader* source_;
  std::string key_;
  AuthorityFenceExpectation expectation_;
  std::function<std::int64_t()> now_unix_ms_;
};

}  // namespace dipole::realtime

#endif  // DIPOLE_REALTIME_DELIVERY_AUTHORITY_FENCE_HPP_
