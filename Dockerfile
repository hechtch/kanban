# syntax=docker/dockerfile:1.7

# ─── Frontend build ──────────────────────────────────────
FROM node:22-alpine AS frontend-build
WORKDIR /src
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npx ng build --configuration production

# ─── Backend build ───────────────────────────────────────
FROM golang:1.25-alpine AS backend-build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# Embed the built frontend so the binary serves both UI + API.
COPY --from=frontend-build /src/dist ./web
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/kanban .

# ─── Runtime ─────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=backend-build /out/kanban /app/kanban
ENV KANBAN_DB_PATH=/data/kanban.db
EXPOSE 8000
USER nonroot
ENTRYPOINT ["/app/kanban", "-addr", ":8000"]
