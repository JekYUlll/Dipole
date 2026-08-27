#include "node_delivery_transport.hpp"

#include <chrono>
#include <fstream>
#include <sstream>
#include <string_view>
#include <utility>

#include <arpa/inet.h>
#include <grpcpp/grpcpp.h>

namespace dipole::realtime {
namespace {

constexpr std::string_view kCallerService = "dipole-realtime";

std::string Trim(const std::string& value) {
  const auto begin = value.find_first_not_of(" \t\r\n");
  if (begin == std::string::npos) return {};
  const auto end = value.find_last_not_of(" \t\r\n");
  return value.substr(begin, end - begin + 1);
}

bool IsLoopbackTarget(const std::string& target) {
  const auto separator = target.rfind(':');
  if (separator == std::string::npos || separator + 1 >= target.size()) return false;
  const auto host = target.substr(0, separator);
  if (host == "localhost" || host == "[::1]") return true;
  in_addr address{};
  return inet_pton(AF_INET, host.c_str(), &address) == 1 &&
         (ntohl(address.s_addr) >> 24U) == 127U;
}

ValidationError ReadFile(const std::string& path, std::string* content) {
  std::ifstream input(path, std::ios::binary);
  if (!input) return "read node transport TLS material";
  std::ostringstream buffer;
  buffer << input.rdbuf();
  *content = buffer.str();
  if (content->empty()) return "node transport TLS material is empty";
  return std::nullopt;
}

}  // namespace

ValidationError ParseNodeTargets(const std::string& raw,
                                 std::map<std::string, std::string>* targets) {
  if (targets == nullptr) return "node target destination is required";
  targets->clear();
  std::size_t start = 0;
  while (start <= raw.size()) {
    const auto end = raw.find(',', start);
    const auto item = Trim(raw.substr(start, end == std::string::npos ? end : end - start));
    if (item.empty()) return "node target entry is empty";
    const auto separator = item.find('=');
    if (separator == std::string::npos || item.find('=', separator + 1) != std::string::npos) {
      return "node target entry must use node=host:port";
    }
    const auto node = Trim(item.substr(0, separator));
    const auto target = Trim(item.substr(separator + 1));
    if (node.empty() || target.empty()) return "node target identity is required";
    if (!targets->emplace(node, target).second) return "node target is duplicated";
    if (end == std::string::npos) break;
    start = end + 1;
  }
  return targets->empty() ? ValidationError("at least one node target is required") : std::nullopt;
}

ValidationError ValidateGrpcNodeTransportConfig(const GrpcNodeTransportConfig& config) {
  if (config.targets.empty() || Trim(config.shared_secret).empty()) {
    return "node transport targets and shared secret are required";
  }
  if (config.timeout_ms < 10 || config.timeout_ms > 30'000) {
    return "node transport timeout is out of range";
  }
  for (const auto& [node, target] : config.targets) {
    if (Trim(node).empty() || Trim(target).empty()) return "node target identity is required";
    if (!config.tls_enabled && !IsLoopbackTarget(target)) {
      return "plaintext node transport target must use loopback";
    }
  }
  if (config.tls_enabled &&
      (config.tls_ca_file.empty() || config.tls_cert_file.empty() || config.tls_key_file.empty() ||
       config.tls_server_name.empty())) {
    return "node transport mTLS material and server name are required";
  }
  return std::nullopt;
}

ValidationError GrpcNodeBatchTransport::Create(
    const GrpcNodeTransportConfig& config,
    std::unique_ptr<GrpcNodeBatchTransport>* transport) {
  if (transport == nullptr) return "node transport destination is required";
  if (const auto error = ValidateGrpcNodeTransportConfig(config); error) return error;

  std::shared_ptr<grpc::ChannelCredentials> credentials;
  if (config.tls_enabled) {
    grpc::SslCredentialsOptions options;
    if (auto error = ReadFile(config.tls_ca_file, &options.pem_root_certs); error) return error;
    if (auto error = ReadFile(config.tls_key_file, &options.pem_private_key); error) return error;
    if (auto error = ReadFile(config.tls_cert_file, &options.pem_cert_chain); error) return error;
    credentials = grpc::SslCredentials(options);
  } else {
    credentials = grpc::InsecureChannelCredentials();
  }

  auto result = std::unique_ptr<GrpcNodeBatchTransport>(new GrpcNodeBatchTransport());
  result->config_ = config;
  for (const auto& [node, target] : config.targets) {
    grpc::ChannelArguments arguments;
    if (config.tls_enabled) {
      arguments.SetSslTargetNameOverride(config.tls_server_name);
    }
    auto channel = grpc::CreateCustomChannel(target, credentials, arguments);
    result->stubs_.emplace(node, delivery::v1::NodeDeliveryService::NewStub(channel));
  }
  *transport = std::move(result);
  return std::nullopt;
}

ValidationError GrpcNodeBatchTransport::Observe(
    const std::vector<delivery::v1::NodeDeliveryBatch>& batches,
    NodeTransportStats* stats) {
  if (stats == nullptr) return "node transport stats destination is required";
  *stats = {};
  for (const auto& batch : batches) {
    if (const auto error = ValidateNodeBatch(batch); error) return error;
    const auto found = stubs_.find(batch.target_node_id());
    if (found == stubs_.end()) return "node transport target is unavailable";
    ++stats->requested;
    grpc::ClientContext context;
    context.set_deadline(std::chrono::system_clock::now() +
                         std::chrono::milliseconds(config_.timeout_ms));
    context.AddMetadata("x-dipole-caller-service", std::string(kCallerService));
    context.AddMetadata("x-dipole-service-token", config_.shared_secret);
    if (!batch.request_id().empty()) context.AddMetadata("x-request-id", batch.request_id());
    if (!batch.trace_id().empty()) context.AddMetadata("x-trace-id", batch.trace_id());

    delivery::v1::NodeDeliveryObservation observation;
    const auto status = found->second->ObserveNodeBatch(&context, batch, &observation);
    if (!status.ok()) {
      return "node observation RPC failed with code " +
             std::to_string(static_cast<int>(status.error_code()));
    }
    if (const auto error = ValidateObservation(observation); error) return error;
    if (observation.batch_id() != batch.batch_id() ||
        observation.target_node_id() != batch.target_node_id()) {
      return "node observation identity drifted";
    }
    switch (observation.status()) {
      case delivery::v1::NODE_OBSERVATION_STATUS_OBSERVED:
        ++stats->observed;
        if (observation.duplicate()) ++stats->duplicate;
        break;
      case delivery::v1::NODE_OBSERVATION_STATUS_REJECTED:
        ++stats->rejected;
        return "node observation was rejected";
      case delivery::v1::NODE_OBSERVATION_STATUS_BACKPRESSURED:
        ++stats->backpressured;
        return "node observation was backpressured";
      default:
        return "node observation status is invalid";
    }
  }
  return std::nullopt;
}

}  // namespace dipole::realtime
