#include <cstdlib>
#include <iostream>
#include <string>

#include "contract_validator.hpp"

namespace {

int failures = 0;

void Check(bool condition, const std::string& message) {
  if (!condition) {
    std::cerr << "FAIL: " << message << '\n';
    ++failures;
  }
}

std::string Testdata(const std::string& name) {
  return std::string(DIPOLE_DELIVERY_TESTDATA_DIR) + "/" + name;
}

}  // namespace

int main() {
  dipole::delivery::v1::DeliveryEnvelope envelope;
  Check(!dipole::realtime::ParseEnvelopeJsonFile(Testdata("delivery_batch.v1.json"), &envelope),
        "parse envelope golden");
  Check(!dipole::realtime::ValidateEnvelope(envelope), "validate envelope golden");

  auto invalid_envelope = envelope;
  invalid_envelope.set_source_offset(-1);
  Check(dipole::realtime::ValidateEnvelope(invalid_envelope).has_value(), "reject negative offset");
  invalid_envelope = envelope;
  invalid_envelope.mutable_items(0)->set_mode(
      static_cast<dipole::delivery::v1::DeliveryMode>(99));
  Check(dipole::realtime::ValidateEnvelope(invalid_envelope).has_value(), "reject unknown mode");
  invalid_envelope = envelope;
  *invalid_envelope.add_items() = invalid_envelope.items(0);
  Check(dipole::realtime::ValidateEnvelope(invalid_envelope).has_value(), "reject duplicate delivery id");

  dipole::delivery::v1::NodeDeliveryBatch node_batch;
  Check(!dipole::realtime::ParseNodeBatchJsonFile(Testdata("node_delivery_batch.v1.json"), &node_batch),
        "parse node batch golden");
  Check(!dipole::realtime::ValidateNodeBatch(node_batch), "validate node batch golden");
  auto invalid_node_batch = node_batch;
  invalid_node_batch.mutable_items(0)->add_connection_ids(
      invalid_node_batch.items(0).connection_ids(0));
  Check(dipole::realtime::ValidateNodeBatch(invalid_node_batch).has_value(),
        "reject duplicate connection id");

  dipole::delivery::v1::DeliveryAck ack;
  Check(!dipole::realtime::ParseAckJsonFile(Testdata("delivery_ack.v1.json"), &ack),
        "parse ack golden");
  Check(!dipole::realtime::ValidateAck(ack), "validate ack golden");
  auto invalid_ack = ack;
  invalid_ack.mutable_results(0)->set_retry_after_ms(0);
  Check(dipole::realtime::ValidateAck(invalid_ack).has_value(), "reject missing retry hint");

  return failures == 0 ? EXIT_SUCCESS : EXIT_FAILURE;
}
