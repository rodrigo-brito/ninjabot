---
title: "Storage"
linkTitle: "Storage"
categories: ["Reference"]
weight: 8
description: >
  This page describes how to set customize the storage (memory, sqlite, sql, etc).
---

## Support

Currently it is possible to use different types of storage, to be a valid storage just sign the following interface:

```go
type Storage interface {
	CreateOrder(order *model.Order) error
	UpdateOrder(order *model.Order) error
	Orders(filters ...OrderFilter) ([]*model.Order, error)
}
```

By default, when no storage is informed, the bot creates a local file called `ninjabot.db`. The storage keeps the orders of the bot, so that pending orders survive a restart.

A storage can be customized at bot startup time, with the option `WithStorage(storage)`:

```go
bot, err := ninjabot.NewBot(
    ninjabot.WithStorage(storage),
    // other options...
)
```

### Memory Storage

An in-memory storage is recommended for backtesting and for situations where you don't want to keep the data in the long term, all data is erased after finishing the execution.

```go
import "github.com/rodrigo-brito/ninjabot/storage"

storage, err := storage.FromMemory()
if err != nil {
    log.Fatal(err)
}
```

### File Storage

A simple file storage format that saves your history in JSON format via BuntDB

```go
import "github.com/rodrigo-brito/ninjabot/storage"

storage, err := storage.FromFile("orders.db")
if err != nil {
    log.Fatal(err)
}
```

### SQL Storage

With SQL Storage you can use relational databases like MySQL, Postgress, SQLite and others. It uses Gorm and the configuration options can be checked [here](https://gorm.io/docs/connecting_to_the_database.html).

```go
import (
    "gorm.io/driver/mysql"
    "gorm.io/gorm"

    "github.com/rodrigo-brito/ninjabot/storage"
)

dsn := "user:pass@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local"
storage, err := storage.FromSQL(mysql.Open(dsn), &gorm.Config{})
if err != nil {
    log.Fatal(err)
}
```

For SQLite, Ninjabot uses the pure Go driver `github.com/glebarez/sqlite`, which requires no CGO:

```go
import (
    "github.com/glebarez/sqlite"

    "github.com/rodrigo-brito/ninjabot/storage"
)

storage, err := storage.FromSQL(sqlite.Open("ninjabot.db"))
if err != nil {
    log.Fatal(err)
}
```

## Querying orders

The orders kept by any storage can be filtered with the helpers of the `storage` package:

```go
orders, err := storage.Orders(
    storage.WithPair("BTCUSDT"),
    storage.WithStatusIn(model.OrderStatusTypeNew, model.OrderStatusTypePartiallyFilled),
)
```

The available filters are `WithPair`, `WithStatus`, `WithStatusIn` and `WithUpdateAtBeforeOrEqual`.
