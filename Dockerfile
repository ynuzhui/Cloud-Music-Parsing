FROM node:20-alpine AS frontend-builder
WORKDIR /app/frontend

COPY package.json package-lock.json ./
RUN npm ci

COPY tsconfig.json tsconfig.node.json vite.config.ts index.html ./
COPY src ./src
RUN npm run build

FROM golang:1.23-alpine AS backend-builder
WORKDIR /app/backend

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/. .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/server ./cmd/server

FROM alpine:3.20
WORKDIR /app/backend

RUN apk add --no-cache su-exec ca-certificates tzdata \
    && addgroup -S -g 1001 app \
    && adduser -S -D -H -u 1001 -G app app \
    && mkdir -p /app/data /app/dist /app/backend

COPY --from=backend-builder /app/server /app/backend/server
COPY --from=frontend-builder /app/frontend/dist /app/dist
COPY docker/entrypoint.sh /app/entrypoint.sh

RUN chmod +x /app/backend/server /app/entrypoint.sh \
    && chown -R app:app /app/backend/server /app/dist

ENV APP_PORT=8099
ENV TZ=Asia/Shanghai
ENV GIN_MODE=release

EXPOSE 8099
ENTRYPOINT ["/app/entrypoint.sh"]
