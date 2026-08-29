# TLS 配置说明

## 架构

TLS 在 nginx 层终止，后端应用节点之间保持 HTTP 通信。

```
客户端 (HTTPS/WSS)
    ↓
nginx :443  ← TLS 终止，证书在此加载
    ↓
dipole-node1:8080  (HTTP)
dipole-node2:8080  (HTTP)
```

HTTP 80 端口自动重定向到 HTTPS 443。

## 证书

使用 [mkcert](https://github.com/FiloSottile/mkcert) 生成本地自签名证书，文件位于 `certs/local/`：

```
certs/local/
├── dipole-local.pem       # 服务端证书
├── dipole-local-key.pem   # 私钥
└── rootCA.crt             # 根 CA 证书（需导入客户端信任库）
```

证书 SAN（Subject Alternative Name）包含：
- `localhost`
- `192.168.1.8`（局域网 IP）
- `127.0.0.1`

如需重新生成：

```bash
mkcert -cert-file certs/local/dipole-local.pem \
       -key-file  certs/local/dipole-local-key.pem \
       localhost 192.168.1.8 127.0.0.1
```

## nginx 配置

`nginx/nginx.conf` 关键部分：

```nginx
server {
    listen 80;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl;

    ssl_certificate     /etc/nginx/certs/local/dipole-local.pem;
    ssl_certificate_key /etc/nginx/certs/local/dipole-local-key.pem;

    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers   HIGH:!aNULL:!MD5;
    ...
}
```

## Docker Compose 挂载

`docker-compose.dist.yml` 中 nginx 服务挂载证书目录：

```yaml
nginx:
  volumes:
    - ./nginx/nginx.conf:/etc/nginx/nginx.conf:ro
    - ./certs:/etc/nginx/certs:ro
  ports:
    - "80:80"
    - "443:443"
```

后端节点无需挂载证书，`config.docker.yaml` 中 `tls.enabled: false`。

## 客户端信任配置

### Windows（Chrome / Edge）

1. `Win + R` → `certmgr.msc`
2. 展开 **受信任的根证书颁发机构** → **证书**
3. 右键 → 所有任务 → 导入 → 选择 `certs/local/rootCA.crt`
4. 确认存储位置为"受信任的根证书颁发机构"
5. 完全重启浏览器

### Firefox

Firefox 使用独立证书库，需单独导入：

设置 → 隐私与安全 → 证书 → 查看证书 → 证书颁发机构 → 导入 → 选择 `rootCA.crt`

### Linux

```bash
sudo cp certs/local/rootCA.crt /usr/local/share/ca-certificates/dipole-rootCA.crt
sudo update-ca-certificates
```
