package main

import (
	"os"
	"log"
	"time"
	"bytes"
	"errors"
	"strings"
	"context"
	"path/filepath"
	"encoding/json"
	"encoding/base64"
	"golang.org/x/crypto/bcrypt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/s3manager"
)

type ImgData struct {
	Img_Id  int    `json:"img_id"`
	Status  int    `json:"status"`
	Url     string `json:"url"`
	Updated string `json:"updated"`
}

type TokenData struct {
	Token     string `json:"token"`
	Created   string `json:"created"`
}

type TokenResponse struct {
	Token     string `json:"token"`
}

type Response events.APIGatewayProxyResponse

var cfg aws.Config
var dynamodbClient *dynamodb.Client

const layout       string = "2006-01-02 15:04"
const layout2      string = "20060102150405"

func HandleRequest(ctx context.Context, request events.APIGatewayProxyRequest) (Response, error) {
	var jsonBytes []byte
	var err error
	d := make(map[string]string)
	json.Unmarshal([]byte(request.Body), &d)
	if v, ok := d["action"]; ok {
		switch v {
		case "uploadimg" :
			log.Print("Upload Img.")
			if t, ok := d["token"]; ok {
				if checkToken(ctx, os.Getenv("TOKEN_TABLE_NAME"), t) {
					if v, ok := d["filename"]; ok {
						if w, ok := d["filedata"]; ok {
							err = uploadImage(ctx, os.Getenv("IMG_TABLE_NAME"), os.Getenv("BUCKET_NAME"), v, w)
							deleteToken(ctx, os.Getenv("TOKEN_TABLE_NAME"), t)
						}
					}
				}
			}
		case "puttoken" :
			hash, err := bcrypt.GenerateFromPassword([]byte("salt2"), bcrypt.DefaultCost)
			if err == nil {
				err = putToken(ctx, os.Getenv("TOKEN_TABLE_NAME"), string(hash))
				if err == nil {
					jsonBytes, err = json.Marshal(TokenResponse{Token:string(hash)})
				}
			}
		}
	}
	if err != nil {
		return Response{}, err
	} else {
		log.Print(request.RequestContext.Identity.SourceIP)
	}
	responseBody := ""
	if len(jsonBytes) > 0 {
		responseBody = string(jsonBytes)
	}
	return Response {
		StatusCode: 200,
		Body: responseBody,
	}, nil
}

func scan(ctx context.Context, tableName string, filt expression.ConditionBuilder, proj expression.ProjectionBuilder)(*dynamodb.ScanOutput, error)  {
	if dynamodbClient == nil {
		dynamodbClient = dynamodb.New(cfg)
	}
	expr, err := expression.NewBuilder().WithFilter(filt).WithProjection(proj).Build()
	if err != nil {
		return nil, err
	}
	input := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(tableName),
	}
	req := dynamodbClient.ScanRequest(input)
	res, err := req.Send(ctx)
	return res.ScanOutput, err
}

func put(ctx context.Context, tableName string, av map[string]dynamodb.AttributeValue) error {
	if dynamodbClient == nil {
		dynamodbClient = dynamodb.New(cfg)
	}
	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(tableName),
	}
	req := dynamodbClient.PutItemRequest(input)
	_, err := req.Send(ctx)
	return err
}

func putToken(ctx context.Context, tokenTableName string, token string) error {
	t := time.Now()
	item := TokenData {
		Token: token,
		Created: t.Format(layout),
	}
	av, err := dynamodbattribute.MarshalMap(item)
	if err != nil {
		return err
	}
	err = put(ctx, tokenTableName, av)
	if err != nil {
		return err
	}
	return nil
}

func get(ctx context.Context, tableName string, key map[string]dynamodb.AttributeValue, att string)(*dynamodb.GetItemOutput, error) {
	if dynamodbClient == nil {
		dynamodbClient = dynamodb.New(cfg)
	}
	input := &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: key,
		AttributesToGet: []string{att},
		ConsistentRead: aws.Bool(true),
		ReturnConsumedCapacity: dynamodb.ReturnConsumedCapacityNone,
	}
	req := dynamodbClient.GetItemRequest(input)
	res, err := req.Send(ctx)
	return res.GetItemOutput, err
}

func getImgCount(ctx context.Context, imgTableName string)(*int64, error)  {
	filt := expression.NotEqual(expression.Name("status"), expression.Value(-1))
	proj := expression.NamesList(expression.Name("img_id"), expression.Name("status"), expression.Name("url"), expression.Name("updated"))
	result, err := scan(ctx, imgTableName, filt, proj)
	if err != nil {
		return nil, err
	}
	return result.ScannedCount, nil
}

func putImg(ctx context.Context, imgTableName string, url string) error {
	t := time.Now()
	count, err := getImgCount(ctx, imgTableName)
	if err != nil {
		return err
	}
	item := ImgData {
		Img_Id: int(*count) + 1,
		Status: 0,
		Url: url,
		Updated: t.Format(layout),
	}
	av, err := dynamodbattribute.MarshalMap(item)
	if err != nil {
		return err
	}
	err = put(ctx, imgTableName, av)
	if err != nil {
		return err
	}
	return nil
}

func checkToken(ctx context.Context, tokenTableName string, token string) bool {
	item := struct {Token string `json:"token"`}{token}
	av, err := dynamodbattribute.MarshalMap(item)
	if err != nil {
		return false
	}
	res, err := get(ctx, tokenTableName, av, "token")
	if err == nil && res.Item != nil{
		return true
	}
	return false
}

func delete(ctx context.Context, tableName string, key map[string]dynamodb.AttributeValue) error {
	if dynamodbClient == nil {
		dynamodbClient = dynamodb.New(cfg)
	}
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: key,
	}

	req := dynamodbClient.DeleteItemRequest(input)
	_, err := req.Send(ctx)
	return err
}

func deleteToken(ctx context.Context, tokenTableName string, token string) error {
	key := map[string]dynamodb.AttributeValue{
		"token": {
			S: aws.String(token),
		},
	}
	err := delete(ctx, tokenTableName, key)
	if err != nil {
		return err
	}
	return nil
}

func uploadImage(ctx context.Context, imgTableName string, bucketName string, filename string, filedata string) error {
	t := time.Now()
	b64data := filedata[strings.IndexByte(filedata, ',')+1:]
	data, err := base64.StdEncoding.DecodeString(b64data)
	if err != nil {
		log.Print(err)
		return err
	}
	extension := filepath.Ext(filename)
	var contentType string

	switch extension {
	case ".jpg":
		contentType = "image/jpeg"
	case ".jpeg":
		contentType = "image/jpeg"
	case ".gif":
		contentType = "image/gif"
	case ".png":
		contentType = "image/png"
	default:
		return errors.New("this extension is invalid")
	}
	filename_ := string([]rune(filename)[:(len(filename) - len(extension))]) + t.Format(layout2) + extension
	uploader := s3manager.NewUploader(cfg)
	_, err = uploader.Upload(&s3manager.UploadInput{
		ACL: s3.ObjectCannedACLPublicRead,
		Bucket: aws.String(bucketName),
		Key: aws.String(filename_),
		Body: bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		log.Print(err)
		return err
	}
	putImg(ctx, imgTableName, "https://" + bucketName + ".s3-" + os.Getenv("REGION") + ".amazonaws.com/" + filename_)
	return nil
}

func init() {
	var err error
	cfg, err = external.LoadDefaultAWSConfig()
	cfg.Region = os.Getenv("REGION")
	if err != nil {
		log.Print(err)
	}
}

func main() {
	lambda.Start(HandleRequest)
}
