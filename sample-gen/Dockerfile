FROM golang:1.24-alpine AS builder

WORKDIR /src

COPY backend/go.mod backend/go.sum ./backend/
COPY sample-gen/go.mod ./sample-gen/go.mod

WORKDIR /src/sample-gen
RUN go mod download

WORKDIR /src
COPY backend ./backend
COPY sample-gen ./sample-gen

WORKDIR /src/sample-gen
RUN go build -o /out/sample-gen ./main.go

FROM alpine:latest

RUN apk add --no-cache tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone && \
    apk del tzdata

WORKDIR /app

COPY --from=builder /out/sample-gen ./sample-gen

CMD ["./sample-gen", "--daemon"]
