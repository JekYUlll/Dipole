FROM alpine:3.22

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY dist/dipole-server /app/dipole-server
COPY dist/dipole-gateway /app/dipole-gateway
COPY dist/dipole-message /app/dipole-message
COPY dist/dipole-migrate /app/dipole-migrate
COPY dist/dipole-cassandra-projector /app/dipole-cassandra-projector
COPY dist/dipole-search-indexer /app/dipole-search-indexer
COPY dist/dipole-search /app/dipole-search
COPY dist/dipole-sync /app/dipole-sync
COPY dist/dipole-sync-replay /app/dipole-sync-replay
COPY dist/dipole-sync-reconcile /app/dipole-sync-reconcile
COPY dist/dipole-sync-baseline /app/dipole-sync-baseline
COPY dist/dipole-search-backfill /app/dipole-search-backfill
COPY dist/dipole-search-reconcile /app/dipole-search-reconcile
COPY dist/dipole-search-alias /app/dipole-search-alias
COPY dist/dipole-search-archive /app/dipole-search-archive
COPY dist/dipole-search-outbox-cleanup /app/dipole-search-outbox-cleanup
COPY dist/dipole-cassandra-backfill /app/dipole-cassandra-backfill
COPY dist/dipole-cassandra-reconcile /app/dipole-cassandra-reconcile

EXPOSE 8080

ENTRYPOINT ["/app/dipole-server"]
