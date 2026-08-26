FROM alpine:3.22

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY dist/dipole-server /app/dipole-server
COPY dist/dipole-gateway /app/dipole-gateway
COPY dist/dipole-message /app/dipole-message
COPY dist/dipole-migrate /app/dipole-migrate
COPY dist/dipole-cassandra-projector /app/dipole-cassandra-projector
COPY dist/dipole-cassandra-backfill /app/dipole-cassandra-backfill

EXPOSE 8080

ENTRYPOINT ["/app/dipole-server"]
