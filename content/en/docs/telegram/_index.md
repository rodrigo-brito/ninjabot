---
title: "Telegram"
linkTitle: "Telegram"
categories: ["Reference"]
weight: 5
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

| Command | Description |
|---|---|
| `/help` | Display help instructions. |
| `/status` | Check bot status: `running` or `stopped`. |
| `/balance` | Wallet balance, with the value of the open positions. |
| `/profit` | Summary of the trade results so far. |
| `/start` | Start buying and selling coins. |
| `/stop` | Stop buying and selling coins. |
| `/buy BTCUSDT 100` | Market buy of 100 USDT (the amount is in quote currency). |
| `/buy BTCUSDT 50%` | Market buy with 50% of the available quote balance. |
| `/sell BTCUSDT 100` | Market sell of 100 USDT worth of the asset. |
| `/sell BTCUSDT 50%` | Market sell of 50% of the asset balance. |

![telegram](https://user-images.githubusercontent.com/7620947/150681951-f81c83ae-203e-4b48-8fba-14c59c08abb4.gif)

### Stopping the bot

`/stop` pauses **every** new order, not only the entries of your strategy. The protective stops and take profits your strategy tries to place are rejected too, so an open position is left unprotected until you send `/start`.

Orders rejected while the bot is stopped are reported back to you, instead of failing silently:

> Bot is stopped, no order was created. Send /start to resume.

The same start/stop control is available on the [dashboard control panel]({{< relref "/docs/dashboard" >}}).

## Notifications

Besides answering commands, Telegram receives a notification for every order created, filled, canceled or rejected, and for the errors of the bot. Nothing else is needed: when `Telegram.Enabled` is `true` in the settings, `NewBot` creates the Telegram client and registers it as the notifier of the bot.

### E-mail notifications

Any type that implements `service.Notifier` can be used as the notifier of the bot, through the `WithNotifier` option - for example, the e-mail notifier:

```go
import "github.com/rodrigo-brito/ninjabot/notification"

mail := notification.NewMail(notification.MailParams{
	SMTPServerAddress: "smtp.gmail.com",
	SMTPServerPort:    587,
	From:              "from@example.com",
	To:                "to@example.com",
	Password:          os.Getenv("SMTP_PASSWORD"),
})

bot, err := ninjabot.NewBot(ctx, settings, exch, strategy, ninjabot.WithNotifier(mail))
```

{{% pageinfo color="warning" %}}
Only one notifier is active at a time. When Telegram is enabled, it takes precedence over the one registered with `WithNotifier`, so use `WithNotifier` for bots that do not use Telegram.
{{% /pageinfo %}}
