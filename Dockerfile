FROM node:22-alpine AS web-builder

WORKDIR /src/web/admin
COPY web/admin/package*.json ./
RUN npm ci

COPY web/admin ./
RUN npm run build

FROM golang:1.22-alpine AS builder

WORKDIR /src
ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.google.cn

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
COPY --from=web-builder /src/internal/admin/web/dist ./internal/admin/web/dist
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/oldbeggar ./cmd/oldbeggar

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /out/oldbeggar /app/oldbeggar
# ddddocr common.onnx 验证码识别模型（见 README “验证码识别”）。
COPY --from=builder /src/ocr /app/ocr

ENTRYPOINT ["/app/oldbeggar"]
