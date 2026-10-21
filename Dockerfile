FROM golang:1.24-alpine AS builder
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/taskmanager ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=builder /out/taskmanager /app/taskmanager
COPY migrations /app/migrations
COPY openapi.yaml /app/openapi.yaml
EXPOSE 8080
ENTRYPOINT ["/app/taskmanager"]
