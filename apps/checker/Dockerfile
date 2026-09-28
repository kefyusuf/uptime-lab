FROM rust:1.98.1-alpine3.24 AS builder
WORKDIR /src/apps/checker
COPY apps/checker/ ./
RUN cargo build --release --locked -p checker

FROM alpine:3.24.2
RUN apk add --no-cache ca-certificates \
    && addgroup -S -g 10001 uptime \
    && adduser -S -D -H -u 10001 -G uptime uptime
COPY --from=builder /src/apps/checker/target/release/checker /usr/local/bin/uptime-lab-checker
RUN chmod 0555 /usr/local/bin/uptime-lab-checker
USER 10001:10001
ENTRYPOINT ["/usr/local/bin/uptime-lab-checker"]
