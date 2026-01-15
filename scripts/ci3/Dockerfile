FROM 192.168.10.67/base/alpine:latest

RUN apk add --no-cache tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

WORKDIR /app
COPY grpc03.bin /app
COPY conf/*.yaml /app/conf/

EXPOSE 8889

ENTRYPOINT ["/app/grpc03.bin","-f=/app/conf/config_local.yaml"]
