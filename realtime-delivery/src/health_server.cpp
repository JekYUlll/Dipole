#include "health_server.hpp"

#include <arpa/inet.h>
#include <netinet/in.h>
#include <poll.h>
#include <sys/socket.h>
#include <unistd.h>

#include <cerrno>
#include <cstdlib>
#include <cstring>
#include <iostream>
#include <limits>
#include <string_view>

namespace dipole::realtime {
namespace {

constexpr char kDefaultPort[] = "8092";
constexpr int kRequestBufferSize = 4096;

enum class HttpStatus : std::uint16_t {
  kOk = 200,
  kServiceUnavailable = 503,
  kNotFound = 404,
};

std::string Environment(const std::string& name, std::string_view fallback) {
  const char* value = std::getenv(name.c_str());
  return value == nullptr || *value == '\0' ? std::string(fallback) : value;
}

bool ParsePort(const std::string& raw, std::uint16_t* port) {
  if (port == nullptr || raw.empty()) {
    return false;
  }
  std::size_t consumed = 0;
  unsigned long value = 0;
  try {
    value = std::stoul(raw, &consumed, 10);
  } catch (...) {
    return false;
  }
  if (consumed != raw.size() || value == 0 || value > std::numeric_limits<std::uint16_t>::max()) {
    return false;
  }
  *port = static_cast<std::uint16_t>(value);
  return true;
}

void Respond(int client, HttpStatus status, std::string_view body) {
  std::string status_text = "404 Not Found";
  if (status == HttpStatus::kOk) {
    status_text = "200 OK";
  } else if (status == HttpStatus::kServiceUnavailable) {
    status_text = "503 Service Unavailable";
  }
  const std::string response = "HTTP/1.1 " + status_text +
                               "\r\nContent-Type: application/json\r\nConnection: close\r\nContent-Length: " +
                               std::to_string(body.size()) + "\r\n\r\n" + std::string(body);
  std::size_t sent = 0;
  while (sent < response.size()) {
    const auto count = ::send(client, response.data() + sent, response.size() - sent, MSG_NOSIGNAL);
    if (count <= 0) {
      return;
    }
    sent += static_cast<std::size_t>(count);
  }
}

}  // namespace

std::optional<std::string> LoadRuntimeConfig(RuntimeConfig* config) {
  if (config == nullptr) {
    return "runtime config destination is required";
  }
  config->host = Environment("DIPOLE_REALTIME_HOST", "0.0.0.0");
  config->mode = Environment("DIPOLE_REALTIME_MODE", "contract_only");
  if (config->mode != "contract_only") {
    return "DIPOLE_REALTIME_MODE must be contract_only in the foundation milestone";
  }
  if (config->host != "0.0.0.0" && config->host != "127.0.0.1") {
    return "DIPOLE_REALTIME_HOST must be 0.0.0.0 or 127.0.0.1";
  }
  const std::string port = Environment("DIPOLE_REALTIME_PORT", kDefaultPort);
  if (!ParsePort(port, &config->port)) {
    return "DIPOLE_REALTIME_PORT must be between 1 and 65535";
  }
  return std::nullopt;
}

int ServeHealth(const RuntimeConfig& config, const volatile std::sig_atomic_t& running) {
  return ServeHealth(config, running, {});
}

int ServeHealth(const RuntimeConfig& config, const volatile std::sig_atomic_t& running,
                const std::function<bool()>& ready_probe) {
  const int server = ::socket(AF_INET, SOCK_STREAM | SOCK_CLOEXEC, 0);
  if (server < 0) {
    std::cerr << "create health socket: " << std::strerror(errno) << '\n';
    return 1;
  }
  const int reuse = 1;
  if (::setsockopt(server, SOL_SOCKET, SO_REUSEADDR, &reuse, sizeof(reuse)) != 0) {
    std::cerr << "configure health socket: " << std::strerror(errno) << '\n';
    ::close(server);
    return 1;
  }

  sockaddr_in address{};
  address.sin_family = AF_INET;
  address.sin_port = htons(config.port);
  if (::inet_pton(AF_INET, config.host.c_str(), &address.sin_addr) != 1 ||
      ::bind(server, reinterpret_cast<const sockaddr*>(&address), sizeof(address)) != 0 ||
      ::listen(server, 64) != 0) {
    std::cerr << "bind health socket " << config.host << ':' << config.port << ": "
              << std::strerror(errno) << '\n';
    ::close(server);
    return 1;
  }

  std::cout << "dipole-realtime-delivery ready mode=" << config.mode << " address=" << config.host
            << ':' << config.port << '\n';
  while (running != 0) {
    pollfd descriptor{server, POLLIN, 0};
    const int ready = ::poll(&descriptor, 1, 200);
    if (ready < 0 && errno != EINTR) {
      std::cerr << "poll health socket: " << std::strerror(errno) << '\n';
      ::close(server);
      return 1;
    }
    if (ready <= 0 || (descriptor.revents & POLLIN) == 0) {
      continue;
    }
    const int client = ::accept4(server, nullptr, nullptr, SOCK_CLOEXEC);
    if (client < 0) {
      continue;
    }
    timeval receive_timeout{};
    receive_timeout.tv_sec = 1;
    if (::setsockopt(client, SOL_SOCKET, SO_RCVTIMEO, &receive_timeout,
                     sizeof(receive_timeout)) != 0) {
      ::close(client);
      continue;
    }
    char request[kRequestBufferSize]{};
    const auto count = ::recv(client, request, sizeof(request) - 1, 0);
    const std::string_view line(request, count > 0 ? static_cast<std::size_t>(count) : 0);
    if (line.starts_with("GET /livez ")) {
      Respond(client, HttpStatus::kOk,
              "{\"status\":\"ok\",\"service\":\"dipole-realtime-delivery\",\"mode\":\"" +
                  config.mode + "\"}");
    } else if (line.starts_with("GET /readyz ") || line.starts_with("GET /health ")) {
      const bool ready = !ready_probe || ready_probe();
      Respond(client, ready ? HttpStatus::kOk : HttpStatus::kServiceUnavailable,
              "{\"status\":\"" + std::string(ready ? "ok" : "not_ready") +
                  "\",\"service\":\"dipole-realtime-delivery\",\"mode\":\"" + config.mode +
                  "\"}");
    } else {
      Respond(client, HttpStatus::kNotFound, R"({"status":"not_found"})");
    }
    ::close(client);
  }
  ::close(server);
  return 0;
}

}  // namespace dipole::realtime
