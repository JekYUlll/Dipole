#ifndef DIPOLE_REALTIME_DELIVERY_SHADOW_RUNTIME_HPP_
#define DIPOLE_REALTIME_DELIVERY_SHADOW_RUNTIME_HPP_

#include <csignal>
#include <cstdint>
#include <string>

#include "health_server.hpp"
#include "hiredis_presence_reader.hpp"
#include "librdkafka_consumer.hpp"
#include "node_delivery_transport.hpp"

namespace dipole::realtime {

enum class DeliveryRuntimeAuthority : std::uint8_t { kShadow, kPrimary };

struct DeliveryRuntimeConfig {
  DeliveryRuntimeAuthority authority = DeliveryRuntimeAuthority::kShadow;
  bool primary_enabled = false;
  RuntimeConfig health;
  LibrdkafkaConsumerConfig kafka;
  std::string evidence_file;
  int poll_timeout_ms = 250;
  int error_backoff_ms = 250;
  bool timeline_notify_enabled = false;
  bool presence_enabled = false;
  HiredisPresenceConfig presence;
  std::int64_t presence_ttl_ms = 120'000;
  bool fencing_enabled = false;
  std::string fencing_key;
  std::uint64_t fencing_epoch = 0;
  std::string instance_id;
  bool node_transport_enabled = false;
  GrpcNodeTransportConfig node_transport;
};

using ShadowRuntimeConfig = DeliveryRuntimeConfig;
using PrimaryRuntimeConfig = DeliveryRuntimeConfig;

ValidationError LoadShadowRuntimeConfig(ShadowRuntimeConfig* config);
ValidationError ValidateShadowRuntimeConfig(const ShadowRuntimeConfig& config);
int RunShadow(const ShadowRuntimeConfig& config, volatile std::sig_atomic_t& running);
ValidationError LoadPrimaryRuntimeConfig(PrimaryRuntimeConfig* config);
ValidationError ValidatePrimaryRuntimeConfig(const PrimaryRuntimeConfig& config);
int RunPrimary(const PrimaryRuntimeConfig& config, volatile std::sig_atomic_t& running);

}  // namespace dipole::realtime

#endif  // DIPOLE_REALTIME_DELIVERY_SHADOW_RUNTIME_HPP_
