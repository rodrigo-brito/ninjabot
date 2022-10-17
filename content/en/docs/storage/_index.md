---
title: "Storage"
linkTitle: "Storage"
categories: ["Reference"]
weight: 2
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
)

dsn := "user:pass@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local"
storage, err := storage.FromFile(mysql.Open(dsn), &gorm.Config{})
if err != nil {
    log.Fatal(err)
}
```
