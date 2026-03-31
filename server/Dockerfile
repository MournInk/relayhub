FROM golang:1.23-bookworm AS deps
WORKDIR /workspace/server
COPY server/go.mod server/go.sum* ./
RUN go mod download

FROM deps AS test
WORKDIR /workspace
COPY server ./server
COPY config ./config
WORKDIR /workspace/server
RUN go test ./...

FROM deps AS build
WORKDIR /workspace
COPY server ./server
COPY config ./config
WORKDIR /workspace/server
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /relayhub ./cmd/relayhub

FROM debian:bookworm-slim AS runtime
WORKDIR /app
COPY --from=build /relayhub /app/relayhub
COPY config /app/config
EXPOSE 8080
ENV RELAYHUB_CONFIG=/app/config/relayhub.json
CMD ["./relayhub"]
