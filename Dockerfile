# ---------- 1. frontend ----------
FROM node:22-alpine AS web
WORKDIR /web
COPY frontend/package*.json ./
RUN npm ci --no-audit --no-fund || npm install --no-audit --no-fund
COPY frontend/ ./
RUN npm run build

# ---------- 2. backend ----------
FROM golang:1.24-alpine AS api
WORKDIR /src
COPY backend/go.mod backend/go.sum* ./
RUN go mod download || true
COPY backend/ ./
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/sentinel ./cmd/server

# ---------- 3. runtime: distroless, non-root, ~35 MB ----------
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=api /out/sentinel /app/sentinel
COPY --from=web /web/dist /app/web
ENV HTTP_ADDR=:8080 STATIC_DIR=/app/web
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/sentinel"]
