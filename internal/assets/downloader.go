package assets

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

const (
	AssetsBaseURL    = "https://resources.download.minecraft.net"
	LibrariesBaseURL = "https://libraries.minecraft.net"
	MaxConcurrent    = 32
)

type DownloadTask struct {
	URL      string
	Path     string
	SHA1     string
	Size     int
	Name     string
}

type Progress struct {
	Total     int64
	Completed int64
	Failed    int64
	Current   string
}

type Downloader struct {
	gameDir  string
	client   *http.Client
	progress *Progress
	mu       sync.Mutex
}

func NewDownloader(gameDir string) *Downloader {
	return &Downloader{
		gameDir: gameDir,
		client: &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxConnsPerHost:     32,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		progress: &Progress{},
	}
}

func (d *Downloader) GetProgress() Progress {
	return Progress{
		Total:     atomic.LoadInt64(&d.progress.Total),
		Completed: atomic.LoadInt64(&d.progress.Completed),
		Failed:    atomic.LoadInt64(&d.progress.Failed),
	}
}

func (d *Downloader) DownloadAll(tasks []DownloadTask, onProgress func(Progress)) error {
	atomic.StoreInt64(&d.progress.Total, int64(len(tasks)))
	atomic.StoreInt64(&d.progress.Completed, 0)
	atomic.StoreInt64(&d.progress.Failed, 0)

	sem := make(chan struct{}, MaxConcurrent)
	var wg sync.WaitGroup
	errChan := make(chan error, len(tasks))

	for _, task := range tasks {
		task := task
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := d.downloadFile(task); err != nil {
				atomic.AddInt64(&d.progress.Failed, 1)
				errChan <- fmt.Errorf("failed %s: %w", task.Name, err)
			} else {
				atomic.AddInt64(&d.progress.Completed, 1)
			}

			if onProgress != nil {
				onProgress(d.GetProgress())
			}
		}()
	}

	wg.Wait()
	close(errChan)

	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("%d downloads failed", len(errs))
	}
	return nil
}

func (d *Downloader) downloadFile(task DownloadTask) error {
	if d.fileExists(task.Path, task.SHA1) {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(task.Path), 0755); err != nil {
		return err
	}

	resp, err := d.client.Get(task.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d for %s", resp.StatusCode, task.URL)
	}

	tmpPath := task.Path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	hasher := sha1.New()
	w := io.MultiWriter(f, hasher)

	if _, err := io.Copy(w, resp.Body); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return err
	}
	f.Close()

	if task.SHA1 != "" {
		got := fmt.Sprintf("%x", hasher.Sum(nil))
		if got != task.SHA1 {
			os.Remove(tmpPath)
			return fmt.Errorf("SHA1 mismatch for %s: got %s, want %s", task.Name, got, task.SHA1)
		}
	}

	return os.Rename(tmpPath, task.Path)
}

func (d *Downloader) fileExists(path, sha1hash string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	if sha1hash == "" {
		return true
	}
	h := sha1.New()
	h.Write(data)
	return fmt.Sprintf("%x", h.Sum(nil)) == sha1hash
}

type AssetObjects struct {
	Objects map[string]AssetObject `json:"objects"`
}

type AssetObject struct {
	Hash string `json:"hash"`
	Size int    `json:"size"`
}

func (d *Downloader) BuildAssetTasks(indexURL, indexSHA1, indexID string) ([]DownloadTask, error) {
	indexDir := filepath.Join(d.gameDir, "assets", "indexes")
	indexPath := filepath.Join(indexDir, indexID+".json")

	if err := os.MkdirAll(indexDir, 0755); err != nil {
		return nil, err
	}

	indexTask := DownloadTask{
		URL:  indexURL,
		Path: indexPath,
		SHA1: indexSHA1,
		Name: "asset index",
	}
	if err := d.downloadFile(indexTask); err != nil {
		return nil, fmt.Errorf("failed to download asset index: %w", err)
	}

	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, err
	}

	var objects AssetObjects
	if err := json.Unmarshal(data, &objects); err != nil {
		return nil, err
	}

	objectsDir := filepath.Join(d.gameDir, "assets", "objects")
	var tasks []DownloadTask

	for _, obj := range objects.Objects {
		prefix := obj.Hash[:2]
		objPath := filepath.Join(objectsDir, prefix, obj.Hash)
		tasks = append(tasks, DownloadTask{
			URL:  fmt.Sprintf("%s/%s/%s", AssetsBaseURL, prefix, obj.Hash),
			Path: objPath,
			SHA1: obj.Hash,
			Size: obj.Size,
			Name: obj.Hash[:8],
		})
	}

	return tasks, nil
}
