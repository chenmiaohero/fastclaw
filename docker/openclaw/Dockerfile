# Custom OpenClaw image with pre-installed tools
# Based on 1panel/openclaw which is already stable in production

FROM 1panel/openclaw:latest

USER root

# Install uv, Python 3.12, and clawhub in a single layer
ENV UV_PYTHON_INSTALL_DIR=/usr/local/share/uv/python
RUN curl -LsSf https://astral.sh/uv/install.sh | env UV_INSTALL_DIR=/usr/local/bin sh && \
    uv python install 3.12 && \
    ln -sf /usr/bin/python3 /usr/local/bin/python && \
    npm install -g clawhub

# Wrapper script: prefer PVC-installed openclaw over built-in version
# Users can upgrade via "npm i -g openclaw" inside the container,
# the new version is installed to PVC-backed npm prefix and persists across restarts.
RUN printf '#!/bin/sh\n\
LOCAL_BIN="$HOME/.openclaw/npm/bin/openclaw"\n\
if [ -x "$LOCAL_BIN" ]; then\n\
  exec "$LOCAL_BIN" "$@"\n\
fi\n\
exec node /app/openclaw.mjs "$@"\n' > /usr/local/bin/openclaw && chmod +x /usr/local/bin/openclaw

USER node

# PVC-backed working directory for skills, plugins, and user-installed packages
ENV CLAWHUB_WORKDIR="/home/node/.openclaw"
ENV NPM_CONFIG_PREFIX="/home/node/.openclaw/npm"
ENV PATH="/home/node/.openclaw/npm/bin:$PATH"
