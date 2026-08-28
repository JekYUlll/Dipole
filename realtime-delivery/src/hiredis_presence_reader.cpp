#include "hiredis_presence_reader.hpp"

#include <cstdlib>
#include <memory>
#include <string>
#include <string_view>
#include <utility>
#include <vector>

#include <hiredis/hiredis.h>

namespace dipole::realtime {
namespace {

using Reply = std::unique_ptr<redisReply, decltype(&freeReplyObject)>;

bool ValidEndpoint(const RedisEndpoint& endpoint) { return !endpoint.host.empty() && endpoint.port > 0; }

Reply Command(redisContext* context, const std::vector<std::string>& arguments) {
  std::vector<const char*> values;
  std::vector<std::size_t> lengths;
  values.reserve(arguments.size());
  lengths.reserve(arguments.size());
  for (const auto& argument : arguments) {
    values.push_back(argument.data());
    lengths.push_back(argument.size());
  }
  return Reply(static_cast<redisReply*>(redisCommandArgv(context, static_cast<int>(values.size()),
                                                        values.data(), lengths.data())),
               freeReplyObject);
}

redisContext* Connect(const RedisEndpoint& endpoint, int timeout_ms) {
  const timeval timeout{.tv_sec = timeout_ms / 1000,
                        .tv_usec = static_cast<decltype(timeval::tv_usec)>(timeout_ms % 1000) * 1000};
  redisContext* context = redisConnectWithTimeout(endpoint.host.c_str(), endpoint.port, timeout);
  if (context == nullptr || context->err != 0) {
    if (context != nullptr) redisFree(context);
    return nullptr;
  }
  if (redisSetTimeout(context, timeout) != REDIS_OK) {
    redisFree(context);
    return nullptr;
  }
  return context;
}

bool Authenticate(redisContext* context, const std::string& password) {
  if (password.empty()) return true;
  const auto reply = Command(context, {"AUTH", password});
  return reply && reply->type == REDIS_REPLY_STATUS;
}

ValidationError DiscoverMaster(const HiredisPresenceConfig& config, RedisEndpoint* master) {
  for (const auto& sentinel : config.sentinels) {
    std::unique_ptr<redisContext, decltype(&redisFree)> context(Connect(sentinel, config.timeout_ms), redisFree);
    if (!context || !Authenticate(context.get(), config.sentinel_password)) continue;
    const auto reply = Command(context.get(), {"SENTINEL", "get-master-addr-by-name",
                                                config.sentinel_master_name});
    if (!reply || reply->type != REDIS_REPLY_ARRAY || reply->elements != 2 ||
        reply->element[0]->type != REDIS_REPLY_STRING || reply->element[1]->type != REDIS_REPLY_STRING) {
      continue;
    }
    const std::string endpoint = std::string(reply->element[0]->str, reply->element[0]->len) + ":" +
                                 std::string(reply->element[1]->str, reply->element[1]->len);
    if (!ParseRedisEndpoint(endpoint, master)) return std::nullopt;
  }
  return "discover Redis master from Sentinel";
}

}  // namespace

ValidationError ParseRedisEndpoint(const std::string& value, RedisEndpoint* endpoint) {
  if (endpoint == nullptr || value.empty()) return "Redis endpoint destination and value are required";
  std::size_t separator = value.rfind(':');
  std::string host;
  if (value.front() == '[') {
    const auto close = value.find(']');
    if (close == std::string::npos || close + 1 != separator) return "Redis IPv6 endpoint is invalid";
    host = value.substr(1, close - 1);
  } else {
    if (separator == std::string::npos || value.find(':') != separator) return "Redis endpoint is invalid";
    host = value.substr(0, separator);
  }
  try {
    std::size_t consumed = 0;
    const int port = std::stoi(value.substr(separator + 1), &consumed);
    if (host.empty() || consumed != value.size() - separator - 1 || port < 1 || port > 65535) {
      return "Redis endpoint host or port is invalid";
    }
    *endpoint = {.host = std::move(host), .port = static_cast<std::uint16_t>(port)};
    return std::nullopt;
  } catch (...) {
    return "Redis endpoint port is invalid";
  }
}

ValidationError ValidateHiredisPresenceConfig(const HiredisPresenceConfig& config) {
  const bool direct = ValidEndpoint(config.direct);
  const bool sentinel = !config.sentinels.empty();
  if (direct == sentinel) return "exactly one Redis direct or Sentinel mode is required";
  if (sentinel) {
    if (config.sentinel_master_name.empty()) return "Redis Sentinel master name is required";
    for (const auto& endpoint : config.sentinels) {
      if (!ValidEndpoint(endpoint)) return "Redis Sentinel endpoint is invalid";
    }
  }
  if (config.db < 0 || config.db > 15 || config.timeout_ms < 10 || config.timeout_ms > 5000) {
    return "Redis db or timeout is out of range";
  }
  return std::nullopt;
}

HiredisPresenceReader::HiredisPresenceReader(HiredisPresenceConfig config) : config_(std::move(config)) {}
HiredisPresenceReader::~HiredisPresenceReader() { Close(); }

ValidationError HiredisPresenceReader::EnsureConnected() {
  if (context_ != nullptr && context_->err == 0) return std::nullopt;
  Close();
  if (auto error = ValidateHiredisPresenceConfig(config_)) return error;
  RedisEndpoint endpoint = config_.direct;
  if (!config_.sentinels.empty()) {
    if (auto error = DiscoverMaster(config_, &endpoint)) return error;
  }
  context_ = Connect(endpoint, config_.timeout_ms);
  if (context_ == nullptr) return "connect Redis Presence reader";
  if (!Authenticate(context_, config_.password)) {
    Close();
    return "authenticate Redis Presence reader";
  }
  if (config_.db != 0) {
    const auto reply = Command(context_, {"SELECT", std::to_string(config_.db)});
    if (!reply || reply->type != REDIS_REPLY_STATUS) {
      Close();
      return "select Redis Presence database";
    }
  }
  return std::nullopt;
}

ValidationError HiredisPresenceReader::ReadUsers(const std::vector<std::string>& user_ids,
                                                 PresenceReadResult* result) {
  if (result == nullptr) return "Presence read result is required";
  *result = {};
  if (auto error = EnsureConnected()) return error;
  for (const auto& user_id : user_ids) {
    if (user_id.empty()) return "Presence read user is required";
    const std::vector<std::string> command{"HGETALL", "presence:user:" + user_id + ":connections"};
    std::vector<const char*> values{command[0].data(), command[1].data()};
    std::vector<std::size_t> lengths{command[0].size(), command[1].size()};
    if (redisAppendCommandArgv(context_, 2, values.data(), lengths.data()) != REDIS_OK) {
      Close();
      return "pipeline Redis Presence read";
    }
  }
  for (const auto& user_id : user_ids) {
    void* raw = nullptr;
    if (redisGetReply(context_, &raw) != REDIS_OK || raw == nullptr) {
      Close();
      return "receive Redis Presence reply";
    }
    Reply reply(static_cast<redisReply*>(raw), freeReplyObject);
    if (reply->type != REDIS_REPLY_ARRAY || reply->elements % 2 != 0) {
      Close();
      return "Redis Presence reply shape is invalid";
    }
    std::vector<std::pair<std::string, std::string>> fields;
    fields.reserve(reply->elements / 2);
    for (std::size_t index = 0; index < reply->elements; index += 2) {
      if (reply->element[index]->type != REDIS_REPLY_STRING ||
          reply->element[index + 1]->type != REDIS_REPLY_STRING) {
        Close();
        return "Redis Presence hash value is invalid";
      }
      fields.emplace_back(std::string(reply->element[index]->str, reply->element[index]->len),
                          std::string(reply->element[index + 1]->str, reply->element[index + 1]->len));
    }
    PresenceHashParseStats stats;
    auto& connections = result->by_user[user_id];
    if (auto error = ParsePresenceHash(user_id, fields, &connections, &stats)) return error;
    result->parse_stats.observed_records += stats.observed_records;
    result->parse_stats.parsed_records += stats.parsed_records;
    result->parse_stats.malformed_records += stats.malformed_records;
  }
  return std::nullopt;
}

ValidationError HiredisPresenceReader::ReadString(const std::string& key, std::string* value) {
  if (key.empty() || value == nullptr) return "Redis string key and destination are required";
  value->clear();
  if (auto error = EnsureConnected()) return error;
  const auto reply = Command(context_, {"GET", key});
  if (reply && reply->type == REDIS_REPLY_NIL) return "Redis string value is missing";
  if (!reply || reply->type != REDIS_REPLY_STRING) {
    if (!reply || reply->type == REDIS_REPLY_ERROR) Close();
    return "Redis string value is unavailable";
  }
  value->assign(reply->str, reply->len);
  return std::nullopt;
}

ValidationError HiredisPresenceReader::WriteStringWithTTL(const std::string& key,
                                                          const std::string& value,
                                                          std::int64_t ttl_ms) {
  if (key.empty() || value.empty() || ttl_ms < 1) return "Redis string write is invalid";
  if (auto error = EnsureConnected()) return error;
  const auto reply = Command(context_, {"SET", key, value, "PX", std::to_string(ttl_ms)});
  if (!reply || reply->type != REDIS_REPLY_STATUS || std::string_view(reply->str, reply->len) != "OK") {
    if (!reply || reply->type == REDIS_REPLY_ERROR) Close();
    return "Redis string write failed";
  }
  return std::nullopt;
}

void HiredisPresenceReader::Close() {
  if (context_ != nullptr) redisFree(context_);
  context_ = nullptr;
}

}  // namespace dipole::realtime
