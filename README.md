<p align="center">
  	<img alt="logo" width="150" src="resources/logo_round.png" >
</p>

# @LightningTipBot 🏅

A Telegram Lightning ⚡️ Bitcoin wallet and tip bot for group chats.

This repository contains everything you need to set up and run your own tip bot. If you simply want to use this bot in your group chat without having to install anything just start a conversation with [@LightningTipBot](https://t.me/LightningTipBot) and invite it into your group chat.

## Setting up the bot

### Installation

To build the bot from source, clone the repository and compile the source code.

```
git clone https://github.com/LightningTipBot/LightningTipBot.git
cd LightningTipBot
go build .
cp config.yaml.example config.yaml
```

### Configuration

You need to edit `config.yaml` before you can start the bot.

#### Create a Telegram bot

First, create a new Telegram bot by starting a conversation with the [@BotFather](https://core.telegram.org/bots#6-botfather). After you have created your bot, you will get an **Api Token** which you need to add to `telegram_api_key` in config.yaml accordingly.

#### Set up LNbits

You can either use your own LNbits instance (recommended) or create an account at [lnbits.com](https://lnbits.com/) to use their custodial service (easy).

1. Create a wallet in LNbits (reachable by the bot via `lnbits_url`).
2. Get the **Admin key** in the API Info tab of your user (`lnbits_admin_key`).
3. Enable the User Manager extension.
4. Get the **Admin ID** of the User Manager (`lnbits_admin_id`).

#### Getting LNbits keys

<p align="center">
  	<img alt="How to set up a lnbits wallet and the User Manager extension." src="resources/lnbits_setup.png" >
</p>

#### More configuration

- `db_path`: User database file path.
- `transactions_path`: Transaction database file path.
- `buntdb_path`: Object storage database file path.
- `lnbits_webhook_server`: URL that lnbits can reach the bot with. This is used for creating webhooks from LNbits to receive notifications about payments (optional).
- `message_dispose_duration`: Duration in seconds after which `/tip` are deleted from a channel (only if the bot is channel admin).
- `http_proxy` uses a proxy for all LNURL-related outbound requests (optional).
- `lnurl_public_host_name` is the public URL of your lnbits/LndHub (for BlueWallet/Zeus support, optional).
- `lnurl_server` is the public URL for inbound LNURL payments and your lightning address host (optional).

### Start 

Run following command after the configuration

```
./LightningTipBot
```

### Alternative way to Start

Install as a service

 sudo cp lightningtipbot.service.example /etc/systemd/system/lightningtipbot.service

Configure PATH-TO-INSTALLATION
 sudo pico /etc/systemd/system/lightningtipbot.service

 ENABLE SERVICE
- sudo systemctl enable --now lightningtipbot.service
 RUN
- sudo systemctl start lightningtipbot.service
 VERIFY
- sudo systemctl status lightningtipbot.service

## Features

#### Commands

```
/tip 🏅 Reply to a message to tip it: /tip <amount> [<memo>]
/balance 👑 Check your balance: /balance
/send 💸 Send funds to a user: /send <amount> <@user> or <user@domain.com> [<memo>]
/invoice ⚡️ Receive over Lightning: /invoice <amount> [<memo>]
/pay ⚡️ Pay over Lightning: /pay <invoice>
/help 📖 Read this help.
/advanced 🤖 Read the advanced help.
/basics 📚 More info.
/donate ❤️ Donate to the project: /donate <amount>
```

#### Advanced commands

```
/link 🔗 Link your wallet to BlueWallet or Zeus
/lnurl ⚡️ Lnurl receive or pay: /lnurl or /lnurl <lnurl>
```

### Inline commands
```
send 💸 Send sats to chat: @LightningTipBot send <amount> [<memo>]
```

📖 You can use inline commands in every chat, even in private conversations. Wait a second after entering an inline command and click the result, don't press enter.

### Inline send

You can use inline commands to send payments to anyone who can read your messages, even inside private chats and group chat in which the bot isn't part of. For that, you need to trigger an [inline command](https://core.telegram.org/bots/inline). Here is how it works: Enter the name of the bot (`@LightningTipBot`) and the command you want to trigger (`send 13 Hi friend!`). When the result pops up above the text field, click it to send it to the chat. Do not press enter. 

<p align="center">
  	<img alt="Send sats inside any chat, including private conversations and groups." src="resources/inline_send.png" >
</p>

### Live tooltips

The bot replies to a tipped message to indicate to all participants how many and what amount of tips a post has received. This tooltip will be updated as new tips are given to a post.

<p align="center">
  	<img alt="How to set up a lnbits wallet and the User Manager extension." src="resources/tooltips.png" >
</p>


### LNURL server

Users can send and receive via . For this to work, you need to set the `lnurl_public_server` in `config.yaml`. The bot will then host a LNURL endpoint at `.well-known/lnurlp/username` which handles the data exchange with other wallets. You can set `http_proxy` in `config.yaml` to send outbound requests only via an HTTP proxy.

### Send and receive via Lightning Address

Every user has a [Lightning Address](https://lightningaddress.com/) a la `username@host.com` with which they can send to via `/send <amount> <user@domain.com>` and receive from other wallets.

### Link to BlueWallet or Zeus

Every user can link their wallet to an external app like [Bluewallet](https://bluewallet.io/) or [Zeus](https://zeusln.app/) by using the command `/link`. If you host the bot, you will have to enable the LndHub extension in LNbits. You also need to edit the `lnbits_public_url` entry in `config.yaml` accordingly to an address that can be reached by the user's wallet (Tor should be fine as well).

<p align="center">
  	<img alt="QR code payment example." src="resources/lndhub.png" >
</p>

### Pay invoices by sending QR codes

To pay a Lightning invoice, you can snap a photo of a QR code and send it directly to the bot. Note that you might need to zoom in, center the QR code, or crop the image if the bot fails to decode the QR code from the photo. By the way, you can also just send an the invoice as a string, the bot will automatically detect it and initiate a payment.

<p align="center">
  	<img alt="QR code payment example." src="resources/qr_code_example.jpg" >
</p>

### Auto-delete commands

To minimize the clutter all the heavy tipping can cause in a group chat, the bot will remove all failed commands (for example due to a syntax error) from the chat immediately. All successful commands will stay visible for `message_dispose_duration` seconds (default 10s) and then be removed. The tips will sill be visible for everyone in the Live tooltip. This feature only works, if the bot is made admin of the group.

## Full Guide to Install and run on a VPS

A complete guide to install and run LightningTipBot + LNBITS (on docker with PostgreSQL) on the same VPS with an external LND funding source has been prepared by Massimo Musumeci (@massmux) and it is available: [LightningTipBot full install](https://www.massmux.com/howto-complete-lightningtipbot-lnbits-setup-vps/)

## Made with

- [LNbits](https://github.com/lnbits/lnbits) – Free and open-source lightning-network wallet/accounts system.
- [telebot](https://github.com/tucnak/telebot) – A Telegram bot framework in Go.
- [gozxing](https://github.com/makiuchi-d/gozxing) – barcode image processing library in Go.
- [ln-decodepay](https://github.com/fiatjaf/ln-decodepay) – Lightning Network BOLT11 invoice decoder.
- [go-lnurl](https://github.com/fiatjaf/go-lnurl) - Helpers for building lnurl support into services.
