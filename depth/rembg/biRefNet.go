package rembg

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"log/slog"
	"mime/multipart"
	"strings"

	nhttp "github.com/chaos-io/depth2STL/util/http"
	"github.com/segmentio/ksuid"
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
	imgName string
	cli     nhttp.IClient
}

func NewBiRefNetRemBG() Remover {
	return &BiRefNetRemBG{
		cli: nhttp.NewHTTPClient(),
	}
}

func (b *BiRefNetRemBG) Remove(ctx context.Context, img image.Image) (image.Image, error) {
	err := b.uploadImage(ctx, img)
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

/*
	curl -X POST "$BASE_URL/api/upload/image" \
	  -H "X-API-Key: $COMFY_CLOUD_API_KEY" \
	  -F "image=@my_image.png" \
	  -F "type=input" \
	  -F "overwrite=true"
*/
func (b *BiRefNetRemBG) uploadImage(ctx context.Context, img image.Image) error {
	// 1. 编码 image.Image → PNG
	imgBuf := &bytes.Buffer{}
	if err := png.Encode(imgBuf, img); err != nil {
		return err
	}

	// 2. multipart body
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// image 文件字段
	b.imgName = ksuid.New().String() + ".png"
	part, err := writer.CreateFormFile("image", b.imgName)
	if err != nil {
		return err
	}

	if _, err := io.Copy(part, imgBuf); err != nil {
		return err
	}

	// 其他字段
	_ = writer.WriteField("type", "input")
	_ = writer.WriteField("overwrite", "true")

	_ = writer.Close()

	reqParam := &nhttp.RequestParam{
		RequestURI: uploadUrl,
		Method:     "GET",
		Header:     nil,
		Body:       body,
	}
	err = b.cli.DoHTTPRequest(ctx, reqParam)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}

	if reqParam.Response == nil {
		return fmt.Errorf("response is empty")
	}
	slog.Debug("get the response", "response", reqParam.Response)

	return nil
}

/*
	curl -X POST "$BASE_URL/api/prompt" \
	  -H "X-API-Key: $COMFY_CLOUD_API_KEY" \
	  -H "Content-Type: application/json" \
	  -d '{"prompt": '"$(cat workflow_api.json)"'}'
*/
func (b *BiRefNetRemBG) prompt(ctx context.Context) error {
	workflowData = strings.Replace(workflowData, "MyImage.png", b.imgName, 1)
	body, err := json.Marshal(map[string]string{"prompt": workflowData})
	if err != nil {
		return fmt.Errorf("marshal workflow data: %w", err)
	}

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

	if reqParam.Response == nil {
		return fmt.Errorf("response is empty")
	}
	slog.Debug("get the response", "response", reqParam.Response)

	return nil
}
