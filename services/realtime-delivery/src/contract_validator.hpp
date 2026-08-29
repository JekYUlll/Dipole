#ifndef DIPOLE_REALTIME_DELIVERY_CONTRACT_VALIDATOR_HPP_
#define DIPOLE_REALTIME_DELIVERY_CONTRACT_VALIDATOR_HPP_

#include <optional>
#include <string>

#include "dipole/delivery/v1/delivery.pb.h"

namespace dipole::realtime {

inline constexpr char kContractVersion[] = "v1";
inline constexpr int kMaxBatchItems = 4096;

using ValidationError = std::optional<std::string>;

ValidationError ValidateEnvelope(const delivery::v1::DeliveryEnvelope& envelope);
ValidationError ValidateNodeBatch(const delivery::v1::NodeDeliveryBatch& batch);
ValidationError ValidateAck(const delivery::v1::DeliveryAck& ack);
ValidationError ValidateObservation(const delivery::v1::NodeDeliveryObservation& observation);

ValidationError ParseEnvelopeJsonFile(const std::string& path, delivery::v1::DeliveryEnvelope* envelope);
ValidationError ParseNodeBatchJsonFile(const std::string& path, delivery::v1::NodeDeliveryBatch* batch);
ValidationError ParseAckJsonFile(const std::string& path, delivery::v1::DeliveryAck* ack);
ValidationError ParseObservationJsonFile(const std::string& path,
                                         delivery::v1::NodeDeliveryObservation* observation);
ValidationError ValidateGoldenDirectory(const std::string& directory);

}  // namespace dipole::realtime

#endif  // DIPOLE_REALTIME_DELIVERY_CONTRACT_VALIDATOR_HPP_
