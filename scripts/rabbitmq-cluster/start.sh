#!/bin/bash
set -e

# --------------------------
# 生产级 RabbitMQ 集群启动
# --------------------------

# 默认用户名密码，可修改
RABBIT_USER=admin
RABBIT_PASS=StrongPassword123!

echo "=== 清理旧容器和网络 ==="
docker-compose down --remove-orphans 2>/dev/null || true
docker network prune -f 2>/dev/null || true

echo "=== 创建数据目录并设置权限 ==="
for d in data/rabbitmq{1,2,3}; do
  mkdir -p "$d"
  chmod 755 "$d"
done

echo "=== 启动 RabbitMQ 节点和 HAProxy ==="
docker-compose up -d

echo "=== 等待节点就绪 ==="
for node in rabbitmq1 rabbitmq2 rabbitmq3; do
  echo "等待 $node 启动..."
  for i in {1..30}; do
    if docker exec "$node" rabbitmq-diagnostics ping >/dev/null 2>&1; then
      echo "✓ $node 已就绪"
      break
    fi
    if [ $i -eq 30 ]; then
      echo "✗ $node 启动超时"
      exit 1
    fi
    sleep 2
  done
done

echo "=== 检查集群组建状态 ==="
CLUSTER_STATUS=$(docker exec rabbitmq1 rabbitmqctl cluster_status 2>/dev/null || echo "")

if ! echo "$CLUSTER_STATUS" | grep -q "rabbit@rabbitmq2\|rabbit@rabbitmq3"; then
  echo "集群未自动组建，手动组建..."
  for node in rabbitmq2 rabbitmq3; do
    docker exec "$node" rabbitmqctl stop_app
    docker exec "$node" rabbitmqctl join_cluster rabbit@rabbitmq1
    docker exec "$node" rabbitmqctl start_app
  done
  echo "等待集群同步..."
  sleep 5
fi

echo "=== 集群状态 ==="
docker exec rabbitmq1 rabbitmqctl cluster_status | grep -E "Cluster name|Running Nodes"

echo ""
echo "集群访问地址: localhost:5672 (通过 HAProxy 负载均衡)"
echo "默认账号密码: $RABBIT_USER / $RABBIT_PASS"
echo "管理界面访问:"
echo "  docker exec -it rabbitmq1 /bin/bash"
echo "  rabbitmq-plugins enable rabbitmq_management"
echo "  然后访问: http://<node-ip>:15672"
