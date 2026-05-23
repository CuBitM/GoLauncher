package modloader

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type Type string

const (
	None   Type = "none"
	Fabric Type = "fabric"
	Forge  Type = "forge"
	Quilt  Type = "quilt"
)

const (
	FabricMetaURL = "https://meta.fabricmc.net/v2"
	ForgeFilesURL = "https://maven.minecraftforge.net"
	QuiltMetaURL  = "https://meta.quiltmc.org/v3"
)

type FabricVersion struct {
	Loader struct {
		Version string `json:"version"`
	} `json:"loader"`
	LaunchWrapper struct {
		Version string `json:"version"`
	} `json:"launchWrapper"`
}

type ForgeVersion struct {
	Version string
	MCVersion string
}

type Installer struct {
	gameDir string
	client  *http.Client
}

func NewInstaller(gameDir string) *Installer {
	return &Installer{
		gameDir: gameDir,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (i *Installer) FetchFabricLoaders(mcVersion string) ([]string, error) {
	url := fmt.Sprintf("%s/versions/loader/%s", FabricMetaURL, mcVersion)
	resp, err := i.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var versions []FabricVersion
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return nil, err
	}

	var loaders []string
	for _, v := range versions {
		loaders = append(loaders, v.Loader.Version)
	}
	return loaders, nil
}

func (i *Installer) InstallFabric(mcVersion, loaderVersion string, onProgress func(string)) error {
	profileID := fmt.Sprintf("fabric-loader-%s-%s", loaderVersion, mcVersion)
	versionDir := filepath.Join(i.gameDir, "versions", profileID)

	if err := os.MkdirAll(versionDir, 0755); err != nil {
		return err
	}

	metaPath := filepath.Join(versionDir, profileID+".json")
	if _, err := os.Stat(metaPath); err == nil {
		if onProgress != nil {
			onProgress("Fabric already installed")
		}
		return nil
	}

	if onProgress != nil {
		onProgress("Downloading Fabric profile...")
	}

	url := fmt.Sprintf("%s/versions/loader/%s/%s/profile/json",
		FabricMetaURL, mcVersion, loaderVersion)

	resp, err := i.client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch Fabric profile: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		return err
	}

	if onProgress != nil {
		onProgress(fmt.Sprintf("Fabric %s installed for MC %s", loaderVersion, mcVersion))
	}

	return nil
}

func (i *Installer) FetchForgeVersions(mcVersion string) ([]string, error) {
	url := fmt.Sprintf("%s/net/minecraftforge/forge/maven-metadata.xml", ForgeFilesURL)
	resp, err := i.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var versions []string
	content := string(data)
	prefix := mcVersion + "-"

	start := 0
	for {
		idx := indexOf(content[start:], "<version>")
		if idx < 0 {
			break
		}
		idx += start + len("<version>")
		end := indexOf(content[idx:], "</version>")
		if end < 0 {
			break
		}
		ver := content[idx : idx+end]
		if len(ver) > len(prefix) && ver[:len(prefix)] == prefix {
			forgeVer := ver[len(prefix):]
			if len(forgeVer) > 0 {
				versions = append(versions, ver)
			}
		}
		start = idx + end + len("</version>")
	}

	if len(versions) > 10 {
		versions = versions[len(versions)-10:]
	}

	reversed := make([]string, len(versions))
	for j, v := range versions {
		reversed[len(versions)-1-j] = v
	}
	return reversed, nil
}

func (i *Installer) DetectInstalled(loaderType Type, mcVersion string) []string {
	versionsDir := filepath.Join(i.gameDir, "versions")
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		return nil
	}

	var installed []string
	prefix := ""
	switch loaderType {
	case Fabric:
		prefix = fmt.Sprintf("fabric-loader-")
	case Forge:
		prefix = mcVersion + "-forge-"
	case Quilt:
		prefix = "quilt-loader-"
	}

	for _, e := range entries {
		if e.IsDir() && len(e.Name()) > len(prefix) && e.Name()[:len(prefix)] == prefix {
			installed = append(installed, e.Name())
		}
	}
	return installed
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
