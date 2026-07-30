FROM node:20-alpine as frontend-builder

WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ .
RUN npm run build

FROM golang:1.23-alpine as go-builder

WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -trimpath -o ai-gateway .

FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata curl
WORKDIR /app
COPY --from=go-builder /app/ai-gateway .
RUN mkdir -p /app/data
EXPOSE 3301
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
  CMD wget --no-verbose -O /dev/null http://localhost:3301/health || exit 1
CMD ["./ai-gateway"]