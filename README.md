# serverless-image-board kit
Simple kit for serverless image board using AWS Lambda.


## Dependence
- aws-lambda-go
- aws-sdk-go-v2


## Requirements
- AWS (Lambda, API Gateway, DynamoDB, S3)
- aws-sam-cli
- golang environment


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
```bash
make clean build
AWS_PROFILE={profile} AWS_DEFAULT_REGION={region} make bucket={bucket} stack={stack name} deploy
```
