#include <cstdlib>
#include <iostream>
#include <string>

#include "librdkafka_consumer.hpp"

namespace {

int failures = 0;

void Check(bool condition, const std::string& message) {
  if (!condition) {
    std::cerr << "FAIL: " << message << '\n';
    ++failures;
  }
}

void TestCanonicalConfig() {
  dipole::realtime::LibrdkafkaConsumerConfig config;
  config.brokers = {"kafka-1:9092", "kafka-2:9092"};
  Check(!dipole::realtime::ValidateLibrdkafkaConsumerConfig(config),
        "canonical shadow config is valid");
}

void TestConfigRejectsAuthorityDrift() {
  dipole::realtime::LibrdkafkaConsumerConfig config;
  config.brokers = {"kafka:9092"};

  auto invalid = config;
  invalid.group_id = "dipole-consumer";
  Check(dipole::realtime::ValidateLibrdkafkaConsumerConfig(invalid).has_value(),
        "production group is rejected");

  invalid = config;
  invalid.topics = {"dipole.message.direct.created"};
  Check(dipole::realtime::ValidateLibrdkafkaConsumerConfig(invalid).has_value(),
        "partial topic set is rejected");

  invalid = config;
  invalid.topics.push_back("dipole.message.created.dead");
  Check(dipole::realtime::ValidateLibrdkafkaConsumerConfig(invalid).has_value(),
        "extra topic is rejected");

  invalid = config;
  invalid.brokers = {};
  Check(dipole::realtime::ValidateLibrdkafkaConsumerConfig(invalid).has_value(),
        "empty broker set is rejected");

  invalid = config;
  invalid.heartbeat_interval_ms = invalid.session_timeout_ms;
  Check(dipole::realtime::ValidateLibrdkafkaConsumerConfig(invalid).has_value(),
        "invalid heartbeat ratio is rejected");
}

void TestFactoryRequiresOutput() {
  dipole::realtime::LibrdkafkaConsumerConfig config;
  config.brokers = {"kafka:9092"};
  Check(dipole::realtime::LibrdkafkaConsumer::Create(config, nullptr).has_value(),
        "factory requires owned output");
}

}  // namespace

int main() {
  TestCanonicalConfig();
  TestConfigRejectsAuthorityDrift();
  TestFactoryRequiresOutput();
  return failures == 0 ? EXIT_SUCCESS : EXIT_FAILURE;
}
