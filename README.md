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
- [x] Environment variable support;
- [x] Provide systemd service template;
- [ ] Package and deploy via Nix.

## Parameters

- `--single-interface`: Specify a single WireGuard interface to monitor. If not specified, auto-discovers all active interfaces;
- `--listen-address`: Listen address for API service, supports IPv4 and IPv6 addresses;
- `--listen-port`: Listen port for API service;
- `--api-key`: Authentication key for API service;
- `--log-level`: Log output level, options: `debug`, `info`, `warn`, `error`, default: `info`;
- `--check-interval`: DNS resolution check interval, supports time units like `s`, `m`, `h`, default: `10s`;
- `--check-only`: Check active WireGuard interfaces and exit (does not start monitoring);
- `--version`: Show version information;
- `--help`: Show help information.

## Environment Variables

In addition to command line parameters, all configuration options support environment variables:

- `WG_DDNS_SINGLE_INTERFACE`: Corresponds to `--single-interface`
- `WG_DDNS_LISTEN_ADDRESS`: Corresponds to `--listen-address`
- `WG_DDNS_LISTEN_PORT`: Corresponds to `--listen-port`
- `WG_DDNS_API_KEY`: Corresponds to `--api-key`
- `WG_DDNS_LOG_LEVEL`: Corresponds to `--log-level`
- `WG_DDNS_CHECK_INTERVAL`: Corresponds to `--check-interval`

**Note**: Command line parameters take precedence over environment variables.

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

- Quick check of active interfaces

```
wg-ddns --check-only
```

- Check specific interface

```
wg-ddns --check-only --single-interface wg0
```
