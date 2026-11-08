# ClearDay Personal Login Setup

This guide replaces the shared `CLEAR_DAY_API_TOKEN` with a private Cognito sign-in page. It is designed for one personal user, low cost, and no public sign-up.

## What This Creates

The isolated Terraform folder creates only four login resources:

- Cognito Essentials user pool
- Public browser app client with authorization-code and PKCE
- Cognito managed-login domain
- Default managed-login branding

It does not change the working EC2, DynamoDB, S3, CloudFront, IAM, or networking resources.

The user pool has:

- Admin-only user creation; visitors cannot register themselves
- 12-character strong passwords
- Software authenticator MFA, with no SMS messages
- One-hour access and ID tokens
- 30-day rotating refresh tokens
- Token revocation on sign out
- Deletion protection

Cognito Essentials includes an indefinite free tier for up to 10,000 monthly active direct users. One personal account is comfortably inside that allowance. Essentials is used because AWS includes refresh-token rotation there. Do not switch to Plus, which has no MAU free tier. Email delivery can be billed separately, so keep an AWS Budget alert even though one personal account should have very low usage.

## Before You Start

You need:

- Your CloudFront address, for example `https://d123example.cloudfront.net/`
- AWS CLI credentials that can create Cognito resources
- Terraform 1.6 or newer
- Access to the frontend and backend GitHub repositories
- AWS Systems Manager access to the EC2 instance

The trailing `/` on the CloudFront URL is required. Cognito callback URLs must match exactly.

## 1. Install Terraform

In an Administrator PowerShell window:

```powershell
winget install Hashicorp.Terraform
```

Close and reopen PowerShell, then verify:

```powershell
terraform version
aws sts get-caller-identity
```

The AWS command should show account `884945121942`. Stop if it shows a different account.

## 2. Prepare the Login Variables

From PowerShell:

```powershell
cd D:\reminder-app\repos\backend\deploy\terraform-cognito
Copy-Item terraform.tfvars.example terraform.tfvars
notepad terraform.tfvars
```

Set the exact CloudFront URL:

```hcl
aws_region      = "ap-southeast-1"
application_url = "https://YOUR_CLOUDFRONT_DOMAIN.cloudfront.net/"

cognito_domain_prefix = ""
```

`terraform.tfvars` is ignored by Git and must not be pushed.

## 3. Review and Create Cognito

Run:

```powershell
terraform init
terraform fmt
terraform validate
terraform plan -out cognito.tfplan
```

Read the plan. It should add four `aws_cognito_*` resources and should not mention EC2, S3, DynamoDB, CloudFront, or IAM changes.

When the plan looks correct:

```powershell
terraform apply cognito.tfplan
terraform output
```

Keep the Terraform state in this folder. It is ignored by Git and is needed for later Cognito updates. Do not delete it casually.

Record these outputs:

```text
cognito_user_pool_id
cognito_web_client_id
cognito_domain
application_url
```

None of these four values is a password. They identify the user pool and public browser client.

## 4. Create Your Only User

In the AWS Console:

1. Open **Amazon Cognito** in `ap-southeast-1`.
2. Open the `clearday-users` user pool.
3. Open **Users**, then choose **Create user**.
4. Enter your own email address as the username.
5. Mark the email as verified.
6. Create a strong temporary password or let Cognito generate one.
7. Choose whether Cognito should email the temporary password to you.

Do not enable self-registration. Terraform intentionally keeps it disabled.

On your first website sign-in, Cognito will ask for a new password and software authenticator setup. Scan the QR code with an authenticator application and keep access to that device.

## 5. Deploy the Updated Backend Code

Commit and push the backend changes first. The existing GitHub Actions SSM workflow will test and deploy them. The old API-token configuration continues working until the EC2 environment is changed.

Confirm the workflow succeeds before continuing.

## 6. Change the EC2 Environment

Open the EC2 instance through **AWS Systems Manager > Session Manager**. Back up the environment file:

```bash
sudo cp /etc/clearday.env /etc/clearday.env.before-cognito
sudo nano /etc/clearday.env
```

Keep the existing DynamoDB settings and add values from `terraform output`:

```bash
CLEAR_DAY_ADDR=:8080
CLEAR_DAY_STORE=dynamodb
CLEAR_DAY_DYNAMODB_TABLE=clearday-reminders
CLEAR_DAY_STATIC_DIR=/opt/clearday/app/web/static
AWS_REGION=ap-southeast-1

CLEAR_DAY_COGNITO_USER_POOL_ID=ap-southeast-1_REPLACE_ME
CLEAR_DAY_COGNITO_CLIENT_ID=REPLACE_ME
CLEAR_DAY_COGNITO_DOMAIN=https://REPLACE_ME.auth.ap-southeast-1.amazoncognito.com
CLEAR_DAY_COGNITO_REDIRECT_URI=https://YOUR_CLOUDFRONT_DOMAIN.cloudfront.net/
CLEAR_DAY_COGNITO_LOGOUT_URI=https://YOUR_CLOUDFRONT_DOMAIN.cloudfront.net/
```

Remove these production lines:

```text
CLEAR_DAY_API_TOKEN
CLEAR_DAY_EXPOSE_API_TOKEN
```

Protect and restart:

```bash
sudo chown root:root /etc/clearday.env
sudo chmod 600 /etc/clearday.env
sudo systemctl restart clearday
sudo systemctl status clearday --no-pager
curl http://127.0.0.1:8080/api/health
curl -i http://127.0.0.1:8080/api/reminders
```

Expected results:

- Health returns `{"status":"ok"}`.
- Reminders without a Cognito access token return HTTP `401`.

Also check that runtime config contains no shared token:

```bash
curl http://127.0.0.1:8080/config.js
```

## 7. Configure the Frontend Repository

Open `clearday-frontend` on GitHub:

**Settings > Secrets and variables > Actions > Variables**

Add or update:

```text
FRONTEND_API_BASE=/api
COGNITO_DOMAIN=<cognito_domain output>
COGNITO_CLIENT_ID=<cognito_web_client_id output>
COGNITO_REDIRECT_URI=<application_url output>
COGNITO_LOGOUT_URI=<application_url output>
```

Delete the old `FRONTEND_API_TOKEN` repository secret. It is no longer read by the workflow.

Commit and push the frontend changes. Wait for the frontend deployment workflow and CloudFront invalidation to finish.

## 8. Test the Complete Flow

Use a private browser window first:

1. Open the CloudFront URL over HTTPS.
2. Confirm ClearDay shows a sign-in screen and never asks for an API token.
3. Select **Sign in securely**.
4. Sign in with the Cognito user.
5. Complete the temporary-password and authenticator steps if requested.
6. Confirm existing reminders load from DynamoDB.
7. Create, edit, and delete one test reminder.
8. Select **Sign out** and confirm reminders are no longer visible.
9. Repeat on your phone.

The browser keeps a rotating refresh token so your phone and computer normally remain signed in. The backend accepts only Cognito access tokens for the configured user pool and browser client.

## 9. Final Safety Check

- CloudFront website uses HTTPS.
- S3 frontend bucket remains private behind CloudFront.
- Cognito self-registration remains disabled.
- Cognito uses Essentials, not Plus.
- MFA uses an authenticator application, not SMS.
- EC2 port `22` is removed when you use only SSM, or restricted to your current public IP `/32`.
- EC2 port `8080` may still be reachable for the CloudFront origin, but reminder APIs require a valid Cognito access token.
- `/etc/clearday.env` is mode `600` and contains no AWS access keys.
- GitHub contains no `.pem`, AWS secret key, API token, `.env`, or `terraform.tfstate` file.
- AWS Billing has a small monthly Budget alert.

## Optional Local Cognito Testing

Normal local development can continue with the local API-token fallback in `backend/.env`. This keeps dummy development data separate and does not affect production login.

To test Cognito locally later, set `local_application_url` in `deploy/terraform-cognito/terraform.tfvars`, apply Terraform, and use the same exact URL in the local backend Cognito settings. Do not add local callback URLs that you do not actively use.

## Rollback

If the frontend has not been deployed yet, restore the EC2 backup and restart:

```bash
sudo cp /etc/clearday.env.before-cognito /etc/clearday.env
sudo chmod 600 /etc/clearday.env
sudo systemctl restart clearday
```

Do not run `terraform destroy` as a troubleshooting step. The user pool has deletion protection so an accidental command cannot remove your login account.
