package java

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const AdoptiumAPI = "https://api.adoptium.net/v3/assets/latest/%d/hotspot?os=%s&architecture=x64&image_type=jre"

type JavaInstall struct {
	Path    string
	Version int
}

type adoptiumAsset struct {
	Binary struct {
		Package struct {
			Link     string `json:"link"`
			Checksum string `json:"checksum"`
		} `json:"package"`
	} `json:"binary"`
	ReleaseName string `json:"release_name"`
}

func FindJava(requiredMajor int) (*JavaInstall, error) {
	candidates := findCandidates(requiredMajor)
	var best *JavaInstall
	for _, path := range candidates {
		install, err := probeJava(path)
		if err != nil {
			continue
		}
		if install.Version >= requiredMajor {
			if best == nil || install.Version < best.Version {
				best = install
			}
		}
	}
	if best != nil {
		return best, nil
	}
	return nil, fmt.Errorf("java%d_not_found", requiredMajor)
}

func IsNotFoundError(err error) (bool, int) {
	if err == nil {
		return false, 0
	}
	s := err.Error()
	var major int
	if n, _ := fmt.Sscanf(s, "java%d_not_found", &major); n == 1 {
		return true, major
	}
	return false, 0
}

func DownloadJava(major int, destDir string, onProgress func(string)) (string, error) {
	osName := adoptiumOS()
	url := fmt.Sprintf(AdoptiumAPI, major, osName)

	if onProgress != nil {
		onProgress(fmt.Sprintf("Hledám Java %d na Adoptium...", major))
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("adoptium API error: %w", err)
	}
	defer resp.Body.Close()

	var assets []adoptiumAsset
	if err := json.NewDecoder(resp.Body).Decode(&assets); err != nil || len(assets) == 0 {
		return "", fmt.Errorf("žádná Java %d nenalezena na Adoptium", major)
	}

	asset := assets[0]
	downloadURL := asset.Binary.Package.Link
	releaseName := asset.ReleaseName

	if onProgress != nil {
		onProgress(fmt.Sprintf("Stahuji %s...", releaseName))
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", err
	}

	ext := ".zip"
	if runtime.GOOS != "windows" {
		ext = ".tar.gz"
	}
	tmpFile := filepath.Join(destDir, "java-download"+ext)

	if err := downloadFile(downloadURL, tmpFile, onProgress, client); err != nil {
		return "", fmt.Errorf("stažení selhalo: %w", err)
	}

	if onProgress != nil {
		onProgress("Rozbaluji Javu...")
	}

	extractDir := filepath.Join(destDir, fmt.Sprintf("java%d", major))
	os.RemoveAll(extractDir)

	var javaExe string
	if runtime.GOOS == "windows" {
		if err := unzip(tmpFile, extractDir); err != nil {
			return "", fmt.Errorf("rozbalení selhalo: %w", err)
		}
		javaExe = findJavaExe(extractDir, "java.exe")
	} else {
		if err := untarGz(tmpFile, extractDir); err != nil {
			return "", fmt.Errorf("rozbalení selhalo: %w", err)
		}
		javaExe = findJavaExe(extractDir, "java")
	}

	os.Remove(tmpFile)

	if javaExe == "" {
		return "", fmt.Errorf("java executable nenalezena po rozbalení")
	}

	if runtime.GOOS != "windows" {
		os.Chmod(javaExe, 0755)
	}

	if onProgress != nil {
		onProgress(fmt.Sprintf("Java %d nainstalována!", major))
	}

	return javaExe, nil
}

func downloadFile(url, dest string, onProgress func(string), client *http.Client) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	total := resp.ContentLength
	var downloaded int64
	buf := make([]byte, 32*1024)
	lastReport := time.Now()

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			f.Write(buf[:n])
			downloaded += int64(n)
			if onProgress != nil && time.Since(lastReport) > 500*time.Millisecond {
				if total > 0 {
					pct := int(downloaded * 100 / total)
					onProgress(fmt.Sprintf("Stahuji Javu... %d%% (%d MB)", pct, downloaded/1024/1024))
				}
				lastReport = time.Now()
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			continue
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, 0755)
			continue
		}
		os.MkdirAll(filepath.Dir(fpath), 0755)
		rc, err := f.Open()
		if err != nil {
			continue
		}
		out, err := os.Create(fpath)
		if err != nil {
			rc.Close()
			continue
		}
		io.Copy(out, rc)
		out.Close()
		rc.Close()
	}
	return nil
}

func untarGz(src, dest string) error {
	cmd := exec.Command("tar", "-xzf", src, "-C", dest, "--strip-components=1")
	os.MkdirAll(dest, 0755)
	return cmd.Run()
}

func findJavaExe(root, name string) string {
	var found string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && info.Name() == name {
			found = path
			return fmt.Errorf("stop")
		}
		return nil
	})
	return found
}

func findCandidates(major int) []string {
	var candidates []string

	// Nejdřív zkontroluj naši vlastní staženou Javu
	home, _ := os.UserHomeDir()
	localJava := filepath.Join(home, ".golauncher", "java", fmt.Sprintf("java%d", major))
	if runtime.GOOS == "windows" {
		candidates = append(candidates, findJavaExe(localJava, "java.exe"))
	} else {
		candidates = append(candidates, findJavaExe(localJava, "java"))
	}

	if p, err := exec.LookPath("java"); err == nil {
		candidates = append(candidates, p)
	}

	switch runtime.GOOS {
	case "windows":
		for _, base := range []string{
			`C:\Program Files\Java`,
			`C:\Program Files\Eclipse Adoptium`,
			`C:\Program Files\Microsoft`,
			`C:\Program Files\Amazon Corretto`,
			`C:\Program Files\BellSoft`,
		} {
			entries, err := os.ReadDir(base)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if e.IsDir() {
					candidates = append(candidates, filepath.Join(base, e.Name(), "bin", "java.exe"))
				}
			}
		}
	case "darwin":
		for _, base := range []string{"/Library/Java/JavaVirtualMachines", "/System/Library/Java/JavaVirtualMachines"} {
			entries, _ := os.ReadDir(base)
			for _, e := range entries {
				candidates = append(candidates, filepath.Join(base, e.Name(), "Contents/Home/bin/java"))
			}
		}
	case "linux":
		for _, base := range []string{"/usr/lib/jvm", "/usr/java", "/opt/java", "/opt/jdk"} {
			entries, _ := os.ReadDir(base)
			for _, e := range entries {
				candidates = append(candidates, filepath.Join(base, e.Name(), "bin", "java"))
			}
		}
	}

	// Filtruj prázdné
	var clean []string
	for _, c := range candidates {
		if c != "" {
			clean = append(clean, c)
		}
	}
	return clean
}

func probeJava(path string) (*JavaInstall, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	out, err := exec.Command(path, "-version").CombinedOutput()
	if err != nil {
		return nil, err
	}
	version := parseVersion(string(out))
	if version == 0 {
		return nil, fmt.Errorf("could not parse java version")
	}
	return &JavaInstall{Path: path, Version: version}, nil
}

func parseVersion(output string) int {
	output = strings.ToLower(output)
	var versionStr string
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "version") {
			parts := strings.Fields(line)
			for _, p := range parts {
				p = strings.Trim(p, `"`)
				if strings.Contains(p, ".") || (len(p) > 0 && p[0] >= '0' && p[0] <= '9') {
					versionStr = p
					break
				}
			}
			break
		}
	}
	var major int
	if strings.HasPrefix(versionStr, "1.") {
		fmt.Sscanf(versionStr, "1.%d", &major)
	} else {
		fmt.Sscanf(versionStr, "%d", &major)
	}
	return major
}

func JavaMajorForMC(mcVersion string) int {
	parts := strings.Split(mcVersion, ".")
	if len(parts) < 2 {
		return 8
	}
	var minor int
	fmt.Sscanf(parts[1], "%d", &minor)
	switch {
	case minor >= 21:
		return 21
	case minor >= 17:
		return 17
	case minor >= 16:
		return 16
	default:
		return 8
	}
}

func adoptiumOS() string {
	switch runtime.GOOS {
	case "windows":
		return "windows"
	case "darwin":
		return "mac"
	default:
		return "linux"
	}
}
