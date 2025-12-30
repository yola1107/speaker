#!/bin/bash
set -e
echo "=== Codis 集群初始化开始 ==="

# 检查和设置数据目录权限
echo "检查数据目录权限..."
if [ -d "data" ]; then
    echo "设置数据目录权限..."
    chmod -R 755 data/
    echo "✓ 数据目录权限设置完成"
else
    echo "✗ data 目录不存在，请确保项目结构正确"
    exit 1
fi

# 创建数据目录（如果不存在）
for dir in 7000 7001 7002 7003; do
    mkdir -p "data/$dir"
    chmod 755 "data/$dir"
done

# 等待 Redis 启动
echo "等待 Redis 启动..."
for port in 7000 7001 7002 7003; do
    until docker exec redis-$port redis-cli ping &>/dev/null; do
        sleep 2
    done
done

# 等待 Dashboard 启动
echo "等待 Dashboard 启动..."
until curl -s http://localhost:18080 &>/dev/null; do
    sleep 2
done

# 创建 Proxy
docker exec codis-dashboard codis-admin -c /config/dashboard.toml --create-proxy -a codis-proxy:11080

# 创建 Redis Group
docker exec codis-dashboard codis-admin -c /config/dashboard.toml --create-group -g 1

# 添加 Redis 实例到 Group
for port in 7000 7001 7002 7003; do
    docker exec codis-dashboard codis-admin -c /config/dashboard.toml --group-add -g 1 -r redis-$port:$port
done

# 初始化 slots
docker exec codis-dashboard codis-admin -c /config/dashboard.toml --rebalance --yes

echo "=== Codis 集群初始化完成 ==="
echo "集群访问地址: <宿主机IP>:19000"
echo "管理界面: http://<宿主机IP>:18080"



#启动流程
#docker-compose up -d
#./init-cluster.sh