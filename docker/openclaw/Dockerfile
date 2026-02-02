# Custom OpenClaw image with pre-installed plugins
# Based on 1panel/openclaw which is already stable in production

FROM 1panel/openclaw:latest

USER root

# Install feishu plugin to /app/extensions/
# Using npm pack to download and extract with correct directory structure
RUN cd /tmp && \
    npm pack @m1heng-clawd/feishu && \
    mkdir -p /app/extensions/feishu && \
    tar -xzf m1heng-clawd-feishu-*.tgz -C /app/extensions/feishu --strip-components=1 && \
    cd /app/extensions/feishu && \
    npm install --omit=dev && \
    rm -f /tmp/m1heng-clawd-feishu-*.tgz && \
    chown -R node:node /app/extensions/feishu

# Add more plugins here as needed:
# RUN cd /tmp && \
#     npm pack @some-scope/another-plugin && \
#     mkdir -p /app/extensions/another-plugin && \
#     tar -xzf *.tgz -C /app/extensions/another-plugin --strip-components=1 && \
#     cd /app/extensions/another-plugin && npm install --omit=dev && \
#     rm -f /tmp/*.tgz && \
#     chown -R node:node /app/extensions/another-plugin

USER node
