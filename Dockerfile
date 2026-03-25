# Build stage: install all deps, compile TypeScript
FROM node:20-alpine AS builder
RUN apk add --no-cache python3 make g++
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci
COPY tsup.config.ts tsconfig.json ./
COPY src/ src/
RUN npm run build

# Runtime stage: production deps + compiled output
FROM node:20-alpine
RUN apk add --no-cache python3 make g++ tzdata
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci --omit=dev && apk del python3 make g++
COPY --from=builder /app/dist/ dist/
EXPOSE 8080
ENTRYPOINT ["node", "dist/server/index.js"]
