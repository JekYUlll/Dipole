#include <cstdlib>
#include <iostream>
#include <map>
#include <memory>
#include <string>

#include <google/protobuf/util/time_util.h>
#include <grpcpp/grpcpp.h>

#include "node_delivery_transport.hpp"

namespace {

int failures = 0;

void Check(bool condition, const std::string& message) {
  if (!condition) {
    std::cerr << "FAIL: " << message << '\n';
    ++failures;
  }
}

dipole::delivery::v1::NodeDeliveryBatch Batch() {
  dipole::delivery::v1::NodeDeliveryBatch batch;
  batch.set_contract_version("v1");
  batch.set_batch_id("NB1");
  batch.set_target_node_id("gateway-1");
  batch.set_source_event_id("E1");
  batch.set_request_id("R1");
  batch.set_trace_id("T1");
  *batch.mutable_created_at() = google::protobuf::util::TimeUtil::SecondsToTimestamp(1);
  auto* item = batch.add_items();
  item->set_delivery_id("D1");
  item->set_recipient_user_id("U1");
  item->add_connection_ids("C1");
  item->set_event_type("chat.message");
  item->set_payload_json(R"({"message_id":"M1"})");
  item->set_ordering_key("direct:U1:U2");
  item->set_mode(dipole::delivery::v1::DELIVERY_MODE_FULL_EVENT);
  return batch;
}

class ObservationService final : public dipole::delivery::v1::NodeDeliveryService::Service {
 public:
  grpc::Status ObserveNodeBatch(
      grpc::ServerContext* context,
      const dipole::delivery::v1::NodeDeliveryBatch* batch,
      dipole::delivery::v1::NodeDeliveryObservation* observation) override {
    const auto& metadata = context->client_metadata();
    const auto caller = metadata.find("x-dipole-caller-service");
    const auto token = metadata.find("x-dipole-service-token");
    const auto request = metadata.find("x-request-id");
    const auto trace = metadata.find("x-trace-id");
    metadata_valid = caller != metadata.end() && caller->second == "dipole-realtime" &&
                     token != metadata.end() && token->second == "test-secret" &&
                     request != metadata.end() && request->second == "R1" &&
                     trace != metadata.end() && trace->second == "T1";
    observation->set_contract_version("v1");
    observation->set_batch_id(batch->batch_id());
    observation->set_target_node_id(batch->target_node_id());
    observation->set_status(status);
    *observation->mutable_observed_at() = google::protobuf::util::TimeUtil::GetCurrentTime();
    if (status == dipole::delivery::v1::NODE_OBSERVATION_STATUS_OBSERVED) {
      observation->set_observed_items(1);
      observation->set_observed_connections(1);
      observation->set_duplicate(duplicate);
    } else if (status == dipole::delivery::v1::NODE_OBSERVATION_STATUS_BACKPRESSURED) {
      observation->set_error_code(dipole::delivery::v1::DELIVERY_ERROR_CODE_QUEUE_FULL);
      auto* pressure = observation->mutable_pressure();
      pressure->set_depth(4);
      pressure->set_capacity(4);
      pressure->set_retry_after_ms(25);
    }
    return grpc::Status::OK;
  }

  grpc::Status DeliverNodeBatch(
      grpc::ServerContext* context,
      const dipole::delivery::v1::NodeDeliveryBatch* batch,
      dipole::delivery::v1::DeliveryAck* acknowledgement) override {
    const auto& metadata = context->client_metadata();
    const auto caller = metadata.find("x-dipole-caller-service");
    const auto token = metadata.find("x-dipole-service-token");
    delivery_metadata_valid = caller != metadata.end() && caller->second == "dipole-realtime" &&
                              token != metadata.end() && token->second == "test-secret";
    acknowledgement->set_contract_version("v1");
    acknowledgement->set_batch_id(batch->batch_id());
    acknowledgement->set_status(dipole::delivery::v1::DELIVERY_ACK_STATUS_ACCEPTED);
    *acknowledgement->mutable_acknowledged_at() =
        google::protobuf::util::TimeUtil::GetCurrentTime();
    auto* result = acknowledgement->add_results();
    result->set_delivery_id(batch->items(0).delivery_id());
    result->set_status(dipole::delivery::v1::DELIVERY_RESULT_STATUS_ENQUEUED);
    result->set_accepted_connections(1);
    return grpc::Status::OK;
  }

  dipole::delivery::v1::NodeObservationStatus status =
      dipole::delivery::v1::NODE_OBSERVATION_STATUS_OBSERVED;
  bool duplicate = false;
  bool metadata_valid = false;
  bool delivery_metadata_valid = false;
};

struct TestServer {
  ObservationService service;
  std::unique_ptr<grpc::Server> server;
  int port = 0;

  TestServer() {
    grpc::ServerBuilder builder;
    builder.AddListeningPort("127.0.0.1:0", grpc::InsecureServerCredentials(), &port);
    builder.RegisterService(&service);
    server = builder.BuildAndStart();
  }

  ~TestServer() {
    if (server != nullptr) server->Shutdown();
  }
};

void TestTargetParsingAndConfigValidation() {
  std::map<std::string, std::string> targets;
  Check(!dipole::realtime::ParseNodeTargets(
            " gateway-1=127.0.0.1:9095,gateway-2=[::1]:9095 ", &targets) &&
            targets.size() == 2 && targets.at("gateway-1") == "127.0.0.1:9095",
        "node targets parse deterministically");
  Check(dipole::realtime::ParseNodeTargets("gateway-1=127.0.0.1:1,gateway-1=127.0.0.1:2",
                                           &targets)
            .has_value(),
        "duplicate node target is rejected");
  Check(dipole::realtime::ValidateGrpcNodeTransportConfig(
            {.targets = {{"gateway-1", "gateway.internal:9095"}},
             .shared_secret = "secret",
             .timeout_ms = 100,
             .tls_enabled = false,
             .tls_ca_file = {},
             .tls_cert_file = {},
             .tls_key_file = {},
             .tls_server_name = {}})
            .has_value(),
        "plaintext remote target is rejected");
  Check(dipole::realtime::ValidateGrpcNodeTransportConfig(
            {.targets = {{"gateway-1", "127.example:9095"}},
             .shared_secret = "secret",
             .timeout_ms = 100,
             .tls_enabled = false,
             .tls_ca_file = {},
             .tls_cert_file = {},
             .tls_key_file = {},
             .tls_server_name = {}})
            .has_value(),
        "DNS name with loopback-looking prefix is rejected");
}

void TestGrpcTransportObservesWithAuthenticatedMetadata() {
  TestServer server;
  Check(server.server != nullptr && server.port > 0, "test gRPC server starts");
  dipole::realtime::GrpcNodeTransportConfig config{
      .targets = {{"gateway-1", "127.0.0.1:" + std::to_string(server.port)}},
      .shared_secret = "test-secret",
      .timeout_ms = 500,
      .tls_enabled = false,
      .tls_ca_file = {},
      .tls_cert_file = {},
      .tls_key_file = {},
      .tls_server_name = {},
  };
  std::unique_ptr<dipole::realtime::GrpcNodeBatchTransport> transport;
  Check(!dipole::realtime::GrpcNodeBatchTransport::Create(config, &transport) &&
            transport != nullptr,
        "gRPC node transport is created");
  dipole::realtime::NodeTransportStats stats;
  Check(!transport->Observe({Batch()}, &stats), "node batch is observed");
  Check(stats.requested == 1 && stats.observed == 1 && stats.duplicate == 0 &&
            server.service.metadata_valid,
        "observation records response and authenticated correlation metadata");

  server.service.duplicate = true;
  Check(!transport->Observe({Batch()}, &stats) && stats.duplicate == 1,
        "duplicate observation remains successful");

  server.service.status = dipole::delivery::v1::NODE_OBSERVATION_STATUS_BACKPRESSURED;
  Check(transport->Observe({Batch()}, &stats).has_value() && stats.backpressured == 1,
        "backpressure remains retryable transport failure");

  std::vector<dipole::delivery::v1::DeliveryAck> acknowledgements;
  Check(!transport->Deliver({Batch()}, &acknowledgements) && acknowledgements.size() == 1 &&
            acknowledgements[0].status() ==
                dipole::delivery::v1::DELIVERY_ACK_STATUS_ACCEPTED &&
            server.service.delivery_metadata_valid,
        "primary delivery validates authenticated acknowledgement without changing shadow mode");
}

}  // namespace

int main() {
  TestTargetParsingAndConfigValidation();
  TestGrpcTransportObservesWithAuthenticatedMetadata();
  return failures == 0 ? EXIT_SUCCESS : EXIT_FAILURE;
}
