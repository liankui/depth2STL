package rembg

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"log/slog"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	nhttp "github.com/chaos-io/depth2STL/util/http"
)

const (
	BiRefNetModel = "BiRefNet"
	baseUrl       = "http://192.168.4.188:8188/"
	uploadUrl     = baseUrl + "api/upload/image"
	promptUrl     = baseUrl + "api/prompt"
)

//go:embed workflow.json
var workflowData string

type BiRefNetRemBG struct {
	imagePath string
	cli       nhttp.IClient
}

func NewBiRefNetRemBG(imagePath string) *BiRefNetRemBG {
	return &BiRefNetRemBG{
		imagePath: imagePath,
		cli:       nhttp.NewHTTPClient(),
	}
}

func (b *BiRefNetRemBG) Remove(ctx context.Context, img image.Image) (image.Image, error) {
	err := b.uploadImage(ctx)
	if err != nil {
		return nil, err
	}

	err = b.prompt(ctx)
	if err != nil {
		return nil, err
	}

	// check status

	return img, nil
}

type uploadImageResp struct {
	Name      string
	Subfolder string
	Type      string
}

/*
	curl -X POST "$BASE_URL/api/upload/image" \
	  -F "image=@my_image.png" \
	  -F "type=input" \
	  -F "overwrite=true"

{"name": "my_image1.png", "subfolder": "", "type": "input"}%
*/
func (b *BiRefNetRemBG) uploadImage(ctx context.Context) error {
	file, err := os.Open(b.imagePath)
	if err != nil {
		return fmt.Errorf("open image: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// image 文件字段
	part, err := writer.CreateFormFile("image", filepath.Base(file.Name()))
	if err != nil {
		return fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("copy form file: %w", err)
	}

	// 其他字段
	_ = writer.WriteField("type", "input")
	_ = writer.WriteField("overwrite", "true")
	_ = writer.Close()

	reqParam := &nhttp.RequestParam{
		RequestURI: uploadUrl,
		Method:     "POST",
		Header:     map[string]string{"Content-Type": writer.FormDataContentType()},
		Body:       body,
		Response:   &uploadImageResp{},
	}
	err = b.cli.DoHTTPRequest(ctx, reqParam)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}

	slog.Debug("get the response", "response", reqParam.Response)

	return nil
}

type promptReq struct {
	Prompt string `json:"prompt"`
}

/*
curl -X POST "$BASE_URL/api/prompt" \

	curl -X POST "http://192.168.4.188:8188/api/prompt" \
	  -H "Content-Type: application/json" \
	  -d '{"prompt": '"$(cat workflow.json)"'}'
*/
func (b *BiRefNetRemBG) prompt(ctx context.Context) error {
	// workflowData = strings.Replace(workflowData, "MyImage.png", filepath.Base(b.imagePath), 1)
	// wk := map[string]any{}
	// err := json.Unmarshal([]byte(workflowData), &wk)
	// if err != nil {
	// 	return fmt.Errorf("unmarshal workflow data: %w", err)
	// }
	// 1. 读取图片并转 base64
	imgBytes, err := os.ReadFile(b.imagePath)
	if err != nil {
		return fmt.Errorf("read image file: %w", err)
	}
	imgBase64 := base64.StdEncoding.EncodeToString(imgBytes)

	// 2. 替换 workflowData
	workflowData = strings.Replace(workflowData, "MyImage.png", "data:image/png;base64,"+imgBase64, 1)

	// 3. unmarshal workflow
	wk := map[string]any{}
	if err := json.Unmarshal([]byte(workflowData), &wk); err != nil {
		return fmt.Errorf("unmarshal workflow data: %w", err)
	}

	// 4. marshal prompt
	body, err := json.Marshal(map[string]any{"prompt": wk})
	if err != nil {
		return fmt.Errorf("marshal workflow data: %w", err)
	}

	// 5. 打印调试
	fmt.Printf("get the prompt request body: %s\n", string(body))

	reqParam := &nhttp.RequestParam{
		RequestURI: promptUrl,
		Method:     "POST",
		Header:     map[string]string{"Content-Type": "application/json"},
		Body:       body,
	}
	err = b.cli.DoHTTPRequest(ctx, reqParam)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}

	fmt.Printf("get the response %#v\n", reqParam.Response)
	slog.Debug("get the response", "response", reqParam.Response)

	return nil
}
