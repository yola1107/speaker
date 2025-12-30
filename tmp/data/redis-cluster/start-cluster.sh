#!/bin/bash
set -e

echo "=== Redis Cluster 单机部署 (3主3从) ==="

# 创建数据目录
for port in {7000..7005}; do
  mkdir -p "data/$port"
  chmod 755 "data/$port"
done

# 启动容器
echo "启动 Redis 实例..."
docker-compose up -d

# 等待 Redis 启动
echo "等待 Redis 启动..."
sleep 5

# 创建集群
echo "创建 Redis Cluster..."
docker run -it --rm --network redis-cluster-net redis:7-alpine \
  redis-cli --cluster create \
  redis-7000:7000 redis-7001:7001 redis-7002:7002 \
  redis-7003:7003 redis-7004:7004 redis-7005:7005 \
  --cluster-replicas 1 -a "A12345!"

echo "=== Redis Cluster 部署完成 ==="
echo "节点: 7000~7005"
echo "集群访问: redis://127.0.0.1:7000~7005"
