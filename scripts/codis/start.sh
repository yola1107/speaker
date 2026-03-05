#!/bin/bash
set -e

echo "=== Codis Redis 集群启动脚本 ==="

docker-compose down --remove-orphans 2>/dev/null || true

containers=(
  codis-etcd
  redis-7000 redis-7001 redis-7002 redis-7003
  codis-dashboard
  codis-server-7000 codis-server-7001 codis-server-7002 codis-server-7003
  codis-proxy
)

for c in "${containers[@]}"; do
  docker stop "$c" 2>/dev/null || true
  docker rm "$c" 2>/dev/null || true
done

docker network prune -f 2>/dev/null || true

echo "✓ 清理完成"

mkdir -p data/{7000,7001,7002,7003}
chmod -R 755 data

echo "✓ 数据目录准备完成"

echo "启动 Codis 集群..."
docker-compose up -d

echo "等待服务稳定启动..."
sleep 20

echo "初始化 Codis 集群..."
./init-cluster.sh

echo ""
echo "=== Codis 集群启动完成 ==="
echo "集群访问地址: localhost:19000"
echo "管理界面: http://localhost:18080"
echo "Proxy管理: http://localhost:11080"
echo ""
echo "测试连接: redis-cli -p 19000 ping"
