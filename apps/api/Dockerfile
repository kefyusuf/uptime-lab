FROM golang:1.27.1-alpine3.24 AS builder

WORKDIR /src/apps/api

COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download

COPY apps/api/ ./

RUN mkdir -p /out \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -buildvcs=false -o /out/uptime-lab-api ./cmd/api \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -buildvcs=false -o /out/uptime-lab-migrate ./cmd/migrate

FROM alpine:3.24.2

RUN apk add --no-cache ca-certificates \
    && addgroup -S -g 10001 uptime \
    && adduser -S -D -H -u 10001 -G uptime uptime

COPY --from=builder /out/uptime-lab-api /usr/local/bin/uptime-lab-api
COPY --from=builder /out/uptime-lab-migrate /usr/local/bin/uptime-lab-migrate
RUN chmod 0555 /usr/local/bin/uptime-lab-api /usr/local/bin/uptime-lab-migrate

USER 10001:10001
ENTRYPOINT ["/usr/local/bin/uptime-lab-api"]
