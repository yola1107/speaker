#!/bin/bash
set -euo pipefail

PASSWORD="A12345!"
REDIS_NODES=(7000 7001 7002 7003 7004 7005)

echo "=== Redis Cluster + Twemproxy + Nginx 启动 ==="

# 创建 Redis 数据目录
for PORT in "${REDIS_NODES[@]}"; do
  mkdir -p "data/$PORT"
  chmod 755 "data/$PORT"
done

# 启动所有容器
docker-compose up -d

# 等待 Redis 节点就绪
for PORT in "${REDIS_NODES[@]}"; do
  echo -n "节点 $PORT ... "
  TIMEOUT=30
  while [ $TIMEOUT -gt 0 ]; do
    if docker-compose exec -T redis-$PORT redis-cli -a "$PASSWORD" ping &>/dev/null; then
      echo "就绪"
      break
    fi
    sleep 1
    TIMEOUT=$((TIMEOUT-1))
  done
  if [ $TIMEOUT -eq 0 ]; then
    echo "ERROR: 节点 $PORT 启动失败"
    exit 1
  fi
done

# 集群幂等创建
CLUSTER_STATE=$(docker-compose exec -T redis-7000 redis-cli -a "$PASSWORD" cluster info | grep cluster_state || true)
if [[ "$CLUSTER_STATE" != *"ok"* ]]; then
  echo "创建 Redis Cluster..."
  NODE_ARGS=""
  for PORT in "${REDIS_NODES[@]}"; do
    NODE_ARGS+="redis-$PORT:$PORT "
  done
  docker run --rm --network redis-cluster-net redis:7-alpine \
    redis-cli --cluster create $NODE_ARGS --cluster-replicas 1 -a "$PASSWORD" --cluster-yes
else
  echo "集群已存在，跳过创建"
fi

echo
echo "=== 部署完成 ==="
echo "客户端连接 Nginx: 127.0.0.1:22121"
echo "内部 Twemproxy 端口: 22121~22124"
echo "Redis Cluster 节点: 7000~7005"
