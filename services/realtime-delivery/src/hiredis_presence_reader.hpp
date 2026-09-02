#ifndef DIPOLE_REALTIME_DELIVERY_HIREDIS_PRESENCE_READER_HPP_
#define DIPOLE_REALTIME_DELIVERY_HIREDIS_PRESENCE_READER_HPP_

#include <cstdint>
#include <string>
#include <vector>

#include "presence_projection.hpp"
#include "authority_fence.hpp"

struct redisContext;

namespace dipole::realtime {

struct RedisEndpoint {
  std::string host;
  std::uint16_t port = 0;
};

struct HiredisPresenceConfig {
  RedisEndpoint direct;
  std::vector<RedisEndpoint> sentinels;
  std::string sentinel_master_name;
  std::string password;
  std::string sentinel_password;
  int db = 0;
  int timeout_ms = 500;
};

ValidationError ParseRedisEndpoint(const std::string& value, RedisEndpoint* endpoint);
ValidationError ValidateHiredisPresenceConfig(const HiredisPresenceConfig& config);

class HiredisPresenceReader final : public PresenceReader, public StringValueReader, public StringValueWriter {
 public:
  explicit HiredisPresenceReader(HiredisPresenceConfig config);
  ~HiredisPresenceReader();
  HiredisPresenceReader(const HiredisPresenceReader&) = delete;
  HiredisPresenceReader& operator=(const HiredisPresenceReader&) = delete;

  ValidationError ReadUsers(const std::vector<std::string>& user_ids,
                            PresenceReadResult* result) override;
  ValidationError ReadString(const std::string& key, std::string* value) override;
  ValidationError WriteStringWithTTL(const std::string& key, const std::string& value,
                                     std::int64_t ttl_ms) override;

 private:
  ValidationError EnsureConnected();
  void Close();

  HiredisPresenceConfig config_;
  redisContext* context_ = nullptr;
};

}  // namespace dipole::realtime

#endif  // DIPOLE_REALTIME_DELIVERY_HIREDIS_PRESENCE_READER_HPP_
