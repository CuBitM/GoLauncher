package versions

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const VersionManifestURL = "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json"

type VersionType string

const (
	Release  VersionType = "release"
	Snapshot VersionType = "snapshot"
	OldBeta  VersionType = "old_beta"
	OldAlpha VersionType = "old_alpha"
)

type VersionEntry struct {
	ID          string      `json:"id"`
	Type        VersionType `json:"type"`
	URL         string      `json:"url"`
	Time        time.Time   `json:"time"`
	ReleaseTime time.Time   `json:"releaseTime"`
}

type VersionManifest struct {
	Latest struct {
		Release  string `json:"release"`
		Snapshot string `json:"snapshot"`
	} `json:"latest"`
	Versions []VersionEntry `json:"versions"`
}

type VersionMeta struct {
	ID          string `json:"id"`
	MainClass   string `json:"mainClass"`
	MinecraftArguments string `json:"minecraftArguments"`
	Arguments   *Arguments `json:"arguments"`
	AssetIndex  AssetIndex `json:"assetIndex"`
	Assets      string     `json:"assets"`
	Downloads   Downloads  `json:"downloads"`
	Libraries   []Library  `json:"libraries"`
	JavaVersion *JavaVersion `json:"javaVersion"`
	Type        VersionType `json:"type"`
}

type Arguments struct {
	Game []interface{} `json:"game"`
	JVM  []interface{} `json:"jvm"`
}

type AssetIndex struct {
	ID        string `json:"id"`
	SHA1      string `json:"sha1"`
	Size      int    `json:"size"`
	TotalSize int    `json:"totalSize"`
	URL       string `json:"url"`
}

type Downloads struct {
	Client DownloadInfo `json:"client"`
	Server DownloadInfo `json:"server"`
}

type DownloadInfo struct {
	SHA1 string `json:"sha1"`
	Size int    `json:"size"`
	URL  string `json:"url"`
}

type Library struct {
	Downloads LibraryDownloads `json:"downloads"`
	Name      string           `json:"name"`
	Rules     []Rule           `json:"rules"`
	Natives   map[string]string `json:"natives"`
	Extract   *Extract          `json:"extract"`
}

type LibraryDownloads struct {
	Artifact    *Artifact            `json:"artifact"`
	Classifiers map[string]*Artifact `json:"classifiers"`
}

type Artifact struct {
	Path string `json:"path"`
	SHA1 string `json:"sha1"`
	Size int    `json:"size"`
	URL  string `json:"url"`
}

type Rule struct {
	Action string `json:"action"`
	OS     *OSRule `json:"os"`
}

type OSRule struct {
	Name string `json:"name"`
}

type Extract struct {
	Exclude []string `json:"exclude"`
}

type JavaVersion struct {
	Component    string `json:"component"`
	MajorVersion int    `json:"majorVersion"`
}

type Manager struct {
	gameDir string
	client  *http.Client
}

func NewManager(gameDir string) *Manager {
	return &Manager{
		gameDir: gameDir,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (m *Manager) FetchManifest() (*VersionManifest, error) {
	resp, err := m.client.Get(VersionManifestURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch version manifest: %w", err)
	}
	defer resp.Body.Close()

	var manifest VersionManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("failed to parse version manifest: %w", err)
	}

	sort.Slice(manifest.Versions, func(i, j int) bool {
		return manifest.Versions[i].ReleaseTime.After(manifest.Versions[j].ReleaseTime)
	})

	return &manifest, nil
}

func (m *Manager) FetchVersionMeta(entry VersionEntry) (*VersionMeta, error) {
	metaPath := filepath.Join(m.gameDir, "versions", entry.ID, entry.ID+".json")

	if data, err := os.ReadFile(metaPath); err == nil {
		var meta VersionMeta
		if json.Unmarshal(data, &meta) == nil {
			return &meta, nil
		}
	}

	resp, err := m.client.Get(entry.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch version meta: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var meta VersionMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse version meta: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(metaPath), 0755); err == nil {
		os.WriteFile(metaPath, data, 0644)
	}

	return &meta, nil
}

func (m *Manager) FilterVersions(manifest *VersionManifest, showSnapshots bool, showOld bool) []VersionEntry {
	var filtered []VersionEntry
	for _, v := range manifest.Versions {
		switch v.Type {
		case Release:
			filtered = append(filtered, v)
		case Snapshot:
			if showSnapshots {
				filtered = append(filtered, v)
			}
		case OldBeta, OldAlpha:
			if showOld {
				filtered = append(filtered, v)
			}
		}
	}
	return filtered
}
