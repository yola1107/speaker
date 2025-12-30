# Codis Redis 集群

基于Docker的Codis Redis集群配置，提供高可用性和自动故障转移。

## 快速开始

### 一键部署 (推荐)
```bash
./start.sh
```
该脚本会自动清理旧进程、设置权限、启动服务并初始化集群。

### 手动部署
```bash
# 1. 设置数据目录权限
chmod -R 755 data/

# 2. 启动服务
docker-compose up -d

# 3. 初始化集群
./init-cluster.sh
```

## 访问集群

```bash
# 连接集群 (密码已配置)
redis-cli -p 19000

# 管理界面
# Dashboard: http://localhost:18080
# Proxy管理: http://localhost:11080
```

## 架构组件

- **etcd**: 集群协调器
- **codis-dashboard**: 管理界面 (18080)
- **codis-proxy**: Redis代理 (19000)
- **codis-server-7000/7001/7002/7003**: 4个Codis服务端
- **redis-7000/7001/7002/7003**: 4个Redis实例

## 故障排除

```bash
# 查看服务状态
docker-compose ps

# 查看日志
docker-compose logs -f codis-proxy

# 重启集群
docker-compose restart

# 清理数据重置
docker-compose down -v

# 一键重启集群
./start.sh
```

## 注意事项

- 确保数据目录权限正确 (`chmod -R 755 data/`)
- 生产环境请修改Redis密码
- 默认密码: `codis_test_2024`
