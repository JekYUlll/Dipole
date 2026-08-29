#include "primary_probe.hpp"

#include <cstdlib>
#include <filesystem>
#include <fstream>
#include <iostream>
#include <memory>
#include <sstream>
#include <string>

#include <google/protobuf/util/json_util.h>

#include "node_delivery_transport.hpp"

namespace dipole::realtime {
namespace {

constexpr std::uintmax_t kMaximumBatchBytes = 1U << 20U;

std::string Environment(const char* name) {
  const char* value = std::getenv(name);
  return value == nullptr ? std::string{} : value;
}

ValidationError ParseTimeout(const std::string& raw, int* timeout_ms) {
  if (raw.empty()) {
    *timeout_ms = 500;
    return std::nullopt;
  }
  try {
    std::size_t consumed = 0;
    const int value = std::stoi(raw, &consumed);
    if (consumed != raw.size() || value < 10 || value > 30'000) {
      return "DIPOLE_REALTIME_NODE_TIMEOUT_MS must be between 10 and 30000";
    }
    *timeout_ms = value;
    return std::nullopt;
  } catch (...) {
    return "DIPOLE_REALTIME_NODE_TIMEOUT_MS must be between 10 and 30000";
  }
}

ValidationError ParseTlsEnabled(const std::string& raw, bool* enabled) {
  if (raw.empty() || raw == "false") {
    *enabled = false;
    return std::nullopt;
  }
  if (raw == "true") {
    *enabled = true;
    return std::nullopt;
  }
  return "DIPOLE_REALTIME_NODE_TLS_ENABLED must be true or false";
}

ValidationError LoadPrimaryProbeConfig(GrpcNodeTransportConfig* config) {
  if (config == nullptr) return "primary probe config destination is required";
  if (auto error = ParseNodeTargets(Environment("DIPOLE_REALTIME_NODE_TARGETS"),
                                    &config->targets)) {
    return error;
  }
  config->shared_secret = Environment("DIPOLE_INTERNAL_RPC_SHARED_SECRET");
  if (auto error = ParseTimeout(Environment("DIPOLE_REALTIME_NODE_TIMEOUT_MS"),
                                &config->timeout_ms)) {
    return error;
  }
  if (auto error = ParseTlsEnabled(Environment("DIPOLE_REALTIME_NODE_TLS_ENABLED"),
                                   &config->tls_enabled)) {
    return error;
  }
  config->tls_ca_file = Environment("DIPOLE_REALTIME_NODE_TLS_CA_FILE");
  config->tls_cert_file = Environment("DIPOLE_REALTIME_NODE_TLS_CERT_FILE");
  config->tls_key_file = Environment("DIPOLE_REALTIME_NODE_TLS_KEY_FILE");
  config->tls_server_name = Environment("DIPOLE_REALTIME_NODE_TLS_SERVER_NAME");
  return ValidateGrpcNodeTransportConfig(*config);
}

}  // namespace

ValidationError LoadPrimaryProbeBatch(const std::string& path,
                                      delivery::v1::NodeDeliveryBatch* batch) {
  if (batch == nullptr || path.empty()) return "primary probe batch destination and path are required";
  std::error_code size_error;
  const auto size = std::filesystem::file_size(path, size_error);
  if (size_error || size == 0 || size > kMaximumBatchBytes) {
    return "primary probe batch file must contain between 1 byte and 1 MiB";
  }
  std::ifstream input(path, std::ios::binary);
  if (!input) return "open primary probe batch file";
  std::ostringstream buffer;
  buffer << input.rdbuf();

  batch->Clear();
  google::protobuf::util::JsonParseOptions options;
  options.ignore_unknown_fields = false;
  const auto status = google::protobuf::util::JsonStringToMessage(buffer.str(), batch, options);
  if (!status.ok()) return "parse primary probe batch JSON";
  return ValidateNodeBatch(*batch);
}

int RunPrimaryProbe(const std::string& batch_path) {
  delivery::v1::NodeDeliveryBatch batch;
  if (const auto error = LoadPrimaryProbeBatch(batch_path, &batch); error) {
    std::cerr << *error << '\n';
    return 2;
  }
  GrpcNodeTransportConfig config;
  if (const auto error = LoadPrimaryProbeConfig(&config); error) {
    std::cerr << *error << '\n';
    return 2;
  }
  std::unique_ptr<GrpcNodeBatchTransport> transport;
  if (const auto error = GrpcNodeBatchTransport::Create(config, &transport); error) {
    std::cerr << *error << '\n';
    return 1;
  }
  std::vector<delivery::v1::DeliveryAck> acknowledgements;
  if (const auto error = transport->Deliver({batch}, &acknowledgements); error) {
    std::cerr << *error << '\n';
    return 1;
  }
  if (acknowledgements.size() != 1) {
    std::cerr << "primary probe acknowledgement count drifted\n";
    return 1;
  }
  std::string output;
  google::protobuf::util::JsonPrintOptions options;
  options.preserve_proto_field_names = true;
  const auto status = google::protobuf::util::MessageToJsonString(
      acknowledgements.front(), &output, options);
  if (!status.ok()) {
    std::cerr << "encode primary probe acknowledgement JSON\n";
    return 1;
  }
  std::cout << output << '\n';
  return 0;
}

}  // namespace dipole::realtime
