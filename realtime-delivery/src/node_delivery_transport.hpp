#ifndef DIPOLE_REALTIME_DELIVERY_NODE_DELIVERY_TRANSPORT_HPP_
#define DIPOLE_REALTIME_DELIVERY_NODE_DELIVERY_TRANSPORT_HPP_

#include <cstddef>
#include <map>
#include <memory>
#include <string>
#include <vector>

#include "contract_validator.hpp"
#include "dipole/delivery/v1/delivery.grpc.pb.h"

namespace dipole::realtime {

struct NodeTransportStats {
  std::size_t requested = 0;
  std::size_t observed = 0;
  std::size_t duplicate = 0;
  std::size_t rejected = 0;
  std::size_t backpressured = 0;
};

class NodeBatchTransport {
 public:
  virtual ~NodeBatchTransport() = default;
  virtual ValidationError Observe(
      const std::vector<delivery::v1::NodeDeliveryBatch>& batches,
      NodeTransportStats* stats) = 0;
};

struct GrpcNodeTransportConfig {
  std::map<std::string, std::string> targets;
  std::string shared_secret;
  int timeout_ms = 500;
  bool tls_enabled = false;
  std::string tls_ca_file;
  std::string tls_cert_file;
  std::string tls_key_file;
  std::string tls_server_name;
};

ValidationError ParseNodeTargets(const std::string& raw,
                                 std::map<std::string, std::string>* targets);
ValidationError ValidateGrpcNodeTransportConfig(const GrpcNodeTransportConfig& config);

class GrpcNodeBatchTransport final : public NodeBatchTransport {
 public:
  static ValidationError Create(const GrpcNodeTransportConfig& config,
                                std::unique_ptr<GrpcNodeBatchTransport>* transport);

  ValidationError Observe(const std::vector<delivery::v1::NodeDeliveryBatch>& batches,
                          NodeTransportStats* stats) override;

 private:
  GrpcNodeBatchTransport() = default;

  GrpcNodeTransportConfig config_;
  std::map<std::string, std::unique_ptr<delivery::v1::NodeDeliveryService::Stub>> stubs_;
};

}  // namespace dipole::realtime

#endif  // DIPOLE_REALTIME_DELIVERY_NODE_DELIVERY_TRANSPORT_HPP_
