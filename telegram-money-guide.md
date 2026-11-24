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

Use a category from the Money tab. Without an explicit date, ClearDay uses when you sent the message in Kuala Lumpur time. The default account is **Unspecified**. Select **Period → All months** to include older transactions.

## Edit or delete from the Telegram card

After a transaction is saved, the bot sends a receipt with its amount, category, date, account, note, and action buttons.

- **Amount**, **Date**, and **Note** ask you to reply with a new value. Enter an amount such as `25.50`, a date such as `2026-11-24`, or `-` to clear a note. Reply prompts expire after 15 minutes. Send `/cancel` to cancel.
- **Category**, **Account**, and **Type** show choices. Category and account also accept custom text. Changing type sets an incompatible category to **Other**, which you can then change.
- **Delete** asks for confirmation before removing the saved transaction.
- **Refresh** updates an older card with the current saved transaction. If another edit has changed it, review the latest card before applying a change.

The bot edits the same ledger as the Money page. The visible Money page checks for changes every three seconds and when you return to it. Automatic updates pause during form editing and while the tab is hidden. Filters stay selected, so an entry outside the selected period or search can be saved without appearing in the current list.

New transactions receive cards after this backend update is deployed. Existing text-only receipts keep their original format.

## Troubleshooting

- If **Connect Telegram** reports pending setup, verify both EC2 environment settings and restart `clearday`.
- If the bot stays silent after you press **Start**, generate a new link and use it in the private chat. Check `sudo journalctl -u clearday -n 50 --no-pager` for Telegram errors. HTTP `401` usually means the token needs checking. HTTP `409` can mean another instance is polling the bot or a webhook is configured. [Telegram Bot API](https://core.telegram.org/bots/api#getupdates)
- If the bot says it saved an entry but you missed the confirmation, refresh Money before sending the transaction again. Retried deliveries of one message are deduplicated. Sending a new message creates a new transaction.
