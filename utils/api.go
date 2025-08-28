package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/adamwreuben/twiggatools/models"
)

type APIClient struct {
	BaseURL        string
	AccountBaseURL string
	Token          string
	HTTP           *http.Client
}

func NewAPIClientFromConfig(cfg *models.Config) *APIClient {

	return &APIClient{
		BaseURL:        cfg.BaseURL,
		AccountBaseURL: cfg.AccountBaseURL,
		Token:          cfg.Token,
		HTTP:           &http.Client{Timeout: 10 * time.Minute},
	}
}

func (a *APIClient) doJSON(ctx context.Context, method, url string, body any) ([]byte, int, error) {
	var reader io.Reader
	if body != nil {
		bs, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reader = bytes.NewReader(bs)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if a.Token != "" {
		req.Header.Set("BONGO-TOKEN", a.Token)
	}
	resp, err := a.HTTP.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return b, resp.StatusCode, nil
}

func (a *APIClient) Authenticate(ctx context.Context, redirectTo string) (string, error) {
	url := fmt.Sprintf("%s/application/authenticate", strings.TrimRight(a.AccountBaseURL, "/"))
	req := map[string]string{"redirectTo": redirectTo}
	b, status, err := a.doJSON(ctx, http.MethodPost, url, req)
	if err != nil {
		return "", err
	}
	if status >= 400 {
		return "", fmt.Errorf("authenticate failed: %s", string(b))
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return "", err
	}
	// check common keys
	for _, k := range []string{"url", "auth_url", "authUrl", "authorization_url"} {
		if v, ok := m[k]; ok {
			if s, ok2 := v.(string); ok2 && s != "" {
				return s, nil
			}
		}
	}
	// fallback: find any string that looks like a url
	for _, v := range m {
		if s, ok := v.(string); ok && strings.HasPrefix(s, "http") {
			return s, nil
		}
	}
	return "", fmt.Errorf("no auth url found in response: %s", string(b))
}

// CreateDocumentAuto creates a new document with auto-generated ID
func (a *APIClient) CreateDocumentAuto(ctx context.Context, db string, collection string, doc any) ([]byte, error) {
	url := fmt.Sprintf("%s/document/%s/%s", a.BaseURL, db, collection)
	res, _, err := a.doJSON(ctx, http.MethodPost, url, doc)
	return res, err
}

// CreateDocumentWithID creates a document with a specified ID
func (a *APIClient) CreateDocumentWithID(ctx context.Context, db string, collection, id string, doc any) ([]byte, error) {
	url := fmt.Sprintf("%s/document/%s/%s/%s", a.BaseURL, db, collection, id)
	res, _, err := a.doJSON(ctx, http.MethodPost, url, doc)
	return res, err
}

func (a *APIClient) QueryDocuments(ctx context.Context, db string, collection string, filter map[string]any) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/document/%s/%s/filter", a.BaseURL, db, collection)

	body, statusCode, err := a.doJSON(ctx, http.MethodPost, url, filter)

	if err != nil {
		return nil, err
	}

	if statusCode == http.StatusOK {

		var doc map[string]interface{}
		if err := json.Unmarshal(body, &doc); err != nil {
			return nil, err
		}

		return doc, nil

	}

	if statusCode == 429 {
		return nil, errors.New("too many request per IP, please try again later")
	}

	return nil, errors.New("Unknown error!")
}

func (a *APIClient) AddBucket(ctx context.Context, name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/storage/buckets", strings.TrimRight(a.BaseURL, "/"))
	req := map[string]string{"name": name}
	b, status, err := a.doJSON(ctx, http.MethodPost, url, req)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("create bucket failed: %s", string(b))
	}

	var data map[string]interface{}
	json.Unmarshal(b, &data)
	return data, nil
}

func (a *APIClient) GetTokenData(ctx context.Context, token string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/user/token/%s", strings.TrimRight(a.AccountBaseURL, "/"), token)
	b, status, err := a.doJSON(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	if status >= 400 {
		return nil, fmt.Errorf("failed: %s", string(b))
	}
	var data map[string]interface{}
	json.Unmarshal(b, &data)

	return data, nil
}

// UploadFiles uploads multiple files (relative paths preserved).
// baseDir is used to compute the relative path used as the object name.
// returns the list of uploaded object names as returned by server.
func (a *APIClient) UploadFiles(ctx context.Context, bucket string, filePaths []string, baseDir string) ([]string, error) {
	url := fmt.Sprintf("%s/storage/buckets/%s/objects", strings.TrimRight(a.BaseURL, "/"), bucket)
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	for _, fullPath := range filePaths {
		rel := filepath.ToSlash(strings.TrimPrefix(fullPath, baseDir+string(os.PathSeparator)))
		if rel == "" {
			rel = filepath.Base(fullPath)
		}
		f, err := os.Open(fullPath)
		if err != nil {
			return nil, err
		}
		// field name must be "files" (server expects form.File["files"])
		part, err := w.CreateFormFile("files", rel)
		if err != nil {
			f.Close()
			return nil, err
		}
		_, err = io.Copy(part, f)
		f.Close()
		if err != nil {
			return nil, err
		}
	}
	w.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	if a.Token != "" {
		req.Header.Set("BONGO-TOKEN", a.Token)
	}

	resp, err := a.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("upload failed: %s", string(respBody))
	}

	var r struct {
		Files []string `json:"files"`
	}
	if err := json.Unmarshal(respBody, &r); err != nil {
		return nil, err
	}
	return r.Files, nil
}

func (a *APIClient) GetFiles(ctx context.Context, bucket string) ([]map[string]any, error) {
	url := fmt.Sprintf("%s/storage/buckets/%s/objects", strings.TrimRight(a.BaseURL, "/"), bucket)
	b, status, err := a.doJSON(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("get files failed: %s", string(b))
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	if arr, ok := m["files"].([]any); ok {
		out := make([]map[string]any, 0, len(arr))
		for _, it := range arr {
			if mm, ok := it.(map[string]any); ok {
				out = append(out, mm)
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("unexpected files response: %s", string(b))
}

func (a *APIClient) GetFileURL(ctx context.Context, bucket, object string) (string, error) {
	url := fmt.Sprintf("%s/storage/buckets/%s/objects/%s", strings.TrimRight(a.BaseURL, "/"), bucket, object)
	b, status, err := a.doJSON(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	if status >= 400 {
		return "", fmt.Errorf("get file url failed: %s", string(b))
	}
	var r struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return "", err
	}
	return r.URL, nil
}

func (a *APIClient) DeleteFile(ctx context.Context, bucket, object string) error {
	url := fmt.Sprintf("%s/storage/buckets/%s/objects/%s", strings.TrimRight(a.BaseURL, "/"), bucket, object)
	b, status, err := a.doJSON(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	if status >= 400 {
		return fmt.Errorf("delete file failed: %s", string(b))
	}
	return nil
}

func (a *APIClient) UploadSiteVersion(ctx context.Context, bucket, siteID, version, dir string) ([]string, error) {
	url := fmt.Sprintf("%s/hosting/%s/%s/%s/upload", strings.TrimRight(a.BaseURL, "/"), bucket, siteID, version)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	files := []string{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			rel := filepath.ToSlash(strings.TrimPrefix(path, dir+string(os.PathSeparator)))
			if rel == "" {
				rel = info.Name()
			}
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			part, err := w.CreateFormFile("files", rel)
			if err != nil {
				return err
			}
			if _, err := io.Copy(part, f); err != nil {
				return err
			}
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	w.Close()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	if a.Token != "" {
		req.Header.Set("BONGO-TOKEN", a.Token)
	}

	resp, err := a.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("upload failed: %s", string(body))
	}

	return files, nil
}

func (a *APIClient) PointChannel(ctx context.Context, bucket, siteID, channel, version string) error {
	url := fmt.Sprintf("%s/hosting/%s/%s/channels/%s", strings.TrimRight(a.BaseURL, "/"), bucket, siteID, channel)

	reqBody := map[string]string{"version": version}
	b, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if a.Token != "" {
		req.Header.Set("BONGO-TOKEN", a.Token)
	}

	resp, err := a.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("point channel failed: %s", string(body))
	}

	return nil
}

// SetBucketPolicy sets the access policy of a bucket ("public" or "private").
func (a *APIClient) SetBucketPolicy(ctx context.Context, bucket, policy string) error {
	url := fmt.Sprintf("%s/storage/buckets/%s/policy", strings.TrimRight(a.BaseURL, "/"), bucket)
	req := map[string]string{"policy": policy}

	b, status, err := a.doJSON(ctx, http.MethodPost, url, req)
	if err != nil {
		return err
	}
	if status >= 400 {
		return fmt.Errorf("set bucket policy failed: %s", string(b))
	}
	return nil
}
