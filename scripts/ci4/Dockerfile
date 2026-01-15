

FROM 192.168.10.67/base/alpine:latest

# 设置上海时区
RUN apk add --no-cache tzdata && \
cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
echo "Asia/Shanghai" > /etc/timezone

WORKDIR /app
COPY egame-logic03.bin /app
COPY conf/*.yaml /app/conf/

EXPOSE 8889

# 确保配置文件存在后，使用正确的配置文件路径
ENTRYPOINT ["/app/egame-logic03.bin","-f=/app/conf/config_local.yaml"]