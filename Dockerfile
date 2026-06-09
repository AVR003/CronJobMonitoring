FROM node:20-alpine AS ui
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.22-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=ui /app/frontend/dist ./frontend/dist
RUN go build -o monitoring-svc .

FROM alpine:3.19
RUN apk add --no-cache fping ca-certificates
COPY --from=builder /app/monitoring-svc /usr/local/bin/
EXPOSE 8080
ENTRYPOINT ["monitoring-svc"]
