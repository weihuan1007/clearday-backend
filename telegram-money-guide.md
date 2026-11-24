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
| `RM12.50 lunch today tng` | An RM12.50 Food expense today, paid with Touch 'n Go, with lunch as the note |
| `RM12.50 lunch today` | An RM12.50 Food expense today |
| `RM12.50 dinner` | An RM12.50 Food expense on the day you sent it |
| `RM12.50 yesterday` | An RM12.50 Other expense dated yesterday |
| `RM20 food dinner` | An RM20 expense in Food, with dinner as the note |
| `RM12.50 Transport Grab @yesterday` | An RM12.50 expense dated yesterday |
| `income RM1,200.00 Salary @2026-11-09` | RM1,200 of income on the date you specify |
| `transfer RM100 Savings` | An RM100 transfer in Savings |

You can put the RM amount, description, date, and account in different orders. `lunch RM12.50 tng yesterday` works too. A bare amount at the beginning uses MYR, so `12.50 dinner cash` also works.

Replying to an ordinary bot receipt can create a new transaction. A reply to an edit prompt changes the selected field and follows that prompt's expiry and cancellation rules.

Expense is the default type. Use **income** or **received** at the beginning for income, and **transfer** for transfers. A category name at the beginning of the description takes precedence over keyword inference. For example, `RM20 Transport Grab dinner` stays in Transport.

Common meal words such as breakfast, lunch, dinner, coffee, and snacks use **Food**. Petrol, parking, toll, and train use **Transport**. Other familiar keywords cover bills, health, shopping, family, and entertainment. Unrecognized descriptions use **Other**, and the card lets you edit the result.

Recognized accounts include **tng / Touch 'n Go**, **applepay / Apple Pay**, **grabpay / GrabPay**, **bank**, **cash**, **credit card / cc**, and **debit card**. You can include **using**, **via**, **with**, or **paid with** before an account. The default account is **Unspecified**.

Dates can be **today**, **yesterday**, **tomorrow**, **last night**, **2 days ago**, **YYYY-MM-DD**, or **DD/MM/YYYY**. Short forms **tdy**, **ytd**, and **tmr** work. The original **@date** form works anywhere and takes precedence over an unmarked date. Dates use when you sent the message in Kuala Lumpur time.

Send one amount, account, and date per message. Conflicting metadata, invalid amounts, and invalid dates produce a correction message before saving. An ambiguous period such as **last week** requires an exact date. Choose **All time** in Money to include older transactions.

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
