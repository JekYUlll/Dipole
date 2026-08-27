#include <csignal>
#include <iostream>
#include <string>

#include "contract_validator.hpp"
#include "health_server.hpp"

namespace {

volatile std::sig_atomic_t running = 1;

void Stop(int) {
  running = 0;
}

int ValidateGoldens(const std::string& directory) {
  if (const auto error = dipole::realtime::ValidateGoldenDirectory(directory); error.has_value()) {
    std::cerr << *error << '\n';
    return 1;
  }

  std::cout << "delivery contract v1 golden vectors valid\n";
  return 0;
}

}  // namespace

int main(int argc, char** argv) {
  if (argc == 3 && std::string(argv[1]) == "validate") {
    return ValidateGoldens(argv[2]);
  }
  if (argc == 3 && std::string(argv[1]) == "serve") {
    if (ValidateGoldens(argv[2]) != 0) {
      return 1;
    }
    dipole::realtime::RuntimeConfig config;
    if (const auto error = dipole::realtime::LoadRuntimeConfig(&config); error.has_value()) {
      std::cerr << *error << '\n';
      return 2;
    }
    std::signal(SIGINT, Stop);
    std::signal(SIGTERM, Stop);
    return dipole::realtime::ServeHealth(config, running);
  }
  std::cerr << "usage: dipole-realtime-delivery <validate|serve> <testdata-dir>\n";
  return 2;
}
