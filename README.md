# serverless-image-board kit
Simple kit for serverless image board using AWS Lambda.


## Dependence
- aws-lambda-go
- aws-sdk-go


## Requirements
- AWS (Lambda, API Gateway, DynamoDB, S3)
- aws-cli
- golang environment


## DynamoDB Setting
- make 2 table
```
Table Name: img
Partition key Name: img_id
Partition key Type: Number

Table Name: token
Partition key Name: token
Partition key Type: String
```

## S3 Setting
 - Create a publicly accessible bucket
 - Open api/main.go and edit 'bucketName', 'bucketRegion' and 'bucketPath'.

## Usage

### Edit View
##### HTML
- Edit templates/index.html

##### CSS
- Edit static/css/main.css

##### Javascript
- Edit static/js/main.js

##### Image
- Add image file into static/img/
- Edit templates/index.html like as 'enter.jpg'.

### Deploy
Open scripts/deploy.sh and edit 'your_function_name'.

Open api/scripts/deploy.sh and edit 'your_api_function_name'.

Open constant/constant.json and edit 'your_api_url'.


Then run this command.

```
$ sh scripts/deploy.sh
$ cd api
$ sh scripts/deploy.sh
```
