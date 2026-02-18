# ビルドステージ
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o winget-src .

# 実行ステージ
FROM alpine:3.19
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/winget-src .
EXPOSE 8080
ENV PORT=8080
CMD ["./winget-src"]
