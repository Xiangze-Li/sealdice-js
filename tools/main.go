package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang-module/carbon"
)

var (
	reMeta      = regexp.MustCompile(`(?s)//[ \t]*==UserScript==[ \t]*\r?\n(.*)//[ \t]*==/UserScript==`)
	reItem      = regexp.MustCompile(`//[ \t]*@(\S+)\s+([^\r\n]+)`)
	ghURLPrefix = "https://raw.githubusercontent.com/sealdice/javascript/main/"
)

type pluginMeta struct {
	Path        string   `json:"path,omitempty"`
	Name        string   `json:"name,omitempty"`
	HomePage    string   `json:"home_page,omitempty"`
	License     string   `json:"license,omitempty"`
	Author      string   `json:"author,omitempty"`
	Version     string   `json:"version,omitempty"`
	Description string   `json:"description,omitempty"`
	UpdateTime  string   `json:"update_time,omitempty"`
	UpdateURLs  []string `json:"update_urls,omitempty"`
	Etag        string   `json:"etag,omitempty"`
	Depents     []string `json:"depents,omitempty"`

	DownloadURL string `json:"download_url,omitempty"`
}

func main() {
	const rootPath = "./scripts"
	const outputPath = "./scripts.json"

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))

	if len(os.Args) > 1 {
		wd := os.Args[1]
		if err := os.Chdir(wd); err != nil {
			slog.Error("failed to change working directory", "error", err)
			os.Exit(1)
		}
	}

	metas, err := walkJS(rootPath)
	if err != nil {
		slog.Error("failed to walk javascript files", "error", err.Error())
		os.Exit(1)
	}

	if err := output(metas, outputPath); err != nil {
		slog.Error("failed to output", "error", err.Error())
		os.Exit(1)
	}
}

func output(metas []pluginMeta, path string) error {
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(metas)
}

func walkJS(rootPath string) ([]pluginMeta, error) {
	ret := []pluginMeta{}

	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || filepath.Ext(path) != ".js" {
			return nil
		}

		meta, err := handleFile(path)
		if err != nil {
			slog.Error("failed to handle javascript file", "path", path, "error", err.Error())
			return nil
		}

		ret = append(ret, meta)
		return nil
	})

	if err != nil {
		return nil, err
	}
	return ret, nil
}

func handleFile(path string) (pluginMeta, error) {
	ret := pluginMeta{Path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		return ret, fmt.Errorf("read file error: %w", err)
	}

	meta := reMeta.FindSubmatch(data)
	if len(meta) != 2 {
		return ret, errors.New("no metadata found")
	}

	items := reItem.FindAllSubmatch(meta[1], -1)
	if len(items) == 0 {
		return ret, errors.New("no metadata found")
	}

	ret.UpdateURLs = []string{}
	ret.Depents = []string{}

	for _, item := range items {
		value := string(item[2])
		switch string(item[1]) {
		case "name":
			ret.Name = value
		case "homepageURL":
			ret.HomePage = value
		case "license":
			ret.License = value
		case "author":
			ret.Author = value
		case "version":
			ret.Version = value
		case "description":
			ret.Description = value
		case "timestamp":
			if ts, errParse := strconv.ParseInt(value, 10, 64); errParse == nil {
				ret.UpdateTime = time.Unix(ts, 0).Local().Format(time.DateTime)
				continue
			}
			if t := carbon.Parse(value); t.IsValid() {
				ret.UpdateTime = t.ToStdTime().Local().Format(time.DateTime)
			}
		case "updateURL":
			ret.UpdateURLs = append(ret.UpdateURLs, value)
		case "etag":
			ret.Etag = value
		case "depents":
			ret.Depents = append(ret.Depents, value)
		}
	}

	pathItems := strings.Split(path, string(filepath.Separator))
	for i := range pathItems {
		pathItems[i] = url.PathEscape(pathItems[i])
	}
	ret.DownloadURL, _ = url.JoinPath(ghURLPrefix, pathItems...)
	return ret, nil
}
