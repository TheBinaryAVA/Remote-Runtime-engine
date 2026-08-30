# ========================================================
# GDG VIT Chennai - Speed-Coding Isolated Execution Runner
# Base container image for containerized sandbox backend
# ========================================================
FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive
ENV PYTHONUNBUFFERED=1
ENV PYTHONDONTWRITEBYTECODE=1

# Install core language toolchains (g++ and python3)
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    g++ \
    python3 \
    python3-minimal \
    libstdc++6 \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create unprivileged runner user and sandbox workspace
RUN groupadd -g 1001 speedcode && \
    useradd -u 1001 -g speedcode -m -s /bin/bash speedcode && \
    mkdir -p /workspace && \
    chown -R speedcode:speedcode /workspace

WORKDIR /workspace
USER speedcode

CMD ["/bin/bash"]
