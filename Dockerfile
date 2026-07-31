FROM alpine:3.23

ARG GOAWAY_VERSION=""
ARG DNS_PORT=53
ARG WEBSITE_PORT=8080
ARG PROXY_PORT=5353

ENV DNS_PORT=${DNS_PORT} WEBSITE_PORT=${WEBSITE_PORT} PROXY_PORT=${PROXY_PORT}

COPY installer.sh ./

RUN apk update && apk upgrade && \
    apk add --no-cache curl jq bash ca-certificates libcap && \
    adduser -D -u 1000 goaway && \
    mkdir -p /app/config /app/data && \
    ./installer.sh $GOAWAY_VERSION && \
    mv /root/.local/bin/goaway /app/goaway && \
    rm -rf /var/cache/apk/* /tmp/* /var/tmp/* /root/.cache /root/.local installer.sh

WORKDIR /app

COPY updater.sh ./

# Bind 53/67/547 and send ICMPv6 RAs without running as root.
# Note: HA VIP mode shells out to `ip addr add` and still needs root.
RUN setcap 'cap_net_bind_service=+ep cap_net_raw=+ep' /app/goaway && \
    chown -R goaway:goaway /app
USER goaway

EXPOSE ${DNS_PORT}/tcp ${DNS_PORT}/udp ${PROXY_PORT}/tcp ${PROXY_PORT}/udp ${WEBSITE_PORT}/tcp 67/udp 547/udp

ENTRYPOINT [ "./goaway" ]