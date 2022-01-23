---
title: "Ninjabot CLI"
linkTitle: "Nijabot CLI"
categories: ["Reference"]
weight: 2
description: >
    This page describes how to install and use Ninjabot CLI to download historical data for backtesting.
---

Ninjabot CLI provides utilities commands to support backtesting and bot development.


## Installation

You can install CLI with the following command
```bash
go install github.com/rodrigo-brito/ninjabot/cmd/ninjabot@latest
```
Or downloading pre-build binaries in [release page](https://github.com/rodrigo-brito/ninjabot/releases).

## Helper (`ninjabot -h`)

```
NAME:
   ninjabot - Utilities for bot creation

USAGE:
   ninjabot [global options] command [command options] [arguments...]

COMMANDS:
   download  Download historical data
   help, h   Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --help, -h  show help (default: false)
```

We have the following commands:
- **download**: Download historical data from the binace exchange.

### Download Command

```
NAME:
   download - Download historical data

USAGE:
   download [command options] [arguments...]

OPTIONS:
   --pair value, -p value       eg. BTCUSDT
   --days value, -d value       eg. 100 (default 30 days) (default: 0)
   --start value, -s value      eg. 2021-12-01 (default: <nil>)
   --end value, -e value        eg. 2020-12-31 (default: <nil>)
   --timeframe value, -t value  eg. 1h
   --output value, -o value     eg. ./btc.csv
   --help, -h                   show help (default: false)
```

**Examples of Usage**

- Downloading 30 days of historical data for the **BTC/USDT** pair with 1d timeframe:
```bash
ninjabot download --pair BTCUSDT --timeframe 1d --days 30 --output ./btc-1d.csv
```

- Downloading historical data for the **BTC/USDT** pair with 1h timeframe and from 2020-12-01 to 2020-12-31:
```bash
ninjabot download -p BTCUSDT -t 1h -s "2020-12-01" -e "2020-12-31" -o ./btc-1h.csv
```
