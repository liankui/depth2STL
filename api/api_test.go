package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCreateHandlerReadsJobOptionsFromRequest(t *testing.T) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	fileWriter, err := writer.CreateFormFile("file", "input.png")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}

	fileContent, err := os.ReadFile(filepath.Join("..", "testdata", "my_image1.png"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	if _, err = fileWriter.Write(fileContent); err != nil {
		t.Fatalf("write form file: %v", err)
	}

	fields := map[string]string{
		"modelWidth":     "66.5",
		"modelThickness": "7.2",
		"baseThickness":  "2.8",
		"skipConv":       "true",
		"invert":         "true",
		"detailLevel":    "3",
	}

	for key, value := range fields {
		if err = writer.WriteField(key, value); err != nil {
			t.Fatalf("write field %s: %v", key, err)
		}
	}

	if err = writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/relief", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	CreateHandler(c)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status code %d, body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		JobID string `json:"jobId"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.JobID == "" {
		t.Fatal("jobId is empty")
	}

	val, ok := jobStore.Load(resp.JobID)
	if !ok {
		t.Fatalf("job %s not found in store", resp.JobID)
	}

	job := val.(*Job)
	if job.ModelWidth != 66.5 {
		t.Fatalf("unexpected modelWidth: %v", job.ModelWidth)
	}
	if job.ModelThickness != 7.2 {
		t.Fatalf("unexpected modelThickness: %v", job.ModelThickness)
	}
	if job.BaseThickness != 2.8 {
		t.Fatalf("unexpected baseThickness: %v", job.BaseThickness)
	}
	if !job.SkipConv {
		t.Fatal("expected skipConv to be true")
	}
	if !job.Invert {
		t.Fatal("expected invert to be true")
	}
	if job.DetailLevel != 3 {
		t.Fatalf("unexpected detailLevel: %d", job.DetailLevel)
	}

	jobStore.Delete(resp.JobID)
	if err = os.RemoveAll(filepath.Dir(job.FilePath)); err != nil {
		t.Fatalf("cleanup temp dir: %v", err)
	}
}
