package main

import (
	"io"
	"log"
	"sort"
	"bytes"
	"context"
	"io/ioutil"
	"html/template"
	"encoding/json"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

type PageData struct {
	Title string
	Api string
	ImgId int
	ImgPage int
	PageList []int
	ImgList []ImgData
}

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

type ConstantData struct {
	Api       string `json:"api"`
	Title     string `json:"title"`
	Threshold int    `json:"threshold"`
}

type Response events.APIGatewayProxyResponse

const imgTableName     string = "img"
const tokenTableName   string = "token"
const layout           string = "2006-01-02 15:04"

func HandleRequest(ctx context.Context, request events.APIGatewayProxyRequest) (Response, error) {
	templates := template.New("tmp")
	var dat PageData
	var err error
	funcMap := template.FuncMap{
		"safehtml": func(text string) template.HTML { return template.HTML(text) },
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"mul": func(a, b int) int { return a * b },
		"div": func(a, b int) int { return a / b },
	}
	buf := new(bytes.Buffer)
	fw := io.Writer(buf)
	jsonString, _ := ioutil.ReadFile("constant/constant.json")
	constant := new(ConstantData)
	json.Unmarshal(jsonString, constant)
	dat.Api = constant.Api
	if err != nil {
		log.Print(err)
		panic(err)
	}
	dat.Title = constant.Title
	dat.ImgId = 0
	dat.ImgPage = 1
	dat.ImgList, err = getImgList()
	sort.Slice(dat.ImgList, func(i, j int) bool { return dat.ImgList[i].Updated > dat.ImgList[j].Updated })
	templates = template.Must(template.New("").Funcs(funcMap).ParseFiles("templates/index.html", "templates/view.html", "templates/header.html", "templates/footer.html", "templates/pager.html", "templates/image_list.html"))
	if err != nil {
		log.Print(err)
		panic(err)
	}
	if e := templates.ExecuteTemplate(fw, "base", dat); e != nil {
		log.Fatal(e)
	} else {
		log.Print(request.RequestContext.Identity.SourceIP)
	}
	res := Response{
		StatusCode:      200,
		IsBase64Encoded: false,
		Body:            string(buf.Bytes()),
		Headers: map[string]string{
			"Content-Type": "text/html",
		},
	}
	return res, nil
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

func getImgList()([]ImgData, error)  {
	var imgList []ImgData
	result, err := scan(imgTableName, expression.Equal(expression.Name("status"), expression.Value(0)))
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

func main() {
	lambda.Start(HandleRequest)
}
