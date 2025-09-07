# WireGuard DDNS

[![wg-ddns](https://img.shields.io/badge/LICENSE-GPLv3%20Liscense-blue?style=flat-square)](./LICENSE)
[![wg-ddns](https://img.shields.io/badge/GitHub-WireGuard%20DDNS-blueviolet?style=flat-square&logo=github)](https://github.com/fernvenue/wg-ddns)

A lightweight tool that provides DDNS dynamic DNS support for WireGuard.

## Features

- [x] Auto-discover active WireGuard interfaces and check `wg-quick` configurations;
- [x] Customizable DNS resolution check interval;
- [x] Single interface mode - monitor specific WireGuard interface by name;
- [x] API interface for triggering WireGuard interface restarts via webhook;
- [x] Header-based API authentication;
- [x] Swagger documentation support for API;
- [x] Rich logging output with INFO, DEBUG levels;
- [ ] Environment variable support;
- [ ] Provide systemd service template;
- [ ] Package and deploy via Nix.

## Parameters

- `--single-interface`: Specify a single WireGuard interface to monitor. If not specified, auto-discovers all active interfaces;
- `--listen-address`: Listen address for API service, supports IPv4 and IPv6 addresses;
- `--listen-port`: Listen port for API service;
- `--api-key`: Authentication key for API service;
- `--log-level`: Log output level, options: `debug`, `info`, `warn`, `error`, default: `info`;
- `--check-interval`: DNS resolution check interval, supports time units like `s`, `m`, `h`, default: `10s`;
- `--version`: Show version information;
- `--help`: Show help information.

## Usage Examples

- Auto-discover

```
wg-ddns
```

- Specify interface

```
wg-ddns --single-interface wg0
```

- Set check interval

```
wg-ddns --check-interval 5m
```

- Enable debug logging

```
wg-ddns --log-level debug
```

- Enable API service

```
wg-ddns --listen-address "[::1]" --listen-port 8080 --api-key "your_api_key"
```

- Single interface mode with API service

```
wg-ddns --single-interface wg0 --listen-address "[::1]" --listen-port 8080 --api-key "your_api_key"
```