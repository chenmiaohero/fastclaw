# Custom OpenClaw image with pre-installed tools
# Based on 1panel/openclaw which is already stable in production

FROM 1panel/openclaw:latest

USER root

# Install uv, Python 3.12, and clawhub in a single layer
ENV UV_PYTHON_INSTALL_DIR=/usr/local/share/uv/python
RUN curl -LsSf https://astral.sh/uv/install.sh | env UV_INSTALL_DIR=/usr/local/bin sh && \
    uv python install 3.12 && \
    npm install -g clawhub

# Add more plugins here as needed:
# RUN cd /tmp && \
#     npm pack @some-scope/another-plugin && \
#     mkdir -p /app/extensions/another-plugin && \
#     tar -xzf *.tgz -C /app/extensions/another-plugin --strip-components=1 && \
#     cd /app/extensions/another-plugin && npm install --omit=dev && \
#     rm -f /tmp/*.tgz && \
#     chown -R node:node /app/extensions/another-plugin

USER node

# clawhub installs skills to PVC-backed directory
ENV CLAWHUB_WORKDIR="/home/node/.openclaw"
