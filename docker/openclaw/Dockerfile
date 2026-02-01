# Custom OpenClaw image with pre-installed plugins
# Based on 1panel/openclaw which is already stable in production

FROM 1panel/openclaw:latest

# Switch to root for installation
USER root

# Create extensions directory
RUN mkdir -p /home/node/.openclaw/extensions && \
    chown -R node:node /home/node/.openclaw

# Switch to node user for npm operations
USER node

# Set working directory
WORKDIR /home/node

# Pre-install feishu plugin (available for use, not enabled by default)
# Users enable plugins at runtime via clawork API or openclaw config
RUN npm install @m1heng-clawd/feishu --prefix /home/node/.openclaw/extensions/feishu --legacy-peer-deps || true

# Add more plugins here as needed:
# RUN npm install @some-scope/another-plugin --prefix /home/node/.openclaw/extensions/another-plugin || true

# Note: Plugin enablement is handled at runtime by each bot's configuration
# Do NOT pre-configure plugins.json here - let users control it

# Default command (inherit from base image)
