# WireGuard DDNS

[![wg-ddns](https://img.shields.io/badge/LICENSE-GPLv3%20Liscense-blue?style=flat-square)](./LICENSE)
[![wg-ddns](https://img.shields.io/badge/GitHub-WireGuard%20DDNS-blueviolet?style=flat-square&logo=github)](https://github.com/fernvenue/wg-ddns)

為 WireGuard 提供 DDNS 動態域名解析支持的輕量級工具.

## 功能

- [x] 支持自動發現當前活躍的 WireGuard 接口並檢查 `wg-quick` 配置;
- [x] 支持自定義 DNS 解析檢查間隔;
- [x] 支持單接口模式, 可指定 WireGuard 接口名稱;
- [x] 提供 API 接口, 可通過 Webhook 觸發 WireGuard 接口重啟;
- [x] API 接口提供基於 Header 的身份認證;
- [x] API 接口提供 Swagger 文檔支持;
- [x] 豐富的日志輸出, 支持 INFO, DEBUG 等級別;
- [ ] 環境變量支持;
- [ ] 提供 systemd service 模板;
- [ ] 通過 Nix 進行打包和部署.

## 參數說明

- `--single-interface`: 指定單一的 WireGuard 接口進行監控, 如果不指定則自動發現所有活躍接口;
- `--listen-address`: 啟用 API 服務時的監聽地址, 支持 IPv4 和 IPv6 地址;
- `--listen-port`: 啟用 API 服務時的監聽端口;
- `--api-key`: 啟用 API 服務時的身份認證密鑰;
- `--log-level`: 日志輸出等級, 可選值為 `debug`, `info`, `warn`, `error`, 默認值為 `info`;
- `--check-interval`: DNS 解析檢查間隔, 支持時間單位如 `s`, `m`, `h`, 默認值為 `10s`;
- `--version`: 顯示版本信息;
- `--help`: 顯示幫助信息.

## 使用示例

- 自動發現

```
wg-ddns
```

- 指定接口

```
wg-ddns --single-interface wg0
```

- 指定檢查間隔

```
wg-ddns --check-interval 5m
```

- 啟用調試日志

```
wg-ddns --log-level debug
```

- 啟用 API 服務

```
wg-ddns --listen-address "[::1]" --listen-port 8080 --api-key "your_api_key"
```

- 單接口模式下啟用 API 服務

```
wg-ddns --single-interface wg0 --listen-address "[::1]" --listen-port 8080 --api-key "your_api_key"
```
