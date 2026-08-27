#include "shadow_runtime.hpp"

#include <chrono>
#include <cstdlib>
#include <fstream>
#include <iostream>
#include <memory>
#include <string>
#include <string_view>
#include <thread>
#include <vector>

#include "shadow_evidence.hpp"

namespace dipole::realtime {
namespace {

std::string Environment(const char* name, std::string_view fallback = {}) {
  const char* value = std::getenv(name);
  return value == nullptr || *value == '\0' ? std::string(fallback) : value;
}

std::vector<std::string> Split(const std::string& value) {
  std::vector<std::string> result;
  std::size_t start = 0;
  while (start <= value.size()) {
    const auto end = value.find(',', start);
    result.push_back(value.substr(start, end == std::string::npos ? end : end - start));
    if (end == std::string::npos) {
      break;
    }
    start = end + 1;
  }
  return result;
}

bool ParseBoundedInt(const std::string& raw, int minimum, int maximum, int* output) {
  try {
    std::size_t consumed = 0;
    const int value = std::stoi(raw, &consumed);
    if (consumed != raw.size() || value < minimum || value > maximum) {
      return false;
    }
    *output = value;
    return true;
  } catch (...) {
    return false;
  }
}

bool ParseBool(const std::string& raw, bool* output) {
  if (raw == "true") {
    *output = true;
    return true;
  }
  if (raw == "false") {
    *output = false;
    return true;
  }
  return false;
}

ValidationError ParseEndpoints(const std::string& raw, std::vector<RedisEndpoint>* endpoints) {
  endpoints->clear();
  if (raw.empty()) return std::nullopt;
  for (const auto& value : Split(raw)) {
    RedisEndpoint endpoint;
    if (auto error = ParseRedisEndpoint(value, &endpoint)) return error;
    endpoints->push_back(std::move(endpoint));
  }
  return std::nullopt;
}

}  // namespace

ValidationError ValidateDeliveryRuntimeConfig(const DeliveryRuntimeConfig& config) {
  const bool primary = config.authority == DeliveryRuntimeAuthority::kPrimary;
  const std::string_view authority_name = primary ? "primary" : "shadow";
  if (config.health.mode != authority_name) {
    return "delivery runtime health mode must match its authority";
  }
  if (config.health.host != "0.0.0.0" && config.health.host != "127.0.0.1") {
    return "delivery runtime host is invalid";
  }
  if (config.health.port == 0 || config.evidence_file.empty()) {
    return "delivery runtime port and evidence file are required";
  }
  if (config.poll_timeout_ms < 10 || config.poll_timeout_ms > 5000 || config.error_backoff_ms < 10 ||
      config.error_backoff_ms > 30000) {
    return "delivery runtime timing is out of range";
  }
  if (config.kafka.authority != (primary ? KafkaConsumerAuthority::kPrimary : KafkaConsumerAuthority::kShadow)) {
    return "Kafka consumer authority must match the delivery runtime";
  }
  if (auto error = ValidateLibrdkafkaConsumerConfig(config.kafka)) return error;
  if (primary && !config.primary_enabled) return "primary runtime enable gate is required";
  if (primary && (!config.presence_enabled || !config.node_transport_enabled)) {
    return "primary runtime requires Presence and node transport";
  }
  if (config.presence_enabled) {
    if (config.presence_ttl_ms < 1000 || config.presence_ttl_ms > 3'600'000) {
      return "delivery Presence ttl is out of range";
    }
    if (auto error = ValidateHiredisPresenceConfig(config.presence)) return error;
  }
  if (config.node_transport_enabled) {
    if (!config.presence_enabled) return "node transport requires Presence routing";
    return ValidateGrpcNodeTransportConfig(config.node_transport);
  }
  return std::nullopt;
}

ValidationError LoadDeliveryRuntimeConfig(DeliveryRuntimeAuthority authority, DeliveryRuntimeConfig* config) {
  if (config == nullptr) {
    return "delivery runtime config destination is required";
  }
  *config = {};
  const bool primary = authority == DeliveryRuntimeAuthority::kPrimary;
  const std::string authority_name = primary ? "primary" : "shadow";
  const std::string desired_authority = Environment("DIPOLE_REALTIME_DELIVERY");
  const std::string expected_authority = primary ? "cpp" : "shadow";
  if (desired_authority != expected_authority) {
    return "DIPOLE_REALTIME_DELIVERY must match the runtime authority";
  }
  config->authority = authority;
  config->primary_enabled = false;
  if (primary && !ParseBool(Environment("DIPOLE_REALTIME_PRIMARY_ENABLED", "false"), &config->primary_enabled)) {
    return "DIPOLE_REALTIME_PRIMARY_ENABLED must be true or false";
  }
  if (primary && !config->primary_enabled) return "primary runtime enable gate is required";
  config->health = {.host = Environment("DIPOLE_REALTIME_HOST", "0.0.0.0"), .port = 0, .mode = authority_name};
  int port = 0;
  if (!ParseBoundedInt(Environment("DIPOLE_REALTIME_PORT", "8092"), 1, 65535, &port)) {
    return "DIPOLE_REALTIME_PORT must be between 1 and 65535";
  }
  config->health.port = static_cast<std::uint16_t>(port);
  config->kafka.authority = primary ? KafkaConsumerAuthority::kPrimary : KafkaConsumerAuthority::kShadow;
  config->kafka.brokers = Split(Environment("DIPOLE_REALTIME_KAFKA_BROKERS"));
  config->kafka.client_id =
      Environment("DIPOLE_REALTIME_KAFKA_CLIENT_ID", primary ? "dipole-realtime-primary" : "dipole-realtime-shadow");
  config->kafka.group_id = Environment("DIPOLE_REALTIME_KAFKA_GROUP_ID",
                                       primary ? "dipole-realtime-primary-v1" : "dipole-realtime-shadow-v1");
  config->evidence_file = Environment("DIPOLE_REALTIME_EVIDENCE_FILE");
  if (!ParseBoundedInt(Environment("DIPOLE_REALTIME_POLL_TIMEOUT_MS", "250"), 10, 5000, &config->poll_timeout_ms) ||
      !ParseBoundedInt(Environment("DIPOLE_REALTIME_ERROR_BACKOFF_MS", "250"), 10, 30000, &config->error_backoff_ms)) {
    return "delivery runtime timing environment is invalid";
  }
  const auto timeline_mode = Environment("DIPOLE_REALTIME_TIMELINE_NOTIFY_MODE", "off");
  if (timeline_mode != "off" && timeline_mode != authority_name) {
    return "DIPOLE_REALTIME_TIMELINE_NOTIFY_MODE must be off or match runtime "
           "authority";
  }
  config->timeline_notify_enabled = timeline_mode == authority_name;
  const auto presence_mode = Environment("DIPOLE_REALTIME_PRESENCE_MODE", "off");
  if (presence_mode != "off" && presence_mode != authority_name) {
    return "DIPOLE_REALTIME_PRESENCE_MODE must be off or match runtime "
           "authority";
  }
  config->presence_enabled = presence_mode == authority_name;
  if (config->presence_enabled) {
    const auto direct = Environment("DIPOLE_REALTIME_REDIS_ENDPOINT");
    if (!direct.empty()) {
      if (auto error = ParseRedisEndpoint(direct, &config->presence.direct)) return error;
    }
    if (auto error = ParseEndpoints(Environment("DIPOLE_REALTIME_REDIS_SENTINELS"), &config->presence.sentinels)) {
      return error;
    }
    config->presence.sentinel_master_name = Environment("DIPOLE_REALTIME_REDIS_MASTER_NAME");
    config->presence.password = Environment("DIPOLE_REALTIME_REDIS_PASSWORD");
    config->presence.sentinel_password = Environment("DIPOLE_REALTIME_REDIS_SENTINEL_PASSWORD");
    int db = 0;
    int timeout_ms = 0;
    int ttl_seconds = 0;
    if (!ParseBoundedInt(Environment("DIPOLE_REALTIME_REDIS_DB", "0"), 0, 15, &db) ||
        !ParseBoundedInt(Environment("DIPOLE_REALTIME_REDIS_TIMEOUT_MS", "500"), 10, 5000, &timeout_ms) ||
        !ParseBoundedInt(Environment("DIPOLE_REALTIME_PRESENCE_TTL_SECONDS", "120"), 1, 3600, &ttl_seconds)) {
      return "delivery Presence Redis environment is invalid";
    }
    config->presence.db = db;
    config->presence.timeout_ms = timeout_ms;
    config->presence_ttl_ms = static_cast<std::int64_t>(ttl_seconds) * 1000;
  }
  const auto transport_mode = Environment("DIPOLE_REALTIME_NODE_TRANSPORT_MODE", "off");
  if (transport_mode != "off" && transport_mode != authority_name) {
    return "DIPOLE_REALTIME_NODE_TRANSPORT_MODE must be off or match runtime "
           "authority";
  }
  config->node_transport_enabled = transport_mode == authority_name;
  if (config->node_transport_enabled) {
    if (auto error = ParseNodeTargets(Environment("DIPOLE_REALTIME_NODE_TARGETS"), &config->node_transport.targets)) {
      return error;
    }
    config->node_transport.shared_secret = Environment("DIPOLE_INTERNAL_RPC_SHARED_SECRET");
    if (!ParseBoundedInt(Environment("DIPOLE_REALTIME_NODE_TIMEOUT_MS", "500"), 10, 30'000,
                         &config->node_transport.timeout_ms) ||
        !ParseBool(Environment("DIPOLE_REALTIME_NODE_TLS_ENABLED", "false"), &config->node_transport.tls_enabled)) {
      return "delivery node transport environment is invalid";
    }
    config->node_transport.tls_ca_file = Environment("DIPOLE_REALTIME_NODE_TLS_CA_FILE");
    config->node_transport.tls_cert_file = Environment("DIPOLE_REALTIME_NODE_TLS_CERT_FILE");
    config->node_transport.tls_key_file = Environment("DIPOLE_REALTIME_NODE_TLS_KEY_FILE");
    config->node_transport.tls_server_name = Environment("DIPOLE_REALTIME_NODE_TLS_SERVER_NAME");
  }
  return ValidateDeliveryRuntimeConfig(*config);
}

ValidationError ValidateShadowRuntimeConfig(const ShadowRuntimeConfig& config) {
  if (config.authority != DeliveryRuntimeAuthority::kShadow) {
    return "shadow runtime authority is required";
  }
  return ValidateDeliveryRuntimeConfig(config);
}

ValidationError ValidatePrimaryRuntimeConfig(const PrimaryRuntimeConfig& config) {
  if (config.authority != DeliveryRuntimeAuthority::kPrimary) {
    return "primary runtime authority is required";
  }
  return ValidateDeliveryRuntimeConfig(config);
}

ValidationError LoadShadowRuntimeConfig(ShadowRuntimeConfig* config) {
  return LoadDeliveryRuntimeConfig(DeliveryRuntimeAuthority::kShadow, config);
}

ValidationError LoadPrimaryRuntimeConfig(PrimaryRuntimeConfig* config) {
  return LoadDeliveryRuntimeConfig(DeliveryRuntimeAuthority::kPrimary, config);
}

int RunDelivery(const DeliveryRuntimeConfig& config, volatile std::sig_atomic_t& running) {
  if (const auto error = ValidateDeliveryRuntimeConfig(config); error) {
    std::cerr << *error << '\n';
    return 2;
  }
  std::ofstream evidence(config.evidence_file, std::ios::app);
  if (!evidence) {
    std::cerr << "open delivery evidence file: " << config.evidence_file << '\n';
    return 1;
  }
  std::unique_ptr<LibrdkafkaConsumer> consumer;
  if (const auto error = LibrdkafkaConsumer::Create(config.kafka, &consumer); error) {
    std::cerr << *error << '\n';
    return 1;
  }
  JsonLineEvidenceSink sink(&evidence);
  std::unique_ptr<HiredisPresenceReader> presence_reader;
  if (config.presence_enabled) {
    presence_reader = std::make_unique<HiredisPresenceReader>(config.presence);
  }
  std::unique_ptr<GrpcNodeBatchTransport> node_transport;
  if (config.node_transport_enabled) {
    if (const auto error = GrpcNodeBatchTransport::Create(config.node_transport, &node_transport); error) {
      std::cerr << *error << '\n';
      return 1;
    }
  }
  const auto transport_mode = config.authority == DeliveryRuntimeAuthority::kPrimary ? NodeTransportMode::kPrimary
                                                                                     : NodeTransportMode::kObserve;
  ShadowRunner runner(consumer.get(), &sink, config.poll_timeout_ms, presence_reader.get(), node_transport.get(),
                      transport_mode);
  const ProjectionPolicy policy{.timeline_notify_shadow = config.timeline_notify_enabled};

  std::thread worker([&]() {
    while (running != 0) {
      std::optional<PresenceProjectionPolicy> presence_policy;
      if (config.presence_enabled) {
        const auto now = std::chrono::system_clock::now().time_since_epoch();
        presence_policy = {.now_unix_ms = std::chrono::duration_cast<std::chrono::milliseconds>(now).count(),
                           .ttl_ms = config.presence_ttl_ms};
      }
      if (const auto error = runner.RunOnce(policy, presence_policy); error) {
        std::cerr << *error << '\n';
        std::this_thread::sleep_for(std::chrono::milliseconds(config.error_backoff_ms));
      }
    }
  });
  const int health_result = ServeHealth(config.health, running, [&runner]() { return runner.Ready(); });
  running = 0;
  worker.join();
  return health_result;
}

int RunShadow(const ShadowRuntimeConfig& config, volatile std::sig_atomic_t& running) {
  if (const auto error = ValidateShadowRuntimeConfig(config); error) {
    std::cerr << *error << '\n';
    return 2;
  }
  return RunDelivery(config, running);
}

int RunPrimary(const PrimaryRuntimeConfig& config, volatile std::sig_atomic_t& running) {
  if (const auto error = ValidatePrimaryRuntimeConfig(config); error) {
    std::cerr << *error << '\n';
    return 2;
  }
  return RunDelivery(config, running);
}

}  // namespace dipole::realtime
