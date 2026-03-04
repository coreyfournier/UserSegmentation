FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /segmentation ./cmd/segmentation

FROM scratch
COPY --from=builder /segmentation /segmentation
COPY config/segments.json /config/segments.json
EXPOSE 8080
ENTRYPOINT ["/segmentation", "-config", "/config/segments.json"]
