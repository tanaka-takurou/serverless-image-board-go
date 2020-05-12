package controller

import (
	"log"
	"time"
	"bytes"
	"errors"
	"strings"
	"strconv"
	"path/filepath"
	"encoding/base64"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

const layout = "2006-01-02 15:04"

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

func getImgList()([]ImgData, error)  {
	var imgList []ImgData
	result, err := scan(imgTableName, expression.NotEqual(expression.Name("status"), expression.Value(-1)))
	if err != nil {
		return nil, err
	}
	for _, i := range result.Items {
		item := ImgData{}
		err = dynamodbattribute.UnmarshalMap(i, &item)
		if err != nil {
			return nil, err
		}
		imgList = append(imgList, item)
	}
	return imgList, nil
}

func getTokenList()([]TokenData, error)  {
	var tokenList []TokenData
	result, err := scan(tokenTableName, expression.NotEqual(expression.Name("token"), expression.Value("")))
	if err != nil {
		return nil, err
	}
	for _, i := range result.Items {
		item := TokenData{}
		err = dynamodbattribute.UnmarshalMap(i, &item)
		if err != nil {
			return nil, err
		}
		tokenList = append(tokenList, item)
	}
	return tokenList, nil
}

func getImgCount()(*int64, error)  {
	result, err := scan(imgTableName, expression.NotEqual(expression.Name("status"), expression.Value(-1)))
	if err != nil {
		return nil, err
	}
	return result.ScannedCount, nil
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

func updateImgStatus(img_id int, value int, name string) error {
	an := map[string]*string{
		"#s": aws.String(name),
	}
	av := map[string]*dynamodb.AttributeValue{
		":new": {
			N: aws.String(strconv.Itoa(value)),
		},
	}
	key := map[string]*dynamodb.AttributeValue{
		"img_id": {
			N: aws.String(strconv.Itoa(img_id)),
		},
	}
	updateExpression := "set #s = :new"
	err := update(imgTableName, an, av, key, updateExpression)
	if err != nil {
		return err
	}
	return nil
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
