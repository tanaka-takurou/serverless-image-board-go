package main

import (
	"log"
	"time"
	"bytes"
	"errors"
	"strings"
	"strconv"
	"context"
	"path/filepath"
	"encoding/json"
	"encoding/base64"
	"golang.org/x/crypto/bcrypt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
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

const imgTableName     string = "img"
const tokenTableName   string = "token"
const bucketName       string = "image-upload"
const bucketRegion     string = "ap-northeast-1"
const bucketPath       string = "img"
const layout           string = "2006-01-02 15:04"

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
				if checkToken(t) {
					if v, ok := d["filename"]; ok {
						if w, ok := d["filedata"]; ok {
							err = uploadImage(v, w)
							deleteToken(t)
						}
					}
				}
			}
		case "puttoken" :
			hash, err := bcrypt.GenerateFromPassword([]byte("salt2"), bcrypt.DefaultCost)
			if err == nil {
				err = putToken(string(hash))
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

func scan(tableName string, filt expression.ConditionBuilder)(*dynamodb.ScanOutput, error)  {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	svc := dynamodb.New(sess)
	expr, err := expression.NewBuilder().WithFilter(filt).Build()
	if err != nil {
		return nil, err
	}
	params := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(tableName),
	}
	return svc.Scan(params)
}

func put(tableName string, av map[string]*dynamodb.AttributeValue) error {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	svc := dynamodb.New(sess)
	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(tableName),
	}
	_, err := svc.PutItem(input)
	return err
}

func putToken(token string) error {
	t := time.Now()
	item := TokenData {
		Token: token,
		Created: t.Format(layout),
	}
	av, err := dynamodbattribute.MarshalMap(item)
	if err != nil {
		return err
	}
	err = put(tokenTableName, av)
	if err != nil {
		return err
	}
	return nil
}

func get(tableName string, key map[string]*dynamodb.AttributeValue, att string)(*dynamodb.GetItemOutput, error) {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	svc := dynamodb.New(sess)
	input := &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: key,
		AttributesToGet: []*string{
			aws.String(att),
		},
		ConsistentRead: aws.Bool(true),
		ReturnConsumedCapacity: aws.String("NONE"),
	}
	return svc.GetItem(input)
}

func update(tableName string, an map[string]*string, av map[string]*dynamodb.AttributeValue, key map[string]*dynamodb.AttributeValue, updateExpression string) error {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	svc := dynamodb.New(sess)
	input := &dynamodb.UpdateItemInput{
		ExpressionAttributeNames: an,
		ExpressionAttributeValues: av,
		TableName: aws.String(tableName),
		Key: key,
		ReturnValues:     aws.String("UPDATED_NEW"),
		UpdateExpression: aws.String(updateExpression),
	}

	_, err := svc.UpdateItem(input)
	return err
}

func updateImg(img_id int, url string, updated string) error {
	an := map[string]*string{
		"#u": aws.String("url"),
		"#d": aws.String("updated"),
	}
	av := map[string]*dynamodb.AttributeValue{
		":u": {
			S: aws.String(url),
		},
		":d": {
			S: aws.String(updated),
		},
	}
	key := map[string]*dynamodb.AttributeValue{
		"img_id": {
			N: aws.String(strconv.Itoa(img_id)),
		},
	}
	updateExpression := "set #u = :u, #d = :d"

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	svc := dynamodb.New(sess)
	input := &dynamodb.UpdateItemInput{
		ExpressionAttributeNames: an,
		ExpressionAttributeValues: av,
		TableName: aws.String(imgTableName),
		Key: key,
		ReturnValues:     aws.String("UPDATED_NEW"),
		UpdateExpression: aws.String(updateExpression),
	}

	_, err := svc.UpdateItem(input)
	if err != nil {
		return err
	}
	return nil
}

func getImgCount()(*int64, error)  {
	result, err := scan(imgTableName, expression.NotEqual(expression.Name("status"), expression.Value(-1)))
	if err != nil {
		return nil, err
	}
	return result.ScannedCount, nil
}

func putImg(url string) error {
	t := time.Now()
	count, err := getImgCount()
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
	err = put(imgTableName, av)
	if err != nil {
		return err
	}
	return nil
}

func checkToken(token string) bool {
	item := struct {Token string `json:"token"`}{token}
	av, err := dynamodbattribute.MarshalMap(item)
	if err != nil {
		return false
	}
	res, err := get(tokenTableName, av, "token")
	if err == nil && res.Item != nil{
		return true
	}
	return false
}

func delete(tableName string, key map[string]*dynamodb.AttributeValue) error {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	svc := dynamodb.New(sess)
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: key,
	}

	_, err := svc.DeleteItem(input)
	return err
}

func deleteToken(token string) error {
	key := map[string]*dynamodb.AttributeValue{
		"token": {
			S: aws.String(token),
		},
	}
	err := delete(tokenTableName, key)
	if err != nil {
		return err
	}
	return nil
}

func uploadImage(filename string, filedata string) error {
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
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(bucketRegion)},
	)
	if err != nil {
		log.Print(err)
		return err
	}
	uploader := s3manager.NewUploader(sess)
	_, err = uploader.Upload(&s3manager.UploadInput{
		ACL: aws.String("public-read"),
		Bucket: aws.String(bucketName),
		Key: aws.String(bucketPath + "/" + filename),
		Body: bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		log.Print(err)
		return err
	}
	putImg("https://" + bucketName + ".s3-" + bucketRegion + ".amazonaws.com/" + bucketPath + "/" + filename)
	return nil
}

func main() {
	lambda.Start(HandleRequest)
}
