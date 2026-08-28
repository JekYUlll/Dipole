#ifndef DIPOLE_REALTIME_DELIVERY_SHADOW_EVIDENCE_HPP_
#define DIPOLE_REALTIME_DELIVERY_SHADOW_EVIDENCE_HPP_

#include <ostream>

#include "shadow_runner.hpp"

namespace dipole::realtime {

class JsonLineEvidenceSink final : public ShadowEvidenceSink {
 public:
  explicit JsonLineEvidenceSink(std::ostream* output);
  ValidationError Append(const ShadowEvidence& evidence) override;

 private:
  std::ostream* output_;
};

}  // namespace dipole::realtime

#endif  // DIPOLE_REALTIME_DELIVERY_SHADOW_EVIDENCE_HPP_
