# Build stage for Svelte frontend
FROM node:20-bookworm-slim AS frontend-builder
WORKDIR /app
COPY frontend/package*.json ./
RUN npm install
COPY frontend/ ./
RUN npm run build

# Build stage for Go backend
FROM golang:1.25-bookworm AS backend-builder
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN go build -o main .

# Final stage
FROM debian:bookworm-slim
WORKDIR /app
RUN apt-get update && apt-get install -y hdparm sqlite3 curl procps && rm -rf /var/lib/apt/lists/*
# Create data directory for SQLite
RUN mkdir /data
COPY --from=frontend-builder /app/dist ./dist
COPY --from=backend-builder /app/main .
COPY config.json .

EXPOSE 48070
CMD ["./main"]
