FROM alpine:3.22

ARG DIPOLE_BINARY
ARG DIPOLE_VCS_REVISION=unknown
ARG DIPOLE_BUILD_CREATED=unknown
ARG DIPOLE_BUILD_DIRTY=unknown

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

# Keep the shared Alpine dependency layer reusable across candidate revisions.
LABEL org.opencontainers.image.revision="${DIPOLE_VCS_REVISION}" \
      org.opencontainers.image.created="${DIPOLE_BUILD_CREATED}" \
      io.dipole.source.dirty="${DIPOLE_BUILD_DIRTY}" \
      io.dipole.service.binary="${DIPOLE_BINARY}"

COPY ${DIPOLE_BINARY} /app/service

EXPOSE 8080

ENTRYPOINT ["/app/service"]
