package main

import (
	"io"
	"os"
	"log"
	"sort"
	"bytes"
	"context"
	"html/template"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
)

type PageData struct {
	Title string
	ApiPath string
	ImgId int
	ImgPage int
	PageList []int
	ImgList []ImgData
}

type ImgData struct {
	Img_Id  int    `dynamodbav:"img_id"`
	Status  int    `dynamodbav:"status"`
	Url     string `dynamodbav:"url"`
	Updated string `dynamodbav:"updated"`
}

type Response events.APIGatewayProxyResponse

var dynamodbClient *dynamodb.Client

const layout string = "2006-01-02 15:04"
const title  string = "Sample Image Board"

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
	dat.ApiPath = os.Getenv("API_PATH")
	if err != nil {
		log.Print(err)
		panic(err)
	}
	dat.Title = title
	dat.ImgId = 0
	dat.ImgPage = 1
	dat.ImgList, err = getImgList(ctx, os.Getenv("IMG_TABLE_NAME"))
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

func scan(ctx context.Context, tableName string, filt expression.ConditionBuilder, proj expression.ProjectionBuilder)(*dynamodb.ScanOutput, error)  {
	if dynamodbClient == nil {
		dynamodbClient = dynamodb.NewFromConfig(getConfig(ctx))
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
	res, err := dynamodbClient.Scan(ctx, input)
	return res, err
}

func getImgList(ctx context.Context, imgTableName string)([]ImgData, error)  {
	var imgList []ImgData
	filt := expression.Equal(expression.Name("status"), expression.Value(0))
	proj := expression.NamesList(expression.Name("img_id"), expression.Name("status"), expression.Name("url"), expression.Name("updated"))
	result, err := scan(ctx, imgTableName, filt, proj)
	if err != nil {
		return nil, err
	}
	for _, i := range result.Items {
		item := ImgData{}
		err = attributevalue.UnmarshalMap(i, &item)
		if err != nil {
			return nil, err
		}
		imgList = append(imgList, item)
	}
	return imgList, nil
}

func getConfig(ctx context.Context) aws.Config {
	var err error
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(os.Getenv("REGION")))
	if err != nil {
		log.Print(err)
	}
	return cfg
}

func main() {
	lambda.Start(HandleRequest)
}
