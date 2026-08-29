#ifndef DIPOLE_REALTIME_DELIVERY_PRIMARY_PROBE_HPP_
#define DIPOLE_REALTIME_DELIVERY_PRIMARY_PROBE_HPP_

#include <string>

#include "contract_validator.hpp"
#include "dipole/delivery/v1/delivery.pb.h"

namespace dipole::realtime {

ValidationError LoadPrimaryProbeBatch(const std::string& path,
                                      delivery::v1::NodeDeliveryBatch* batch);
int RunPrimaryProbe(const std::string& batch_path);

}  // namespace dipole::realtime

#endif  // DIPOLE_REALTIME_DELIVERY_PRIMARY_PROBE_HPP_
