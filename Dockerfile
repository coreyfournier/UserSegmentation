FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /segmentation ./cmd/segmentation

FROM alpine:3.20
RUN apk add --no-cache wget
COPY --from=builder /segmentation /segmentation
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s CMD wget -qO- http://localhost:8080/v1/health || exit 1
ENTRYPOINT ["/segmentation"]
CMD ["-config", "/config/segments.json", "-addr", ":8080"]
