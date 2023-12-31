#!/bin/bash
API_NAME='your_api'
FUNCTION_NAME='your_function_name'
API_FUNCTION_NAME='your_api_function_name'
IMG_TABLE_NAME='sample_img'
TOKEN_TABLE_NAME='sample_token'
BUCKET_NAME='sample-bucket-'`date "+%Y%m%d"`
REGION='ap-northeast-1'
STAGE_NAME='your_stage'
ROLE_NAME='your-lambda-role'
aws iam create-role --role-name $ROLE_NAME --path /service-role/ --assume-role-policy-document file://`pwd`/`dirname $0`/policy.json
ROLE_ARN=`aws iam get-role --role-name $ROLE_NAME | jq -r  .'Role.Arn'`
aws iam attach-role-policy --role-name $ROLE_NAME --policy-arn "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
aws iam attach-role-policy --role-name $ROLE_NAME --policy-arn "arn:aws:iam::aws:policy/AmazonDynamoDBFullAccess"
cd `dirname $0`/../
touch constant/constant.json

echo 'Creating template...'
`dirname $0`/create_template.sh

echo 'Creating function.zip...'
`dirname $0`/create_function.sh

echo 'Create Lambda-Function...'
cd `dirname $0`/../
aws lambda create-function \
	--function-name $FUNCTION_NAME \
	--runtime provided.al2 \
	--role $ROLE_ARN \
	--handler bootstrap \
	--zip-file fileb://`pwd`/function.zip \
	--region $REGION > tmp.txt
TMP_ARN=$(jq .FunctionArn tmp.txt)
FUNCTION_ARN=${TMP_ARN//\"/}
aws sts get-caller-identity > tmp.txt
TMP_ID=$(jq .Account tmp.txt)
ACCOUNT_ID=${TMP_ID//\"/}

`dirname $0`/../api/scripts/init.sh
cd `dirname $0`/../api
TMP_ARN=$(jq .FunctionArn tmp.txt)
API_FUNCTION_ARN=${TMP_ARN//\"/}

echo 'Create Dynamo-Table...'

aws dynamodb create-table --table-name $IMG_TABLE_NAME --attribute-definitions \
	AttributeName=img_id,AttributeType=N \
	--key-schema AttributeName=img_id,KeyType=HASH \
	--provisioned-throughput ReadCapacityUnits=3,WriteCapacityUnits=3

aws dynamodb create-table --table-name $TOKEN_TABLE_NAME --attribute-definitions \
	AttributeName=token,AttributeType=S \
	--key-schema AttributeName=token,KeyType=HASH \
	--provisioned-throughput ReadCapacityUnits=3,WriteCapacityUnits=3

cd `dirname $0`/../
echo 'Create API...'
aws apigateway create-rest-api \
	--name $API_NAME \
	--description 'API for lambda-function' \
	--region $REGION \
	--endpoint-configuration '{ "types": ["REGIONAL"] }' > tmp.txt
TMP_ID=$(jq .id tmp.txt)
REST_API_ID=${TMP_ID//\"/}
aws apigateway get-resources \
	--rest-api-id $REST_API_ID \
	--region $REGION > tmp.txt
TMP_ID=$(jq .items[0].id tmp.txt)
RESOURCE_ID=${TMP_ID//\"/}
aws apigateway put-method \
	--rest-api-id $REST_API_ID \
	--resource-id $RESOURCE_ID \
	--http-method GET \
	--authorization-type "NONE" \
	--region $REGION
aws apigateway put-integration \
	--rest-api-id $REST_API_ID \
	--resource-id $RESOURCE_ID \
	--http-method GET \
	--integration-http-method POST \
	--type AWS_PROXY \
	--uri arn:aws:apigateway:$REGION:lambda:path/2015-03-31/functions/$FUNCTION_ARN/invocations \
	--region $REGION
aws apigateway put-method-response \
	--rest-api-id $REST_API_ID \
	--resource-id $RESOURCE_ID \
	--http-method GET \
	--status-code 200 \
	--response-models '{"text/html": "Empty"}'

aws apigateway create-resource \
	--rest-api-id $REST_API_ID \
	--parent-id  $RESOURCE_ID \
	--path-part api \
	--region $REGION > tmp.txt
TMP_ID=$(jq .id tmp.txt)
RESOURCE_API_ID=${TMP_ID//\"/}
aws apigateway put-method \
	--rest-api-id $REST_API_ID \
	--resource-id $RESOURCE_API_ID \
	--http-method POST \
	--authorization-type "NONE" \
	--region $REGION
aws apigateway put-integration \
	--rest-api-id $REST_API_ID \
	--resource-id $RESOURCE_API_ID \
	--http-method POST \
	--integration-http-method POST \
	--type AWS_PROXY \
	--uri arn:aws:apigateway:$REGION:lambda:path/2015-03-31/functions/$API_FUNCTION_ARN/invocations \
	--region $REGION
aws apigateway put-method-response \
	--rest-api-id $REST_API_ID \
	--resource-id $RESOURCE_API_ID \
	--http-method POST \
	--status-code 200 \
	--response-models '{"application/json": "Empty"}'
aws apigateway create-deployment \
	--rest-api-id $REST_API_ID \
	--stage-name $STAGE_NAME
aws lambda add-permission \
	--function-name $FUNCTION_NAME \
	--statement-id apigateway-test \
	--action lambda:InvokeFunction \
	--principal apigateway.amazonaws.com \
	--source-arn "arn:aws:execute-api:$REGION:$ACCOUNT_ID:$REST_API_ID/*/GET/"
aws lambda add-permission \
	--function-name $API_FUNCTION_NAME \
	--statement-id apigateway-api-test \
	--action lambda:InvokeFunction \
	--principal apigateway.amazonaws.com \
	--source-arn "arn:aws:execute-api:$REGION:$ACCOUNT_ID:$REST_API_ID/*/POST/api"
cd `dirname $0`/../
echo "{\"title\":\"Sample Image Board\", \"threshold\": 10, \"api\":\"https://$REST_API_ID.execute-api.$REGION.amazonaws.com/$STAGE_NAME/api\", \"imgTableName\":\"$IMG_TABLE_NAME\"}" > constant/constant.json
echo 'Creating function.zip...'
`dirname $0`/create_function.sh
echo 'Updating Lambda-Function...'
cd `dirname $0`/../
aws lambda update-function-code \
	--profile default \
	--function-name $FUNCTION_NAME \
	--zip-file fileb://`pwd`/function.zip \
	--cli-connect-timeout 6000 \
	--publish
rm tmp.txt
echo 'Finish.'
echo "https://$REST_API_ID.execute-api.$REGION.amazonaws.com/$STAGE_NAME/"
