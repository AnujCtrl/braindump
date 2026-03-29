FROM oven/bun:1 AS builder
WORKDIR /app
COPY package.json bun.lockb* ./
RUN bun install --frozen-lockfile
COPY src/ src/
COPY tsconfig.json ./
RUN bun build --compile src/cli/index.ts --outfile braindump

FROM debian:bookworm-slim
COPY --from=builder /app/braindump /usr/local/bin/braindump
ENV BRAINDUMP_HOME=/data
ENV BRAINDUMP_DOCKER=1
EXPOSE 8080
ENTRYPOINT ["braindump", "server"]
