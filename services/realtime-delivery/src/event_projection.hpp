#ifndef DIPOLE_REALTIME_DELIVERY_EVENT_PROJECTION_HPP_
#define DIPOLE_REALTIME_DELIVERY_EVENT_PROJECTION_HPP_

#include <cstdint>
#include <string>

#include "contract_validator.hpp"

namespace dipole::realtime {

// KafkaRecord keeps broker mechanics outside the deterministic projection.
struct KafkaRecord {
  std::string topic;
  std::int32_t partition = -1;
  std::int64_t offset = -1;
  std::string value;
};

// Hot-group classification remains an explicit input until the Redis adapter
// is introduced. This keeps event projection deterministic and testable.
struct ProjectionPolicy {
  bool timeline_notify_shadow = false;
  bool hot_group = false;
  int recent_message_count = 0;
};

ValidationError ProjectMessageEvent(const KafkaRecord& record, const ProjectionPolicy& policy,
                                    delivery::v1::DeliveryEnvelope* output);

}  // namespace dipole::realtime

#endif  // DIPOLE_REALTIME_DELIVERY_EVENT_PROJECTION_HPP_
