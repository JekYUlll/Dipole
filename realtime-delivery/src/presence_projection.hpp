#ifndef DIPOLE_REALTIME_DELIVERY_PRESENCE_PROJECTION_HPP_
#define DIPOLE_REALTIME_DELIVERY_PRESENCE_PROJECTION_HPP_

#include <cstdint>
#include <string>
#include <unordered_map>
#include <utility>
#include <vector>

#include "contract_validator.hpp"

namespace dipole::realtime {

struct PresenceConnection {
  std::string connection_id;
  std::string user_id;
  std::string node_id;
  std::int64_t last_seen_unix_ms = 0;
};

using PresenceByUser = std::unordered_map<std::string, std::vector<PresenceConnection>>;

struct PresenceProjectionPolicy {
  std::int64_t now_unix_ms = 0;
  std::int64_t ttl_ms = 0;
};

struct PresenceProjectionStats {
  std::size_t observed_connections = 0;
  std::size_t eligible_connections = 0;
  std::size_t stale_connections = 0;
  std::size_t offline_items = 0;
};

struct PresenceHashParseStats {
  std::size_t observed_records = 0;
  std::size_t parsed_records = 0;
  std::size_t malformed_records = 0;
};

ValidationError ParsePresenceHash(
    const std::string& user_id,
    const std::vector<std::pair<std::string, std::string>>& fields,
    std::vector<PresenceConnection>* connections, PresenceHashParseStats* stats);

ValidationError ProjectPresence(const delivery::v1::DeliveryEnvelope& envelope,
                                const PresenceByUser& presence,
                                const PresenceProjectionPolicy& policy,
                                std::vector<delivery::v1::NodeDeliveryBatch>* batches,
                                PresenceProjectionStats* stats);

}  // namespace dipole::realtime

#endif  // DIPOLE_REALTIME_DELIVERY_PRESENCE_PROJECTION_HPP_
