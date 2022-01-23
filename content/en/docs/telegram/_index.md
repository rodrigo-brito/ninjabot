---
title: "Telegram"
linkTitle: "Telegram"
categories: ["Reference"]
weight: 2
description: >
    This page describes how to set up Telegram with Ninjabot.
---

## Installation

To set up a Telegram bot, you should follow these steps:

### Create a new bot in your Telegram account 

You can create bots in telegram accessing [BotFather](https://telegram.me/BotFather).

Send the message `/newbot`.

BotFather response:

> Alright, a new bot. How are we going to call it? Please choose a name for your bot.

Choose the public name of your bot (e.x. `NinjaBot`)

BotFather response:

> Good. Now let's choose a username for your bot. It must end in bot. Like this, for example: TetrisBot or tetris_bot.

Choose the name id of your bot and send it to the BotFather (e.g. `my_ninjabot_bot`)

BotFather response:

> Done! Congratulations on your new bot. You will find it at t.me/my_ninjabot_bot. You can now add a description, about section and profile picture for your bot, see /help for a list of commands. By the way, when you've finished creating your cool bot, ping our Bot Support if you want a better username for it. Just make sure the bot is fully operational before you do this.  
Use this token to access the HTTP API: `111111:ABCDEFGH`

Copy the API Token (`111111:ABCDEFGH` in the above example) and store it in a safe place.


### Discovering your ID

Ninjabot requires your account ID to limit the access of the bot to your account.

Talk to the [userinfobot](https://telegram.me/userinfobot) and send the command `/start` to get your ID.

Example of bot respose:

>@example  
Id: 12345  
First: Foo  
Last: Bar  
Lang: en

Get your "Id" and store in a safe place.

### Setup NinjaBot

With your ID and API Token, you can now setup NinjaBot, the bot settings are place in `ninjabot.Settings` as follow:

```go
settings := ninjabot.Settings{
    Pairs: []string{
        "BTCUSDT",
        "ETHUSDT",
    },
    Telegram: ninjabot.TelegramSettings{
        Enabled: true,
        Token:   "111111:ABCDEFGH",
        Users:   []int{12345},
    },
}
```

## Usage

Telegram bot requires that your bot is `running` to control and get information about your account.

We have the following commands:

- `/help` - Display help instructions
- `/stop` - Stop buy and sell coins
- `/start` - Start buy and sell coins
- `/status` - Check bot status
- `/balance` - Wallet balance
- `/profit` - Summary of last trade results
- `/buy` - open a buy order
- `/sell` - open a sell order

![telegram](https://user-images.githubusercontent.com/7620947/150681951-f81c83ae-203e-4b48-8fba-14c59c08abb4.gif)
