#include "librdkafka_consumer.hpp"

#include <atomic>
#include <cstdint>
#include <limits>
#include <memory>
#include <string>
#include <string_view>
#include <unordered_set>
#include <utility>

#include <librdkafka/rdkafka.h>

namespace dipole::realtime {
namespace {

constexpr int kMinimumSessionTimeoutMs = 6000;
constexpr int kMaximumSessionTimeoutMs = 300000;

bool IsBlank(std::string_view value) {
  return value.empty() || value.find_first_not_of(" \t\r\n") == std::string_view::npos;
}

ValidationError SetConfig(rd_kafka_conf_t* config, const char* name, const std::string& value) {
  char error[512] = {};
  if (rd_kafka_conf_set(config, name, value.c_str(), error, sizeof(error)) != RD_KAFKA_CONF_OK) {
    return "set librdkafka " + std::string(name) + ": " + error;
  }
  return std::nullopt;
}

std::string Join(const std::vector<std::string>& values) {
  std::string joined;
  for (const auto& value : values) {
    if (!joined.empty()) {
      joined += ',';
    }
    joined += value;
  }
  return joined;
}

}  // namespace

struct LibrdkafkaConsumer::Impl {
  rd_kafka_t* handle = nullptr;
  std::atomic<std::size_t> assignment_count = 0;

  ~Impl() {
    if (handle != nullptr) {
      rd_kafka_consumer_close(handle);
      rd_kafka_destroy(handle);
    }
  }

  static void Rebalance(rd_kafka_t* consumer, rd_kafka_resp_err_t error,
                        rd_kafka_topic_partition_list_t* partitions, void* opaque) {
    auto* self = static_cast<Impl*>(opaque);
    if (self == nullptr) {
      return;
    }
    if (error == RD_KAFKA_RESP_ERR__ASSIGN_PARTITIONS) {
      const auto assign_error = rd_kafka_assign(consumer, partitions);
      self->assignment_count.store(assign_error == RD_KAFKA_RESP_ERR_NO_ERROR
                                        ? static_cast<std::size_t>(partitions->cnt)
                                        : 0);
      return;
    }
    self->assignment_count.store(0);
    rd_kafka_assign(consumer, nullptr);
  }
};

ValidationError ValidateLibrdkafkaConsumerConfig(const LibrdkafkaConsumerConfig& config) {
  if (config.brokers.empty()) {
    return "librdkafka brokers are required";
  }
  for (const auto& broker : config.brokers) {
    if (IsBlank(broker)) {
      return "librdkafka broker is empty";
    }
  }
  if (IsBlank(config.client_id)) {
    return "librdkafka client_id is required";
  }
  if (!config.group_id.starts_with(kShadowGroupPrefix) ||
      config.group_id.size() <= std::string_view(kShadowGroupPrefix).size()) {
    return "librdkafka group_id must use the dedicated shadow prefix";
  }
  const std::unordered_set<std::string> expected = {
      "dipole.message.direct.created",
      "dipole.message.group.created",
  };
  const std::unordered_set<std::string> actual(config.topics.begin(), config.topics.end());
  if (actual != expected || config.topics.size() != expected.size()) {
    return "librdkafka topics must be the two canonical message-created topics";
  }
  if (config.session_timeout_ms < kMinimumSessionTimeoutMs ||
      config.session_timeout_ms > kMaximumSessionTimeoutMs) {
    return "librdkafka session_timeout_ms is out of range";
  }
  if (config.heartbeat_interval_ms <= 0 ||
      config.heartbeat_interval_ms * 3 > config.session_timeout_ms) {
    return "librdkafka heartbeat_interval_ms must fit the session timeout";
  }
  return std::nullopt;
}

LibrdkafkaConsumer::LibrdkafkaConsumer(std::unique_ptr<Impl> impl) : impl_(std::move(impl)) {}

LibrdkafkaConsumer::~LibrdkafkaConsumer() = default;

ValidationError LibrdkafkaConsumer::Create(const LibrdkafkaConsumerConfig& config,
                                           std::unique_ptr<LibrdkafkaConsumer>* output) {
  if (output == nullptr) {
    return "librdkafka consumer output is required";
  }
  output->reset();
  if (auto error = ValidateLibrdkafkaConsumerConfig(config); error) {
    return error;
  }

  auto impl = std::make_unique<Impl>();
  rd_kafka_conf_t* raw_config = rd_kafka_conf_new();
  const std::pair<const char*, std::string> settings[] = {
      {"bootstrap.servers", Join(config.brokers)},
      {"client.id", config.client_id},
      {"group.id", config.group_id},
      {"auto.offset.reset", "earliest"},
      {"enable.auto.commit", "false"},
      {"enable.auto.offset.store", "false"},
      {"partition.assignment.strategy", "roundrobin"},
      {"session.timeout.ms", std::to_string(config.session_timeout_ms)},
      {"heartbeat.interval.ms", std::to_string(config.heartbeat_interval_ms)},
  };
  for (const auto& [name, value] : settings) {
    if (auto error = SetConfig(raw_config, name, value); error) {
      rd_kafka_conf_destroy(raw_config);
      return error;
    }
  }
  rd_kafka_conf_set_opaque(raw_config, impl.get());
  rd_kafka_conf_set_rebalance_cb(raw_config, &Impl::Rebalance);

  char error[512] = {};
  impl->handle = rd_kafka_new(RD_KAFKA_CONSUMER, raw_config, error, sizeof(error));
  if (impl->handle == nullptr) {
    return "create librdkafka consumer: " + std::string(error);
  }
  rd_kafka_poll_set_consumer(impl->handle);

  rd_kafka_topic_partition_list_t* topics =
      rd_kafka_topic_partition_list_new(static_cast<int>(config.topics.size()));
  for (const auto& topic : config.topics) {
    rd_kafka_topic_partition_list_add(topics, topic.c_str(), RD_KAFKA_PARTITION_UA);
  }
  const auto subscribe_error = rd_kafka_subscribe(impl->handle, topics);
  rd_kafka_topic_partition_list_destroy(topics);
  if (subscribe_error != RD_KAFKA_RESP_ERR_NO_ERROR) {
    return "subscribe librdkafka consumer: " + std::string(rd_kafka_err2str(subscribe_error));
  }

  output->reset(new LibrdkafkaConsumer(std::move(impl)));
  return std::nullopt;
}

PollResult LibrdkafkaConsumer::Poll(int timeout_ms) {
  if (impl_ == nullptr || impl_->handle == nullptr) {
    return {.status = PollStatus::kError, .record = {}, .error = "consumer is closed"};
  }
  if (timeout_ms <= 0) {
    return {.status = PollStatus::kError, .record = {}, .error = "poll timeout must be positive"};
  }

  rd_kafka_message_t* message = rd_kafka_consumer_poll(impl_->handle, timeout_ms);
  if (message == nullptr) {
    return {};
  }
  if (message->err != RD_KAFKA_RESP_ERR_NO_ERROR) {
    const auto error = message->err;
    rd_kafka_message_destroy(message);
    if (error == RD_KAFKA_RESP_ERR__PARTITION_EOF) {
      return {};
    }
    return {.status = PollStatus::kError,
            .record = {},
            .error = std::string(rd_kafka_err2str(error))};
  }

  KafkaRecord record;
  record.topic = rd_kafka_topic_name(message->rkt);
  record.partition = message->partition;
  record.offset = message->offset;
  if (message->payload != nullptr && message->len > 0) {
    record.value.assign(static_cast<const char*>(message->payload), message->len);
  }
  rd_kafka_message_destroy(message);
  return {.status = PollStatus::kRecord, .record = std::move(record), .error = {}};
}

ValidationError LibrdkafkaConsumer::Commit(const KafkaRecord& record) {
  if (impl_ == nullptr || impl_->handle == nullptr) {
    return "commit on closed librdkafka consumer";
  }
  if (IsBlank(record.topic) || record.partition < 0 || record.offset < 0 ||
      record.offset == std::numeric_limits<std::int64_t>::max()) {
    return "commit Kafka coordinates are invalid";
  }
  rd_kafka_topic_partition_list_t* offsets = rd_kafka_topic_partition_list_new(1);
  auto* partition =
      rd_kafka_topic_partition_list_add(offsets, record.topic.c_str(), record.partition);
  partition->offset = record.offset + 1;
  const auto commit_error = rd_kafka_commit(impl_->handle, offsets, 0);
  rd_kafka_topic_partition_list_destroy(offsets);
  if (commit_error != RD_KAFKA_RESP_ERR_NO_ERROR) {
    return "commit librdkafka offset: " + std::string(rd_kafka_err2str(commit_error));
  }
  return std::nullopt;
}

std::size_t LibrdkafkaConsumer::AssignmentCount() const {
  return impl_ == nullptr ? 0 : impl_->assignment_count.load();
}

}  // namespace dipole::realtime
