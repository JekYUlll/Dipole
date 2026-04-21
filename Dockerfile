FROM alpine:3.22

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY dist/dipole-server /app/dipole-server

EXPOSE 8080

ENTRYPOINT ["/app/dipole-server"]
