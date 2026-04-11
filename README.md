# phonecall

## 编译

### 前置要求

- Go 1.16 或更高版本
- Node.js 16 或更高版本

### 编译步骤

```bash
# 编译完整项目（包含前端）
make build

# 仅编译前端
make web

# 清理构建产物
make clean
```

编译完成后会生成 `phonecall` 可执行文件。

## 使用

```bash
# 运行应用
./phonecall
```

在浏览器中打开应用地址（默认为 `https://{DOMAIN}}:8443`）即可使用。
