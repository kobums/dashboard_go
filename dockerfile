FROM --platform=linux/amd64 golang:1.26-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o main .

# Production stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/main .

# SPA 빌드 산출물 (Makefile 의 web 타겟이 ../dashboard_web/dist 를 복사해 둔다)
COPY dist ./dist

# 시크릿 없는 설정만 이미지에 포함. 실제 값은 서버 /data/dashboard/.env 환경변수로 주입.
COPY .env.yml.docker .env.yml

# Creating webdata directory in the CURRENT working directory
RUN mkdir -p webdata


ENV APP_MODE=production

CMD ["./main"]