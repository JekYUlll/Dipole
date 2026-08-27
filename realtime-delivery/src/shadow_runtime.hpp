#ifndef DIPOLE_REALTIME_DELIVERY_SHADOW_RUNTIME_HPP_
#define DIPOLE_REALTIME_DELIVERY_SHADOW_RUNTIME_HPP_

#include <csignal>
#include <string>

#include "health_server.hpp"
#include "hiredis_presence_reader.hpp"
#include "librdkafka_consumer.hpp"

namespace dipole::realtime {

struct ShadowRuntimeConfig {
  RuntimeConfig health;
  LibrdkafkaConsumerConfig kafka;
  std::string evidence_file;
  int poll_timeout_ms = 250;
  int error_backoff_ms = 250;
  bool timeline_notify_shadow = false;
  bool presence_shadow = false;
  HiredisPresenceConfig presence;
  std::int64_t presence_ttl_ms = 120'000;
};

ValidationError LoadShadowRuntimeConfig(ShadowRuntimeConfig* config);
ValidationError ValidateShadowRuntimeConfig(const ShadowRuntimeConfig& config);
int RunShadow(const ShadowRuntimeConfig& config, volatile std::sig_atomic_t& running);

}  // namespace dipole::realtime

#endif  // DIPOLE_REALTIME_DELIVERY_SHADOW_RUNTIME_HPP_
