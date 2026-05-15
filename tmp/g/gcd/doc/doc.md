## 游戏文档

https://wjq224.axshare.com/#g=1&id=9zmum8&p=%E9%A1%B9%E7%9B%AE%E8%AF%B4%E6%98%8E_3

# proto 指令

```bash
protoc --proto_path=. --go_out=../pb --go_opt=paths=source_relative *.proto
find "../pb" -name "*.pb.go" -type f -exec sed -i '' 's/,omitempty//g' {} +
```