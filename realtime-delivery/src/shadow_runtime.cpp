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

}  // namespace

ValidationError ValidateShadowRuntimeConfig(const ShadowRuntimeConfig& config) {
  if (config.health.mode != "shadow") {
    return "shadow runtime health mode must be shadow";
  }
  if (config.health.host != "0.0.0.0" && config.health.host != "127.0.0.1") {
    return "shadow runtime host is invalid";
  }
  if (config.health.port == 0 || config.evidence_file.empty()) {
    return "shadow runtime port and evidence file are required";
  }
  if (config.poll_timeout_ms < 10 || config.poll_timeout_ms > 5000 ||
      config.error_backoff_ms < 10 || config.error_backoff_ms > 30000) {
    return "shadow runtime timing is out of range";
  }
  return ValidateLibrdkafkaConsumerConfig(config.kafka);
}

ValidationError LoadShadowRuntimeConfig(ShadowRuntimeConfig* config) {
  if (config == nullptr) {
    return "shadow runtime config destination is required";
  }
  config->health = {.host = Environment("DIPOLE_REALTIME_HOST", "0.0.0.0"),
                    .port = 0,
                    .mode = "shadow"};
  int port = 0;
  if (!ParseBoundedInt(Environment("DIPOLE_REALTIME_PORT", "8092"), 1, 65535, &port)) {
    return "DIPOLE_REALTIME_PORT must be between 1 and 65535";
  }
  config->health.port = static_cast<std::uint16_t>(port);
  config->kafka.brokers = Split(Environment("DIPOLE_REALTIME_KAFKA_BROKERS"));
  config->kafka.client_id = Environment("DIPOLE_REALTIME_KAFKA_CLIENT_ID", "dipole-realtime-shadow");
  config->kafka.group_id = Environment("DIPOLE_REALTIME_KAFKA_GROUP_ID", "dipole-realtime-shadow-v1");
  config->evidence_file = Environment("DIPOLE_REALTIME_EVIDENCE_FILE");
  if (!ParseBoundedInt(Environment("DIPOLE_REALTIME_POLL_TIMEOUT_MS", "250"), 10, 5000,
                       &config->poll_timeout_ms) ||
      !ParseBoundedInt(Environment("DIPOLE_REALTIME_ERROR_BACKOFF_MS", "250"), 10, 30000,
                       &config->error_backoff_ms)) {
    return "shadow runtime timing environment is invalid";
  }
  const auto timeline_mode = Environment("DIPOLE_REALTIME_TIMELINE_NOTIFY_MODE", "off");
  if (timeline_mode != "off" && timeline_mode != "shadow") {
    return "DIPOLE_REALTIME_TIMELINE_NOTIFY_MODE must be off or shadow";
  }
  config->timeline_notify_shadow = timeline_mode == "shadow";
  return ValidateShadowRuntimeConfig(*config);
}

int RunShadow(const ShadowRuntimeConfig& config, volatile std::sig_atomic_t& running) {
  if (const auto error = ValidateShadowRuntimeConfig(config); error) {
    std::cerr << *error << '\n';
    return 2;
  }
  std::ofstream evidence(config.evidence_file, std::ios::app);
  if (!evidence) {
    std::cerr << "open shadow evidence file: " << config.evidence_file << '\n';
    return 1;
  }
  std::unique_ptr<LibrdkafkaConsumer> consumer;
  if (const auto error = LibrdkafkaConsumer::Create(config.kafka, &consumer); error) {
    std::cerr << *error << '\n';
    return 1;
  }
  JsonLineEvidenceSink sink(&evidence);
  ShadowRunner runner(consumer.get(), &sink, config.poll_timeout_ms);
  const ProjectionPolicy policy{.timeline_notify_shadow = config.timeline_notify_shadow};

  std::thread worker([&]() {
    while (running != 0) {
      if (const auto error = runner.RunOnce(policy); error) {
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

}  // namespace dipole::realtime
