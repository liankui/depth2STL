package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
)

func main() {
	pageURL := "https://liquipedia.net/dota2/Dota_2_X_Monster_Hunter/Hero_Atlas" // 要爬的页面
	saveDir := "./images"

	err := os.MkdirAll(saveDir, 0755)
	if err != nil {
		panic(err)
	}

	resp, err := http.Get(pageURL)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	// 匹配 img 标签中的 src
	re := regexp.MustCompile(`<img[^>]+src="([^">]+)"`)
	matches := re.FindAllSubmatch(body, -1)

	baseURL, _ := url.Parse(pageURL)

	for _, m := range matches {
		imgURL := string(m[1])
		if !strings.Contains(imgURL, "Dota_2_Monster_Hunter_codex") {
			continue
		}
		imgURL = normalizeLiquipediaImageURL(imgURL)

		// 补全相对路径
		u, err := url.Parse(imgURL)
		if err != nil {
			continue
		}
		fullURL := baseURL.ResolveReference(u).String()

		fmt.Println("下载:", fullURL)

		err = downloadImage(fullURL, saveDir)
		if err != nil {
			fmt.Println("失败:", err)
		}
	}
}

func downloadImage(imgURL, saveDir string) error {
	resp, err := http.Get(imgURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status code %d", resp.StatusCode)
	}

	u, _ := url.Parse(imgURL)
	filename := path.Base(u.Path)
	filePath := path.Join(saveDir, filename)

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	return err
}

func normalizeLiquipediaImageURL(imgURL string) string {
	if strings.Contains(imgURL, "/thumb/") {
		parts := strings.Split(imgURL, "/thumb/")
		if len(parts) != 2 {
			return imgURL
		}
		sub := parts[1]
		idx := strings.LastIndex(sub, "/")
		if idx == -1 {
			return imgURL
		}
		return parts[0] + "/" + sub[:idx]
	}
	return imgURL
}
