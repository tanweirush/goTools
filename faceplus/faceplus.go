package faceplus

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"strings"

	"github.com/sunreaver/gotools/http"
)

// Result define FaceplusResult
type Result struct {
	ID        string `json:"image_id"`
	RequestID string `json:"request_id"`
	TimeUsed  int    `json:"time_used"`
	Err       string `json:"error_message"`
	Faces     []Face `json:"faces"`
}

// Face define Face
type Face struct {
	Attribute AttributeValue `json:"attributes"`
	Token     string         `json:"face_token"`
}

// AttributeValue define AttributeValue
type AttributeValue struct {
	Age struct {
		Value int `json:"value"`
	} `json:"age"`
	Beauty struct {
		Female float64 `json:"female_score"`
		Male   float64 `json:"male_score"`
	} `json:"beauty"`
	Ethnicity struct {
		Value string `json:"value"` //Asian,White,Black
	} `json:"ethnicity"`
	Gender struct {
		Value string `json:"value"` //Male,Female
	} `json:"gender"`
	Blur struct {
		Blurness struct {
			Value     float64 `json:"value"`
			Threshold float64 `json:"threshold"`
		} `json:"blurness"`
	} `json:"blur"`
	Quality struct {
		Threshold float64 `json:"threshold"`
		Value     float64 `json:"value"`
	} `json:"facequality"`
}

// Verification define 接口验证
type Verification struct {
	Key    string
	Secret string
}

// Process will Process
func Process(file, name string, age int, v *Verification) (noNeed bool, reason string) {
	//打开文件句柄操作
	fh, err := os.Open(file)
	if err != nil {
		return true, "opening file"
	}
	defer fh.Close()

	return ProcessWithData(name, fh, age, v)
}

// ProcessWithData will ProcessWithData
func ProcessWithData(name string, fileData io.Reader, age int, v *Verification) (noNeed bool, reason string) {
	re, e := postData(name, "https://api-cn.faceplusplus.com/facepp/v3/detect", fileData, v)
	if e != nil {
		return false, e.Error()
	}

	weights := -0.000001
	for _, face := range re.Faces {
		if face.Attribute.Gender.Value == "Male" {
			reason += fmt.Sprint("男性")
			weights -= 0.8
		}

		// 小15岁加一分
		a := age - face.Attribute.Age.Value
		weights += float64(a) / 15

		// 漂亮20分加一分
		b := face.Attribute.Beauty.Female - 70.0
		weights += b / 20

		if face.Attribute.Ethnicity.Value != "Asian" {
			weights -= 0.9
		}

		if blur := face.Attribute.Blur.Blurness.Value - face.Attribute.Blur.Blurness.Threshold; blur > 0.0 {
			weights -= 0.015 * blur
		}

		if qua := face.Attribute.Quality.Value - face.Attribute.Quality.Threshold; qua > 0.0 {
			weights -= 0.25 * qua
		}
	}

	noNeed = weights < 0.0
	reason = fmt.Sprintf("%0.1f", weights)
	return
}

func postData(name, uri string, fileData io.Reader, v *Verification) (result *Result, err error) {

	if v == nil {
		return nil, errors.New("no verification")
	} else if fileData == nil {
		return nil, errors.New("no data")
	}
	defer func() {
		recover()
	}()

	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)
	apiKey, _ := bodyWriter.CreateFormField("api_key")
	io.Copy(apiKey, strings.NewReader(v.Key))
	apiSec, _ := bodyWriter.CreateFormField("api_secret")
	io.Copy(apiSec, strings.NewReader(v.Secret))
	re, _ := bodyWriter.CreateFormField("return_attributes")
	io.Copy(re, strings.NewReader("age,facequality,ethnicity,beauty,gender,blur"))

	//关键的一步操作
	fileWriter, err := bodyWriter.CreateFormFile("image_file", name)
	if err != nil {
		fmt.Println("error writing to buffer")
		return nil, err
	}

	//iocopy
	_, err = io.Copy(fileWriter, fileData)
	if err != nil {
		return nil, err
	}
	contentType := bodyWriter.FormDataContentType()
	bodyWriter.Close()
	resp, err := http.Post(uri,
		map[string]string{"Content-Type": contentType},
		bodyBuf)
	if err != nil {
		return nil, err
	}
	result = &Result{}
	if e := json.Unmarshal(resp.GetContent(), result); e != nil {
		return nil, errors.New("接口返回错误")
	} else if result.Err != "" {
		return nil, errors.New(result.Err)
	}

	return result, nil
}
