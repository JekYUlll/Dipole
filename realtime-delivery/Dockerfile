FROM ubuntu:24.04 AS builder

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
    && apt-get install --yes --no-install-recommends \
      cmake \
      curl \
      g++ \
      libhiredis-dev \
      libgrpc++-dev \
      librdkafka-dev \
      libprotobuf-dev \
      libssl-dev \
      ninja-build \
      nlohmann-json3-dev \
      pkg-config \
      protobuf-compiler \
      protobuf-compiler-grpc \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src
COPY api/proto /src/api/proto
COPY realtime-delivery /src/realtime-delivery

RUN cmake \
      -S /src/realtime-delivery \
      -B /build \
      -G Ninja \
      -DCMAKE_BUILD_TYPE=Release \
      -DBUILD_TESTING=ON \
    && cmake --build /build --parallel \
    && ctest --test-dir /build --output-on-failure

FROM ubuntu:24.04

ARG DIPOLE_VCS_REVISION=unknown
ARG DIPOLE_BUILD_CREATED=unknown
ARG DIPOLE_VCS_DIRTY=unknown

LABEL org.opencontainers.image.revision="${DIPOLE_VCS_REVISION}" \
      org.opencontainers.image.created="${DIPOLE_BUILD_CREATED}" \
      io.dipole.source.dirty="${DIPOLE_VCS_DIRTY}"

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
    && apt-get install --yes --no-install-recommends \
      ca-certificates \
      libhiredis1.1.0 \
      libgrpc++1.51t64 \
      librdkafka1 \
      libprotobuf32t64 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /build/dipole-realtime-delivery /app/dipole-realtime-delivery
COPY api/proto/dipole/delivery/v1/testdata /app/contracts/delivery/v1

EXPOSE 8092
ENTRYPOINT ["/app/dipole-realtime-delivery"]
CMD ["serve", "/app/contracts/delivery/v1"]
