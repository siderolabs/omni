FROM alpine:3.19.1

ARG TARGETARCH

# Update apk repositories and install necessary packages
RUN apk update && apk add --no-cache \
    coreutils \
    util-linux \
    gnupg \
    curl \
    jq

# Create a dedicated user for initialization processes with user ID 1000
RUN adduser -D -u 1000 omni-init

# Switch to the user 'omni-init'
USER omni-init

# Create a directory for the omni-init user to download kubectl
RUN mkdir /home/omni-init/bin
WORKDIR /home/omni-init/bin

# Install kubectl based on the architecture
RUN if [ "$TARGETARCH" = "amd64" ]; then \
        KUBECTL_ARCH="amd64"; \
    elif [ "$TARGETARCH" = "arm64" ]; then \
        KUBECTL_ARCH="arm64"; \
    else \
        echo "Unsupported architecture: $TARGETARCH"; \
        exit 1; \
    fi && \
    curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/${KUBECTL_ARCH}/kubectl" \
    && chmod +x kubectl

# Append /home/omni-init/bin to PATH so kubectl can be executed directly
ENV PATH="/home/omni-init/bin:${PATH}"
