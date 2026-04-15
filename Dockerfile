FROM golang:1.26-alpine AS builder

WORKDIR /build

ENV GOPROXY=https://goproxy.cn,direct
ENV GONOSUMCHECK=*

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o dipole-server ./cmd/server

FROM alpine:3.22

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /build/dipole-server /app/dipole-server

EXPOSE 8080

ENTRYPOINT ["/app/dipole-server"]
