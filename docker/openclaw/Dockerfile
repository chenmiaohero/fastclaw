# Custom OpenClaw image with pre-installed plugins
# Based on 1panel/openclaw which is already stable in production

FROM 1panel/openclaw:latest

USER root

# Add more plugins here as needed:
# RUN cd /tmp && \
#     npm pack @some-scope/another-plugin && \
#     mkdir -p /app/extensions/another-plugin && \
#     tar -xzf *.tgz -C /app/extensions/another-plugin --strip-components=1 && \
#     cd /app/extensions/another-plugin && npm install --omit=dev && \
#     rm -f /tmp/*.tgz && \
#     chown -R node:node /app/extensions/another-plugin

USER node
