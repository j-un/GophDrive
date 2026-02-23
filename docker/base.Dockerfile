ARG NODE_VERSION=22
FROM node:${NODE_VERSION}-bookworm

ARG GO_VERSION=1.26.0

# Install basic utilities
RUN apt-get update && apt-get install -y \
    curl \
    unzip \
    zip \
    jq \
    && rm -rf /var/lib/apt/lists/*

# Install Go
RUN curl -L "https://go.dev/dl/go${GO_VERSION}.linux-$(dpkg --print-architecture).tar.gz" -o go.tar.gz \
    && tar -C /usr/local -xzf go.tar.gz \
    && rm go.tar.gz

ENV PATH="/usr/local/go/bin:${PATH}"
ENV GOPATH="/go"
ENV PATH="${GOPATH}/bin:${PATH}"

# Install AWS CLI v2
RUN ARCH=$(dpkg --print-architecture) && \
    case "${ARCH}" in \
    amd64) AWS_ARCH="x86_64" ;; \
    arm64) AWS_ARCH="aarch64" ;; \
    *) echo "Unsupported architecture: ${ARCH}"; exit 1 ;; \
    esac && \
    curl "https://awscli.amazonaws.com/awscli-exe-linux-${AWS_ARCH}.zip" -o "awscliv2.zip" \
    && unzip -q awscliv2.zip \
    && ./aws/install \
    && rm -rf aws awscliv2.zip

# Install CDK and cdklocal globally
RUN npm install -g aws-cdk aws-cdk-local typescript ts-node

WORKDIR /workspace

CMD ["/bin/bash"]
