#ifndef DIPOLE_REALTIME_DELIVERY_HEALTH_SERVER_HPP_
#define DIPOLE_REALTIME_DELIVERY_HEALTH_SERVER_HPP_

#include <csignal>
#include <cstdint>
#include <optional>
#include <string>

namespace dipole::realtime {

struct RuntimeConfig {
  std::string host;
  std::uint16_t port;
  std::string mode;
};

std::optional<std::string> LoadRuntimeConfig(RuntimeConfig* config);
int ServeHealth(const RuntimeConfig& config, const volatile std::sig_atomic_t& running);

}  // namespace dipole::realtime

#endif  // DIPOLE_REALTIME_DELIVERY_HEALTH_SERVER_HPP_
