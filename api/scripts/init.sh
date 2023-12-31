#!/bin/bash
if [ -z "$API_FUNCTION_NAME" ]; then
API_FUNCTION_NAME='your_api_function_name'
fi
if [ -z "$IMG_TABLE_NAME" ]; then
IMG_TABLE_NAME='sample_img'
fi
if [ -z "$TOKEN_TABLE_NAME" ]; then
TOKEN_TABLE_NAME='sample_token'
fi
if [ -z "$BUCKET_NAME" ]; then
BUCKET_NAME='sample-bucket-'`date "+%Y%m%d"`
fi
API_ROLE_NAME='your-api-lambda-role'
REGION='ap-northeast-1'
aws iam create-role --role-name $API_ROLE_NAME --path /service-role/ --assume-role-policy-document file://`pwd`/`dirname $0`/policy.json
API_ROLE_ARN=`aws iam get-role --role-name $API_ROLE_NAME | jq -r  .'Role.Arn'`
aws iam attach-role-policy --role-name $API_ROLE_NAME --policy-arn "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
aws iam attach-role-policy --role-name $API_ROLE_NAME --policy-arn "arn:aws:iam::aws:policy/AmazonDynamoDBFullAccess"
aws iam attach-role-policy --role-name $API_ROLE_NAME --policy-arn "arn:aws:iam::aws:policy/AmazonS3FullAccess"

echo 'Create S3 bucket...'
aws s3 mb s3://$BUCKET_NAME --region $REGION

cd `dirname $0`/../
echo "{\"imgTableName\":\"$IMG_TABLE_NAME\", \"tokenTableName\":\"$TOKEN_TABLE_NAME\", \"bucketName\":\"$BUCKET_NAME\"}" > constant/constant.json


echo 'Create API Lambda-Function...'
rm function.zip
rm bootstrap
zip -r9 function.zip constant
GOARCH=arm64 GOOS=linux CGO_ENABLED=0 go build -o bootstrap main.go
zip -g function.zip bootstrap
aws lambda create-function \
	--function-name $API_FUNCTION_NAME \
	--runtime provided.al2 \
	--role $API_ROLE_ARN \
	--handler bootstrap \
	--zip-file fileb://`pwd`/function.zip \
	--region $REGION > tmp.txt
