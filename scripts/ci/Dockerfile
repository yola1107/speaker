FROM alpine:latest

# 日志级别：DEBUG:0 INFO:1 WARN:2 ERROR:3 FATAL:4
ENV LOG_V 1
# 是否标准输出：true、false
ENV LOG_STDOUT true
# 日志文件目录，如果配置会输出日志到文件，否则不输出日志文件 需要输出文件请run的时候-e
# ENV LOG_DIR ./log/tasklog
# grpc服务，tcp://0.0.0.0:9000/?timeout=1s&idle_timeout=60s
# ENV GRPC tcp://0.0.0.0:9000
# http服务，tcp://0.0.0.0:8000/?timeout=1s
# ENV HTTP tcp://0.0.0.0:8000
# 服务名称
ENV APP_ID okey
# 注册中心节点，127.0.0.1:7171,127.0.0.2:7171
ENV DISCOVERY_NODES 172.13.12.42:7000
# 部署地区，
ENV ZONE singapore001
# 部署环境
ENV DEPLOY_ENV dev
# 注册到注册中心的grpc服务端口
# ENV DISCOVERY_GRPC_ADDR 172.13.12.2:8000
# 链路中心地址 参数为是否关闭采样
# ENV TRACE jaeger+udp://172.13.0.120:6831?disable_sample=true

# Apollo客户端配置
# Apollo app id
ENV APOLLO_APP_ID  ckokey
# 集群，默认default
ENV APOLLO_CLUSTER default
# 配置缓存目录
#ENV APOLLO_CACHE_DIR ./tmp
# 配置中心地址
ENV APOLLO_META_ADDR 172.13.0.79:8080
# 配置命名空间/文件，以","分开
ENV APOLLO_NAMESPACES application.txt,http.txt,websocket.txt,grpc.txt,cardsCfg.txt,cardsDnCfg.txt,baseconfig.txt,\
gameconfig_1.txt,gameconfig_2.txt,gameconfig_3.txt,gameconfig_4.txt,\
gameconfig_11.txt,gameconfig_12.txt,gameconfig_13.txt,gameconfig_14.txt,\
gameconfig_21.txt,gameconfig_22.txt,gameconfig_23.txt,gameconfig_24.txt,\
gameconfig_31.txt,gameconfig_32.txt,gameconfig_33.txt,gameconfig_34.txt,\
gameconfig_101.txt,gameconfig_201.txt
#ENV CONFCENTER OFF

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g' /etc/apk/repositories
RUN apk add --no-cache tzdata
RUN cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
&& echo 'Asia/Shanghai' >/etc/timezone
RUN mkdir -p /opt/service/bin
RUN mkdir -p /opt/service/configs

WORKDIR /opt/service/bin

COPY /cmd/cmd /opt/service/bin/
COPY configs /opt/service/configs

#CMD ["./cmd", "-conf", "../configs"]
CMD ["./cmd"]

