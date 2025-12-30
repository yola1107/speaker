#!/bin/bash

# Redis Cluster Memory Monitor Script
# 监控所有Redis节点的内存使用情况

echo "=== Redis Cluster Memory Monitor ==="
echo "Time: $(date)"
echo

# 颜色定义
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

# 密码
PASSWORD="A12345!"

# 检查内存使用情况
check_memory() {
    local port=$1
    local container="redis-$port"

    # 获取内存信息
    local memory_info=$(docker exec $container redis-cli -p $port -a "$PASSWORD" info memory 2>/dev/null)

    if [ $? -ne 0 ]; then
        echo -e "${RED}Node $port: CONNECTION FAILED${NC}"
        return
    fi

    # 解析内存信息
    local used_memory=$(echo "$memory_info" | grep "used_memory:" | cut -d: -f2)
    local max_memory=$(echo "$memory_info" | grep "maxmemory:" | cut -d: -f2)
    local used_human=$(echo "$memory_info" | grep "used_memory_human:" | cut -d: -f2)
    local max_human=$(echo "$memory_info" | grep "maxmemory_human:" | cut -d: -f2)

    # 计算使用百分比
    local usage_pct=0
    if [ "$max_memory" -gt 0 ]; then
        usage_pct=$(( used_memory * 100 / max_memory ))
    fi

    # 根据使用率设置颜色
    local color=$GREEN
    if [ $usage_pct -gt 80 ]; then
        color=$RED
    elif [ $usage_pct -gt 60 ]; then
        color=$YELLOW
    fi

    printf "Node %s: ${color}%3d%%${NC} (%s / %s)\n" \
           $port $usage_pct "$used_human" "$max_human"
}

# 检查所有节点
echo "Memory Usage Status:"
echo "-------------------"

for port in 7000 7001 7002 7003 7004 7005; do
    check_memory $port
done

echo
echo "Memory Policy: allkeys-lru (Least Recently Used)"
echo "Max Memory per Node: 2GB"
echo "Total Cluster Memory: 12GB (6 nodes × 2GB)"
echo

# 检查集群状态
echo "Cluster Status:"
echo "--------------"
docker exec redis-7000 redis-cli -p 7000 -a "$PASSWORD" cluster info | grep -E "(cluster_state|cluster_size|cluster_known_nodes)" 2>/dev/null || echo "Failed to get cluster info"

echo
echo "Legend:"
echo "- ${GREEN}Green${NC}: Memory usage < 60%"
echo "- ${YELLOW}Yellow${NC}: Memory usage 60-80%"
echo "- ${RED}Red${NC}: Memory usage > 80%"
