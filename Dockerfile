# -----------------------------------------------------------------------------
# XVynora IPTV / Threadfin
# -----------------------------------------------------------------------------

FROM golang:1.23-bullseye AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 \
    GOOS=linux \
    go build \
    -mod=mod \
    -trimpath \
    -ldflags="-s -w" \
    -o threadfin .

# -----------------------------------------------------------------------------
# Runtime
# -----------------------------------------------------------------------------

FROM ubuntu:24.04

ARG THREADFIN_PORT=34400

ENV THREADFIN_BIN=/home/threadfin/bin \
    THREADFIN_CONF=/home/threadfin/conf \
    THREADFIN_HOME=/home/threadfin \
    THREADFIN_TEMP=/tmp/threadfin \
    THREADFIN_CACHE=/home/threadfin/cache \
    THREADFIN_UID=31337 \
    THREADFIN_GID=31337 \
    THREADFIN_USER=threadfin \
    THREADFIN_BRANCH=main \
    THREADFIN_DEBUG=0 \
    THREADFIN_PORT=34400 \
    THREADFIN_BIND_IP_ADDRESS=0.0.0.0 \
    PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/home/threadfin/bin \
    DEBIAN_FRONTEND=noninteractive

WORKDIR $THREADFIN_HOME

ARG TARGETARCH
ARG OS_VERSION=ubuntu
ARG OS_CODENAME=noble

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        ffmpeg \
        vlc \
        tzdata \
        gnupg \
        apt-transport-https && \
    mkdir -p \
        $THREADFIN_BIN \
        $THREADFIN_CONF \
        $THREADFIN_TEMP \
        $THREADFIN_CACHE && \
    chmod a+rwX \
        $THREADFIN_CONF \
        $THREADFIN_TEMP \
        $THREADFIN_CACHE && \
    sed -i 's/geteuid/getppid/' /usr/bin/vlc && \
    curl -fsSL https://repo.jellyfin.org/master/ubuntu/jellyfin_team.gpg.key \
        | gpg --dearmor -o /etc/apt/trusted.gpg.d/ubuntu-jellyfin.gpg && \
    if [ "${TARGETARCH}" = "arm" ]; then \
        echo "deb [arch=armhf] https://repo.jellyfin.org/master/${OS_VERSION} ${OS_CODENAME} main" \
        > /etc/apt/sources.list.d/jellyfin.list; \
    else \
        echo "deb [arch=${TARGETARCH}] https://repo.jellyfin.org/master/${OS_VERSION} ${OS_CODENAME} main" \
        > /etc/apt/sources.list.d/jellyfin.list; \
    fi && \
    apt-get update && \
    apt-get install -y --no-install-recommends --no-install-suggests \
        jellyfin-ffmpeg7 && \
    apt-get remove -y gnupg apt-transport-https && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/threadfin $THREADFIN_BIN/threadfin

RUN chmod +rx $THREADFIN_BIN/threadfin

VOLUME $THREADFIN_CONF
VOLUME $THREADFIN_TEMP

EXPOSE 34400

ENTRYPOINT ["sh", "-c", "${THREADFIN_BIN}/threadfin -port=${THREADFIN_PORT} -bind=${THREADFIN_BIND_IP_ADDRESS} -config=${THREADFIN_CONF} -debug=${THREADFIN_DEBUG}"]
