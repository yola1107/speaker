# Redis Cluster 部署配置

基于Docker的Redis集群配置，3主3从高可用架构。

## 🚀 快速开始

```bash
# 部署集群
cd /data/redis-cluster
chmod +x start-cluster.sh
./start-cluster.sh

# 连接使用
redis-cli -c -h localhost -p 7000 -a "A12345!"
set test "hello"
get test

# 查看状态
cluster info
cluster nodes

# 监控集群
./monitor-cluster.sh

# 实时监控
watch -n 5 ./monitor-cluster.sh

```

## ⚙️ 配置说明

### 集群架构
- **3主3从**: 7000/7002/7004为主节点，7001/7003/7005为从节点
- **自动故障转移**: 主节点故障时，从节点自动提升
- **数据分片**: 16384个哈希槽自动分布

### 端口映射
- 7000 → Master 1
- 7001 → Slave 1
- 7002 → Master 2
- 7003 → Slave 2
- 7004 → Master 3
- 7005 → Slave 3

### 基本配置
- **内存限制**: 每个节点2GB（LRU淘汰）
- **访问认证**: 密码 "A12345!"
- **数据持久化**: AOF模式

### 基本操作
```bash
# 连接集群（必须用-c参数）
redis-cli -c -h localhost -p 7000 -a "A12345!"

# 数据操作（自动分片）
set user:1 "Alice"
get user:1
cluster keyslot user:1  # 查看键分布
```

### 集群管理
```bash
cluster info      # 集群状态
cluster nodes     # 节点列表
info memory       # 内存使用
```

## 🔍 故障排除

```bash
# 启动失败
docker-compose logs
docker-compose down -v && ./start-cluster.sh

# 连接问题
redis-cli -c -h localhost -p 7000 -a "A12345!"

# 状态检查
./monitor-cluster.sh
cluster info
```

## 🧹 清理环境
```bash
# 停止集群
docker-compose down

# 清理数据
docker-compose down -v
```

---


## Linux 系统调优
# 增大文件描述符限制
ulimit -n 65535

# TCP 参数优化
sysctl -w net.core.somaxconn=65535
sysctl -w net.ipv4.tcp_max_syn_backlog=65535
sysctl -w net.ipv4.tcp_fin_timeout=15
sysctl -w net.ipv4.tcp_tw_reuse=1

