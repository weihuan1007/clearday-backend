# Record ClearDay Money transactions from Telegram

Telegram reads messages from a private chat with your bot. ClearDay connects that chat to your Cognito account and saves transactions in the same ledger you edit in **Money**.

## Connect your bot

1. Open Telegram’s official [@BotFather](https://t.me/BotFather), send `/newbot`, and choose a name and a unique username ending in `bot`. BotFather gives you the bot token. [Telegram bot setup](https://core.telegram.org/bots/tutorial#obtain-your-bot-token)
2. In EC2 Session Manager, open `sudo nano /etc/clearday.env`. Set these values once, then restart ClearDay after the new backend is deployed.

   ```dotenv
   CLEAR_DAY_TELEGRAM_BOT_TOKEN=the-token-from-BotFather
   CLEAR_DAY_TELEGRAM_OWNER=your-Cognito-user-sub
   ```

   Find `sub` under your user’s attributes in the ClearDay Cognito user pool. Use a new bot dedicated to ClearDay. Keep its token in the EC2 environment file and use the existing instance role’s DynamoDB `Query` and `PutItem` permissions.
3. Sign in to ClearDay, open **Money**, and choose **Connect Telegram → Open Telegram → Start**. The link expires after 10 minutes and works once. The bot confirms when the chat is connected.

ClearDay checks the saved connection each time you sign in. **Open Telegram** goes directly to the bot chat. Use **Link another chat** if you change Telegram accounts.

## Send a transaction

| Message | What ClearDay records |
|---|---|
| `RM20 food dinner` | An RM20 expense in Food, with dinner as the note |
| `RM12.50 Transport Grab @yesterday` | An RM12.50 expense dated yesterday |
| `income RM1,200.00 Salary @2026-11-09` | RM1,200 of income on the date you specify |
| `transfer RM100 Savings` | An RM100 transfer in Savings |

Use a category from the Money tab. Without an explicit date, ClearDay uses when you sent the message in Kuala Lumpur time. The default account is **Unspecified**, and you can assign it from the transaction’s **Edit** form. After the bot confirms the save, use **Money → Refresh** to see the entry. Select **Period → All months** to include older transactions.

## Troubleshooting

- If **Connect Telegram** reports pending setup, verify both EC2 environment settings and restart `clearday`.
- If the bot stays silent after you press **Start**, generate a new link and use it in the private chat. Check `sudo journalctl -u clearday -n 50 --no-pager` for Telegram errors. HTTP `401` usually means the token needs checking. HTTP `409` can mean another instance is polling the bot or a webhook is configured. [Telegram Bot API](https://core.telegram.org/bots/api#getupdates)
- If the bot says it saved an entry but you missed the confirmation, refresh Money before sending the transaction again. Retried deliveries of one message are deduplicated. Sending a new message creates a new transaction.
