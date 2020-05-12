package controller

import (
	"log"
	"sort"
	"strconv"
	"net/http"
	"io/ioutil"
	"html/template"
	"encoding/json"
)

type PageData struct {
	Title string
	ImgId int
	ImgPage int
	PageList []int
	ImgList []ImgData
	TokenList []TokenData
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
	Title string `json:"title"`
}

const imgTableName     string = "img"
const tokenTableName   string = "token"
const bucketName       string = "image-upload"
const bucketRegion     string = "ap-northeast-1"
const bucketPath       string = "img"

func HttpHandler(w http.ResponseWriter, request *http.Request){
	var img_id int
	var s_img_id string
	var dat PageData
	var err error
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		log.Fatal(err)
	}
	d := make(map[string]string)
	json.Unmarshal(body, &d)
	q := request.URL.Query()
	if q != nil && q["img_id"] != nil {
		s_img_id = q["img_id"][0]
	}
	funcMap := template.FuncMap{
		"safehtml": func(text string) template.HTML { return template.HTML(text) },
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"mul": func(a, b int) int { return a * b },
		"div": func(a, b int) int { return a / b },
	}
	templates := template.New("tmp")
	if v, ok := d["action"]; ok {
		switch v {
		case "uploadimg" :
			log.Print("Upload Img.")
			if v, ok := d["filename"]; ok {
				if w, ok := d["filedata"]; ok {
					err = uploadImage(v, w)
				}
			}
		case "updateimg" :
			log.Print("Update Img.")
			img_id, err = strconv.Atoi(d["img_id"])
			if err == nil {
				status_, err_ := strconv.Atoi(d["status"])
				if err_ == nil {
					err = updateImgStatus(img_id, status_, "status")
				}
			}
		case "deletetoken" :
			log.Print("Delete Token.")
			if v, ok := d["token"]; ok {
				err = deleteToken(v)
			}
		}
	}
	if err != nil {
		log.Print(err)
		panic(err)
	}
	if len(s_img_id) > 0 {
		dat.ImgId = 1
		dat.ImgPage = 1
		dat.ImgList, err = getImgList()
		dat.TokenList, _ = getTokenList()
		sort.Slice(dat.ImgList, func(i, j int) bool { return dat.ImgList[i].Updated < dat.ImgList[j].Updated })
		templates = template.Must(template.New("").Funcs(funcMap).ParseFiles("templates/index.html", "templates/view.html", "templates/header.html", "templates/footer.html", "templates/pager.html", "templates/image_list.html", "templates/token.html"))
		if err != nil {
			log.Print(err)
			panic(err)
		}
	} else {
		dat.ImgId = 0
		dat.ImgPage = 1
		dat.ImgList, err = getImgList()
		dat.TokenList, _ = getTokenList()
		sort.Slice(dat.ImgList, func(i, j int) bool { return dat.ImgList[i].Updated < dat.ImgList[j].Updated })
		templates = template.Must(template.New("").Funcs(funcMap).ParseFiles("templates/index.html", "templates/view.html", "templates/header.html", "templates/footer.html", "templates/pager.html", "templates/image_list.html", "templates/token.html"))
		if err != nil {
			log.Print(err)
			panic(err)
		}
	}
	dat.Title = "Sample Management Page"
	err = templates.ExecuteTemplate(w, "base", dat)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
