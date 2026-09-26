FROM ghcr.io/pnpm/pnpm:12.4.1 AS pnpm

FROM node:24-bookworm-slim AS frontend
COPY --from=pnpm /opt/pnpm/pnpm /usr/local/bin/pnpm
WORKDIR /app
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY frontend/package.json ./frontend/package.json
RUN pnpm install --frozen-lockfile
COPY frontend ./frontend
COPY backend/bingo/route_segments.json ./backend/bingo/route_segments.json
RUN pnpm build

FROM golang:1.25 AS development
COPY --from=pnpm /opt/pnpm/pnpm /usr/local/bin/pnpm
WORKDIR /app/backend
RUN go install github.com/air-verse/air@v1.63.0
COPY backend/go.* ./
RUN go mod download
WORKDIR /app
COPY . .
COPY --from=frontend /app/backend/static ./backend/static
WORKDIR /app/backend
CMD ["air", "-c", ".air.toml"]

FROM golang:1.25 AS builder
WORKDIR /app/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend ./
COPY --from=frontend /app/backend/static ./static
RUN CGO_ENABLED=0 GOOS=linux go build -o server

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=builder /app/backend/server .
COPY --from=builder --chown=nonroot:nonroot /app/backend/bingo.json /app/data/bingo.json
ENV BINGO_DATA=/app/data/bingo.json
EXPOSE 8080
USER nonroot:nonroot
CMD ["./server"]
