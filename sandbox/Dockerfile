FROM ubuntu:24.04

ARG SANDBOX_UID=10001
ARG SANDBOX_GID=10001

ENV DEBIAN_FRONTEND=noninteractive
ENV HOME=/home/docode

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        bash \
        build-essential \
        ca-certificates \
        curl \
        git \
        golang-go \
        jq \
        make \
        nodejs \
        npm \
        openssh-client \
        python3 \
        python3-pip \
        python3-venv \
        python-is-python3 \
        ripgrep \
        unzip \
        zip \
    && groupadd --gid "${SANDBOX_GID}" docode \
    && useradd --uid "${SANDBOX_UID}" --gid "${SANDBOX_GID}" --create-home --shell /bin/bash docode \
    && mkdir -p /workspace \
    && chown -R "${SANDBOX_UID}:${SANDBOX_GID}" /workspace /home/docode \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace
USER docode

CMD ["sleep", "infinity"]
