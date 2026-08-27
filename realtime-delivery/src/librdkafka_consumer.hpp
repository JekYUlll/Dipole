#ifndef DIPOLE_REALTIME_DELIVERY_LIBRDKAFKA_CONSUMER_HPP_
#define DIPOLE_REALTIME_DELIVERY_LIBRDKAFKA_CONSUMER_HPP_

#include <cstddef>
#include <cstdint>
#include <memory>
#include <string>
#include <vector>

#include "shadow_runner.hpp"

namespace dipole::realtime {

inline constexpr char kShadowGroupPrefix[] = "dipole-realtime-shadow-";
inline constexpr char kPrimaryGroupPrefix[] = "dipole-realtime-primary-";

enum class KafkaConsumerAuthority : std::uint8_t { kShadow, kPrimary };

struct LibrdkafkaConsumerConfig {
  KafkaConsumerAuthority authority = KafkaConsumerAuthority::kShadow;
  std::vector<std::string> brokers;
  std::string client_id = "dipole-realtime-shadow";
  std::string group_id = "dipole-realtime-shadow-v1";
  std::vector<std::string> topics = {
      "dipole.message.direct.created",
      "dipole.message.group.created",
  };
  int session_timeout_ms = 30000;
  int heartbeat_interval_ms = 3000;
};

ValidationError ValidateLibrdkafkaConsumerConfig(const LibrdkafkaConsumerConfig& config);

class LibrdkafkaConsumer final : public ShadowRecordConsumer {
 public:
  static ValidationError Create(const LibrdkafkaConsumerConfig& config, std::unique_ptr<LibrdkafkaConsumer>* output);

  ~LibrdkafkaConsumer() override;
  LibrdkafkaConsumer(const LibrdkafkaConsumer&) = delete;
  LibrdkafkaConsumer& operator=(const LibrdkafkaConsumer&) = delete;

  PollResult Poll(int timeout_ms) override;
  ValidationError Commit(const KafkaRecord& record) override;
  [[nodiscard]] std::size_t AssignmentCount() const override;

 private:
  struct Impl;
  explicit LibrdkafkaConsumer(std::unique_ptr<Impl> impl);

  std::unique_ptr<Impl> impl_;
};

}  // namespace dipole::realtime

#endif  // DIPOLE_REALTIME_DELIVERY_LIBRDKAFKA_CONSUMER_HPP_
