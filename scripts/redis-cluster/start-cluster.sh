#!/bin/bash
set -euo pipefail

echo "=== Redis Cluster 单机部署 (3主3从) ==="

NODES=(7000 7001 7002 7003 7004 7005)
PASSWORD="A12345!"

echo "=== Redis Cluster 启动 ==="

# 创建数据目录
for PORT in "${NODES[@]}"; do
  mkdir -p "data/$PORT" && chmod 755 "data/$PORT"
done

# 启动 Redis 容器
docker-compose up -d

# 等待所有节点就绪
for PORT in "${NODES[@]}"; do
  echo -n "节点 $PORT ... "
  TIMEOUT=30
  until docker-compose exec -T redis-$PORT redis-cli -a "$PASSWORD" ping &>/dev/null || [ $TIMEOUT -eq 0 ]; do
    sleep 1
    TIMEOUT=$((TIMEOUT-1))
  done
  [ $TIMEOUT -eq 0 ] && { echo "ERROR: 节点 $PORT 启动失败"; exit 1; }
  echo "就绪"
done

# 自动生成集群参数
NODE_ARGS=$(for P in "${NODES[@]}"; do echo -n "redis-$P:$P "; done)

# 幂等创建集群
CLUSTER_STATE=$(docker-compose exec -T redis-7000 redis-cli -a "$PASSWORD" cluster info | grep cluster_state || true)
if [[ "$CLUSTER_STATE" != *"ok"* ]]; then
  echo "创建 Redis Cluster..."
  docker run --rm --network redis-cluster-net redis:7-alpine \
    redis-cli --cluster create $NODE_ARGS --cluster-replicas 1 -a "$PASSWORD" --cluster-yes
fi

echo "=== Redis Cluster 启动完成 ==="

# 打印简洁节点信息表格
echo "=== Redis Cluster 节点信息 ==="
printf "%-40s %-15s %-10s %-10s\n" "NODE ID" "IP:PORT" "ROLE" "STATUS"

# 去重输出每个节点一次
docker-compose exec -T redis-7000 redis-cli -a "$PASSWORD" cluster nodes \
  | awk '{ ipport=$2; split(ipport,a,":"); role=($3 ~ /master/) ? "master" : "slave"; print $1, a[1]":"a[2], role, $3 }' \
  | sort -u \
  | while read id ip role status; do
      printf "%-40s %-15s %-10s %-10s\n" "$id" "$ip" "$role" "$status"
    done

echo "=== Redis Cluster 部署完成 ==="
echo "节点: 7000~7005"
echo "集群访问: redis://127.0.0.1:7000~7005"