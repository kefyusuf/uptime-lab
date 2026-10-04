FROM node:24.21.0-bookworm-slim@sha256:0e0ff40c39bc087845bfb27465a0df4ea419520094bc35842ff83dd8cbe6f9b6 AS build
WORKDIR /build
RUN npm install --global npm@11.19.1
COPY apps/web/package.json apps/web/package-lock.json apps/web/.npmrc ./
RUN npm ci
COPY apps/web/ ./
RUN npm run build

FROM node:24.21.0-bookworm-slim@sha256:0e0ff40c39bc087845bfb27465a0df4ea419520094bc35842ff83dd8cbe6f9b6 AS runtime
RUN groupadd --gid 10001 uptime \
    && useradd --uid 10001 --gid uptime --no-create-home --shell /usr/sbin/nologin uptime
WORKDIR /app
COPY --from=build --chown=10001:10001 /build/dist ./dist
COPY --from=build --chown=10001:10001 /build/dist-server ./dist-server
COPY --from=build --chown=10001:10001 /build/package.json ./package.json
USER 10001:10001
EXPOSE 8080
CMD ["node", "dist-server/main.js"]
