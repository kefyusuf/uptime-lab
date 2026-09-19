FROM alpine:3.24.2

RUN addgroup -S -g 10001 uptime \
    && adduser -S -D -H -u 10001 -G uptime uptime \
    && mkdir -p /run/uptime-lab \
    && chown 10001:10001 /run/uptime-lab

COPY --chown=10001:10001 entrypoint.sh /usr/local/bin/placeholder-entrypoint
RUN chmod 0555 /usr/local/bin/placeholder-entrypoint

USER 10001:10001
ENTRYPOINT ["/usr/local/bin/placeholder-entrypoint"]
