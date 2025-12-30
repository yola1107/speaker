# Redis Cluster 部署配置

基于Docker的Redis集群配置，3主3从高可用架构。

## 📁 文件结构

```
redis-cluster/
├── docker-compose.yml      # Docker编排文件
├── start-cluster.sh        # 一键部署脚本
├── monitor-memory.sh       # 内存监控脚本
├── conf/                   # Redis配置文件
└── README.md              # 说明文档
```

## 🚀 快速开始

### 部署集群
```bash
cd tmp/data/redis-cluster
chmod +x start-cluster.sh
./start-cluster.sh
```

### 连接使用
```bash
# 连接集群
redis-cli -c -h localhost -p 7000 -a "A12345!"

# 基本操作
set test_key "Hello Redis Cluster"
get test_key

# 查看集群状态
cluster info
cluster nodes
```

### 内存监控
```bash
# 监控内存使用
./monitor-memory.sh
```

## ⚙️ 核心配置

### 集群架构
- **3主3从**：7000/7002/7004为主节点，7001/7003/7005为从节点
- **内存限制**：每个节点2GB（针对32GB系统优化）
- **数据持久化**：AOF模式
- **访问认证**：密码 "A12345!"

### 端口映射
- 7000 → Master 1
- 7001 → Slave 1
- 7002 → Master 2
- 7003 → Slave 2
- 7004 → Master 3
- 7005 → Slave 3

## 🔧 基本操作

### 查看集群状态
```bash
# 集群信息
docker exec redis-7000 redis-cli -a "A12345!" cluster info
cluster nodes

# 连接测试
redis-cli -c -h localhost -p 7000 -a "A12345!"
set test "hello"
get test
```

### 数据操作
```bash
# 键会自动分布到不同节点
set user:1 "Alice"
set user:2 "Bob"
get user:1

# 查看键所在槽位
cluster keyslot user:1
```

## 🔍 故障排除

### 常见问题
```bash
# 集群启动失败
docker-compose down -v && docker-compose up -d
./start-cluster.sh

# 连接不上集群
redis-cli -c -h localhost -p 7000 -a "A12345!"

# 查看日志
docker logs redis-7000
```

## 📊 监控

### 内存监控
```bash
# 快速查看所有节点内存使用
./monitor-memory.sh

# 实时监控
watch -n 5 ./monitor-memory.sh
```

### 集群状态
```bash
# 集群信息
cluster info
cluster nodes

# 连接数统计
info clients
```

## 📝 说明

- **内存配置**：每个节点2GB限制
- **认证密码**：A12345!
- **数据持久化**：AOF模式
- **高可用**：支持自动故障转移

## 🧹 清理环境
```bash
# 停止集群
docker-compose down

# 清理数据
docker-compose down -v
```

---

