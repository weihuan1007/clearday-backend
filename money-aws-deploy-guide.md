# Deploy the Money tab to AWS

Deploy in this order. Create the DynamoDB table and grant EC2 access, then update the Go API on EC2, then publish the frontend to S3 and invalidate CloudFront.

Use the existing ClearDay AWS account, region, CloudFront distribution, frontend bucket, EC2 instance, and Cognito app client. In the examples the region is Singapore (`ap-southeast-1`). Use the region your installation already uses. For Cognito setup, see [the ClearDay login guide](cognito-login-guide.md).

## 1. Create the transactions table

Choose the path that matches how you created the existing deployment.

### AWS Console

1. Open **DynamoDB → Tables → Create table** in your ClearDay region. [AWS DynamoDB console instructions](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/rbac-create-table.html)
2. Enter `clearday-money-transactions` as the table name. Set the partition key to `owner` (**String**) and sort key to `id` (**String**).
3. Use **On-demand** capacity. Enable deletion protection in the table settings. Create the table, wait for **Active**, and copy its ARN from the table details.
4. In **EC2 → Instances**, select the backend instance and open the **IAM role** shown in its details. In IAM, choose **Add permissions → Create inline policy → JSON**. Paste this policy after replacing the example resource with the table ARN. Name it `clearday-money-access`.

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["dynamodb:Query", "dynamodb:PutItem", "dynamodb:DeleteItem"],
      "Resource": "arn:aws:dynamodb:ap-southeast-1:123456789012:table/clearday-money-transactions"
    }
  ]
}
```

### Existing Terraform deployment

Use the state and variables already used to deploy ClearDay. In the existing `terraform.tfvars`, add:

```hcl
money_table_name = "clearday-money-transactions"
```

From PowerShell:

```powershell
Set-Location 'D:\reminder-app\repos\backend\deploy\terraform'
terraform init
terraform plan -out=money.tfplan
```

Review the plan. The Money change adds one table and one inline policy on the existing EC2 role. Investigate any unrelated changes or replacements before applying the plan.

```powershell
terraform apply money.tfplan
terraform output -raw money_table_name
```

## 2. Configure and deploy the EC2 backend

1. Connect to the existing instance using **EC2 → Instances → Connect → Session Manager** or your usual SSH connection.
2. Edit the service environment with `sudoedit /etc/clearday.env`. Add the line below using your actual table name. Keep the existing reminder storage and Cognito settings.

   ```dotenv
   CLEAR_DAY_MONEY_TABLE=clearday-money-transactions
   ```

3. In PowerShell, open the **backend** checkout and review the working tree.

   ```powershell
   Set-Location 'D:\reminder-app\repos\backend'
   git status --short
   git diff
   ```

   Stage the backend changes you intend to deploy, including the new `backend/internal/money/` package and Cognito changes. Review the staged diff, then commit and push.

   ```powershell
   git diff --cached
   git commit -m "Add signed-in money transactions"
   git push origin main
   ```
4. In the backend GitHub repository, open **Actions → Deploy Backend** and wait for the run for that commit to succeed. The workflow packages the backend, uploads the release to the private deployment bucket, and tells Systems Manager to install it on EC2. Its existing variables are `AWS_REGION`, `AWS_BACKEND_ROLE_TO_ASSUME`, `AWS_BACKEND_DEPLOY_BUCKET`, and `AWS_BACKEND_INSTANCE_ID`. [ClearDay SSM setup](ssm-github-actions-deploy-guide.md)
5. On EC2, verify the service and confirm the new endpoint rejects a signed-out request. The health request should return `200` and the transactions request should return `401`.

   ```bash
   sudo systemctl is-active clearday
   curl -i http://127.0.0.1:8080/api/health
   curl -i http://127.0.0.1:8080/api/transactions
   ```

If your backend deploy is manual, archive the reviewed backend commit and copy that archive to the instance via your usual SSH/SCP connection. The checkout has a deployment script that builds, installs, and restarts ClearDay. [Manual EC2 deployment instructions](deploy/aws/README.md)

```powershell
# Run from the backend checkout after committing the release.
git archive --format=tar.gz --output=..\clearday-backend-release.tar.gz HEAD backend deploy
# Replace the key path and EC2 hostname with your own SSH details.
scp -i 'C:\path\to\ec2-key.pem' ..\clearday-backend-release.tar.gz ubuntu@EC2_PUBLIC_DNS:~/
```

On EC2, after uploading the archive to your home directory:

```bash
release="$(mktemp -d)"
tar -xzf ~/clearday-backend-release.tar.gz -C "$release"
sudo bash "$release/deploy/aws/update-backend.sh" "$release"
```

## 3. Publish the frontend

### GitHub Actions

1. In PowerShell, open `D:\reminder-app\repos\frontend`. Review `git status --short` and `git diff`. Stage the frontend changes you intend to deploy, including the new `money-*.js` files and `money.css`, then commit and push.

   ```powershell
   git diff --cached
   git commit -m "Add the ClearDay Money tab"
   git push origin main
   ```

2. Open **Actions → Deploy Frontend** in the frontend GitHub repository. Wait for the run for that commit to succeed. It uploads the assets and invalidates CloudFront.
3. Verify the workflow still uses `AWS_FRONTEND_BUCKET` and `CLOUDFRONT_DISTRIBUTION_ID` for your existing deployment, `FRONTEND_API_BASE=/api`, and your existing `COGNITO_DOMAIN`, `COGNITO_CLIENT_ID`, `COGNITO_REDIRECT_URI`, and `COGNITO_LOGOUT_URI`. The redirect and logout URLs are the existing site root with its trailing slash.

### Manual S3 upload

1. Open **S3 → Buckets → your ClearDay frontend bucket → Objects → Upload**. Add these files from `D:\reminder-app\repos\frontend` directly to the bucket root.

   ```text
   api.js
   app.js
   calendar-view.js
   session-view.js
   money-model.js
   money-repository.js
   money-page.js
   money.css
   ```

2. Upload `index.html` after those assets. Use the production `config.js` already in the bucket. Upload application assets only.
3. In **CloudFront → Distributions → your distribution → Invalidations**, create an invalidation for `/*` and wait for it to complete. [AWS CloudFront invalidation instructions](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/Invalidation_Requests.html)
4. Under **Behaviors**, verify the existing `/api/*` behavior routes to EC2, forwards `Authorization`, allows `POST`, `PUT`, and `DELETE`, and disables caching. [AWS CloudFront cache behavior settings](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/DownloadDistValuesCacheBehavior.html)

## 4. Verify the deployed app

1. Open your current ClearDay HTTPS link, sign in, and select **Money**. You can bookmark the link with `#money` at the end.
2. Add a test income of RM10.00 and a test expense of RM2.35 in the same month. Check that **Net** shows RM7.65. Reload and confirm both persist. Edit one, verify the total changes, then delete both.
3. If another test Cognito account is available, sign in with it in a separate browser session and check that it has its own empty ledger.
4. Open **Calendar** and **Reminders** and confirm your reminder list still loads.

If Money fails to load, check the request to `/api/transactions` in the browser's Network tab. A `404` usually means the new backend or `/api/*` routing is missing, or `CLEAR_DAY_MONEY_TABLE` is unset. A `401` after sign-in indicates an access token or Cognito configuration problem. For a `500`, check the table name, region, and EC2 IAM permissions with `sudo journalctl -u clearday -n 50 --no-pager`.

## 5. Move existing Money Flow records

1. Open the old Money Flow app at its usual address in the same browser and profile you used to record transactions. Click **Export records** in the header to save the `money-flow-*.json` file.
2. Sign in to ClearDay with the account that should own the records. In **Money → Import JSON**, choose the export and confirm. ClearDay shows the number added, already present, and invalid. It keeps existing records with the same ID.
3. Choose **All months** to check the full history. Transfer records appear in the list and are excluded from income, expenses, and net. If the import stops partway, choose the same file again to resume. The original app and JSON backup remain separate from the AWS upload.

For bank statements, use Money Flow's statement import first, then export the resulting records as JSON.

## Local preview and code map

Start the Go backend from `D:\reminder-app\repos\backend\backend` with `go run ./cmd/clearday-server`. Open `http://localhost:8080/#money`. With Cognito off, the Money tab uses browser storage for preview. Use dummy data when testing.

- Frontend `money-model.js` handles integer sen, categories, totals, and Money Flow backup conversion.
- Frontend `money-repository.js` handles cloud requests and localhost storage.
- Frontend `money-page.js` and `money.css` handle forms, filters, the monthly view, and imports.
- Backend `backend/internal/money/` contains transaction validation, the HTTP API, and DynamoDB access. The API verifies the Cognito access token and derives the owner on the server for every operation.
