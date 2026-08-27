# Safe Backend Deployment With GitHub Actions And AWS SSM

This is the recommended backend deployment path for ClearDay.

```text
GitHub Actions -> AWS OIDC role -> S3 release bucket -> AWS Systems Manager -> EC2 -> clearday.service
```

This avoids public SSH from GitHub Actions. You do not store your EC2 private key in GitHub.

## Why This Is Safer

- GitHub receives short-lived AWS credentials through OIDC.
- The backend release package goes into a private S3 bucket.
- EC2 downloads the package using its own IAM instance profile.
- Systems Manager runs the deployment command on EC2.
- Port `22` does not need to be open to `0.0.0.0/0`.

## GitHub Variables

Open:

```text
clearday-backend -> Settings -> Secrets and variables -> Actions -> Variables
```

Add:

```text
AWS_REGION=ap-southeast-1
AWS_BACKEND_ROLE_TO_ASSUME=arn:aws:iam::YOUR_ACCOUNT_ID:role/clearday-backend-github-actions
AWS_BACKEND_DEPLOY_BUCKET=YOUR_BACKEND_DEPLOY_BUCKET
AWS_BACKEND_INSTANCE_ID=YOUR_EC2_INSTANCE_ID
```

Example instance id:

```text
i-0123456789abcdef0
```

You can find it in:

```text
AWS Console -> EC2 -> Instances -> select your backend instance -> Instance ID
```

## GitHub Secrets

For this SSM deployment flow, the backend repo does not need an SSH key secret.

You can remove this old secret if you already added it:

```text
VM_SSH_PRIVATE_KEY
```

## AWS Step 1: EC2 Instance Profile

Your EC2 instance needs an IAM instance profile with:

```text
AmazonSSMManagedInstanceCore
```

It also needs permission to read from the backend deploy bucket:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "s3:ListBucket",
      "Resource": "arn:aws:s3:::YOUR_BACKEND_DEPLOY_BUCKET"
    },
    {
      "Effect": "Allow",
      "Action": "s3:GetObject",
      "Resource": "arn:aws:s3:::YOUR_BACKEND_DEPLOY_BUCKET/*"
    }
  ]
}
```

If you use this repository's Terraform, it creates these permissions on the `clearday-ec2-profile` instance profile.

## AWS Step 2: Backend Deploy Bucket

Create a private S3 bucket for backend release packages.

Recommended name:

```text
clearday-reminderapp-backend-deploy
```

Keep:

```text
Block all public access: On
Bucket versioning: Off
Default encryption: Amazon S3 managed keys
```

If you use this repository's Terraform, it creates this bucket and automatically expires old release packages after 7 days.

## AWS Step 3: GitHub OIDC Role

Create a separate IAM role for the backend workflow:

```text
clearday-backend-github-actions
```

Trust only your backend repo:

```text
weihuan1007/clearday-backend
```

Audience:

```text
sts.amazonaws.com
```

Attach permissions like this:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "s3:ListBucket",
      "Resource": "arn:aws:s3:::YOUR_BACKEND_DEPLOY_BUCKET"
    },
    {
      "Effect": "Allow",
      "Action": [
        "s3:PutObject",
        "s3:DeleteObject"
      ],
      "Resource": "arn:aws:s3:::YOUR_BACKEND_DEPLOY_BUCKET/*"
    },
    {
      "Effect": "Allow",
      "Action": "ssm:SendCommand",
      "Resource": [
        "arn:aws:ec2:ap-southeast-1:YOUR_ACCOUNT_ID:instance/YOUR_EC2_INSTANCE_ID",
        "arn:aws:ssm:ap-southeast-1::document/AWS-RunShellScript"
      ]
    },
    {
      "Effect": "Allow",
      "Action": [
        "ssm:DescribeInstanceInformation",
        "ssm:GetCommandInvocation",
        "ssm:ListCommandInvocations",
        "ssm:ListCommands"
      ],
      "Resource": "*"
    }
  ]
}
```

If you use this repository's Terraform, set these values in `terraform.tfvars`:

```hcl
backend_deploy_bucket_name = "clearday-reminderapp-backend-deploy"
github_backend_repository  = "weihuan1007/clearday-backend"
backend_ec2_instance_id    = "YOUR_EC2_INSTANCE_ID"
```

Then run:

```powershell
cd D:\reminder-app\repos\backend\deploy\terraform
terraform fmt
terraform plan
terraform apply
terraform output
```

Copy these outputs into GitHub variables:

```text
backend_deploy_bucket_name      -> AWS_BACKEND_DEPLOY_BUCKET
github_backend_deploy_role_arn  -> AWS_BACKEND_ROLE_TO_ASSUME
```

## AWS Step 4: EC2 SSM Agent And AWS CLI

SSH into EC2 one last time from your laptop and run:

```bash
sudo snap list amazon-ssm-agent || sudo snap install amazon-ssm-agent --classic
sudo snap start amazon-ssm-agent || true
sudo systemctl enable --now snap.amazon-ssm-agent.amazon-ssm-agent.service || true
aws --version
```

If `aws --version` fails, install AWS CLI v2:

```bash
sudo apt-get update
sudo apt-get install -y ca-certificates curl unzip
curl -fsSLo /tmp/awscliv2.zip https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip
rm -rf /tmp/aws
unzip -q /tmp/awscliv2.zip -d /tmp
sudo /tmp/aws/install --update
aws --version
```

## AWS Step 5: Check Systems Manager

Open:

```text
AWS Console -> Systems Manager -> Fleet Manager
```

Your EC2 instance should appear as a managed node.

If it does not appear:

- Check that the EC2 instance profile is attached.
- Check that the profile has `AmazonSSMManagedInstanceCore`.
- Restart SSM Agent.
- Wait a few minutes and refresh Fleet Manager.

## Security Group

For backend deployment through SSM:

```text
Port 22: your laptop IP /32 only, or closed after SSM is confirmed working
Port 8080: open only if CloudFront still needs to reach the backend directly
```

You do not need:

```text
TCP 22 from 0.0.0.0/0
```

## Deployment Flow

After setup, deploy backend by pushing to `main`:

```powershell
cd D:\reminder-app\repos\backend
git add .
git commit -m "Update backend"
git push
```

Then open:

```text
clearday-backend -> Actions -> Deploy Backend
```

The workflow should show:

```text
Test backend
Configure AWS credentials
Verify AWS and SSM access
Upload release to S3
Deploy on EC2 through SSM
Delete uploaded release package
```

