#!/bin/bash
set -e

echo "=== 启动 Redis Cluster 3主3从 ==="

# 创建数据目录（如果不存在）
for dir in 7000 7001 7002 7003 7004 7005; do
    mkdir -p "data/$dir"
    chmod 755 "data/$dir"
done

# 启动容器
docker-compose up -d

# 等待 Redis 启动
for port in {7000..7005}; do
  echo "等待 redis-$port 启动..."
  until docker exec redis-$port redis-cli ping &>/dev/null; do
    sleep 2
  done
  echo "redis-$port 已就绪"
done

# 创建集群
echo "创建集群..."
docker exec -i redis-7000 redis-cli --cluster create \
  127.0.0.1:7000 \
  127.0.0.1:7001 \
  127.0.0.1:7002 \
  127.0.0.1:7003 \
  127.0.0.1:7004 \
  127.0.0.1:7005 \
  --cluster-replicas 1 \
  -a A12345!

echo "=== Redis Cluster 部署完成 ==="
echo "主节点: 7000,7001,7002"
echo "从节点: 7003,7004,7005"
