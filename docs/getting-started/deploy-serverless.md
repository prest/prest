---
date: 2020-11-10T15:21:22-07:00
title: Deploy Serverless
type: homepage
menu:
  getting-started:
    parent: "getting-started"
weight: 2
---

Here are all the ways you can install pREST, choose one that best fits your needs.

## Index

1. [AWS Lambda](/getting-started/deploy-serverless/#aws-lambda)
1. [LambCI](/getting-started/deploy-serverless/#lambci)

### AWS Lambda

Per [AWS documentation](https://docs.aws.amazon.com/lambda/latest/dg/golang-package.html),

```sh
zip function.zip prestd
aws lambda create-function --function-name my-function --runtime go1.x \
    --zip-file fileb://function.zip --handler prestd \
    --environment Variables='{PREST_PG_USER=postgres,PREST_PG_PASS=mysecretpassword,PREST_PG_DATABASE=prest,PREST_PG_PORT=5432,PREST_HTTP_PORT=3010,PREST_LAMBDA_MODE=true}' \
    --role arn:aws:iam::123456789012:role/execution_role
```

The Lambda will consume [API Gateway events](https://github.com/awsdocs/aws-lambda-developer-guide/blob/master/sample-apps/nodejs-apig/event.json).

### LambCI

```
docker run --rm -d \
    --name postgres \
    -e POSTGRES_USER=postgres \
    -e POSTGRES_PASSWORD=mysecretpassword \
    -e POSTGRES_DB=prest \
    -p 5432:5432 \
    postgres

docker run --rm \
    -v "$PWD":/var/task:ro,delegated,z \
    -e DOCKER_LAMBDA_STAY_OPEN=1 \
    -e PREST_PG_USER=postgres \
    -e PREST_PG_PASS=mysecretpassword \
    -e PREST_PG_DATABASE=prest \
    -e PREST_PG_PORT=5432 \
    -e PREST_HTTP_PORT=3010 \
    -e PREST_LAMBDA_MODE=true \
    -p 9001:9001 \
    --net=host \
    lambci/lambda:go1.x prestd
```

Then test with a [API Gateway event](https://github.com/awsdocs/aws-lambda-developer-guide/blob/master/sample-apps/nodejs-apig/event.json),
```
$ aws --region us-east-1 \
    --endpoint http://localhost:9001 \
    --no-sign-request \
    lambda invoke --function-name myfunction \
        --payload '{"path": "/databases"}' /dev/stdout | jq
{
  "statusCode": 200,
  "headers": {
    "Content-Type": "application/json"
  },
  "multiValueHeaders": {
    "Content-Type": [
      "application/json"
    ]
  },
  "body": "[{\"datname\":\"postgres\"}, \n {\"datname\":\"prest\"}]"
}
{
  "StatusCode": 200,
  "ExecutedVersion": "$LATEST"
}
```
