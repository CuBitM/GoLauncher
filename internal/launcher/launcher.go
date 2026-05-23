package launcher

import (
	"archive/zip"
	"fmt"
	"io"
	"mclauncher/internal/assets"
	"mclauncher/internal/java"
	"mclauncher/internal/versions"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type Config struct {
	GameDir     string
	PlayerName  string
	UUID        string
	AccessToken string
	Version     versions.VersionEntry
	VersionMeta *versions.VersionMeta
	JVM         string
	JVMArgs     []string
	AllocMin    int
	AllocMax    int
	Fullscreen  bool
	OnStatus    func(string)
}

type LaunchResult struct {
	Cmd    *exec.Cmd
	Stdout io.ReadCloser
	Stderr io.ReadCloser
}

func Launch(cfg *Config, onProgress func(assets.Progress)) (*LaunchResult, error) {
	meta := cfg.VersionMeta
	status := func(s string) {
		if cfg.OnStatus != nil {
			cfg.OnStatus(s)
		}
	}

	requiredJava := java.JavaMajorForMC(meta.ID)
	if meta.JavaVersion != nil {
		requiredJava = meta.JavaVersion.MajorVersion
	}

	javaPath := cfg.JVM
	if javaPath == "" {
		status(fmt.Sprintf("Hledám Java %d...", requiredJava))
		install, err := java.FindJava(requiredJava)
		if err != nil {
			if notFound, major := java.IsNotFoundError(err); notFound {
				status(fmt.Sprintf("Java %d nenalezena, stahuji automaticky...", major))
				home, _ := os.UserHomeDir()
				javaDir := filepath.Join(home, ".golauncher", "java")
				javaPath, err = java.DownloadJava(major, javaDir, status)
				if err != nil {
					return nil, fmt.Errorf("nepodařilo se stáhnout Javu: %w", err)
				}
			} else {
				return nil, err
			}
		} else {
			javaPath = install.Path
		}
	}

	status("Připravuji soubory hry...")
	downloader := assets.NewDownloader(cfg.GameDir)

	status("Stahuji client JAR...")
	if err := ensureClient(cfg, downloader); err != nil {
		return nil, fmt.Errorf("client jar: %w", err)
	}

	status("Připravuji knihovny...")
	libTasks, nativesDir, err := buildLibraryTasks(cfg, meta)
	if err != nil {
		return nil, err
	}

	status("Připravuji assety...")
	assetTasks, err := downloader.BuildAssetTasks(
		meta.AssetIndex.URL,
		meta.AssetIndex.SHA1,
		meta.AssetIndex.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("assets: %w", err)
	}

	allTasks := append(libTasks, assetTasks...)
	if err := downloader.DownloadAll(allTasks, onProgress); err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}

	status("Rozbaluji natives...")
	if err := extractNatives(cfg, meta, nativesDir); err != nil {
		status("Varování: natives extraction: " + err.Error())
	}

	status("Spouštím Minecraft...")
	classpath := buildClasspath(cfg, meta)
	jvmArgs := buildJVMArgs(cfg, classpath, nativesDir, meta)
	gameArgs := buildGameArgs(cfg, meta)

	args := append(jvmArgs, meta.MainClass)
	args = append(args, gameArgs...)

	cmd := exec.Command(javaPath, args...)
	cmd.Dir = cfg.GameDir

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start Minecraft: %w", err)
	}

	return &LaunchResult{Cmd: cmd, Stdout: stdout, Stderr: stderr}, nil
}

func ensureClient(cfg *Config, d *assets.Downloader) error {
	meta := cfg.VersionMeta
	clientPath := filepath.Join(cfg.GameDir, "versions", meta.ID, meta.ID+".jar")
	task := assets.DownloadTask{
		URL:  meta.Downloads.Client.URL,
		Path: clientPath,
		SHA1: meta.Downloads.Client.SHA1,
		Name: "client.jar",
	}
	return d.DownloadAll([]assets.DownloadTask{task}, nil)
}

func extractNatives(cfg *Config, meta *versions.VersionMeta, nativesDir string) error {
	libDir := filepath.Join(cfg.GameDir, "libraries")
	osName := currentOSName()

	for _, lib := range meta.Libraries {
		if !rulesAllow(lib.Rules) {
			continue
		}
		nativeKey, ok := lib.Natives[osName]
		if !ok {
			continue
		}
		// Replace arch placeholder
		nativeKey = strings.ReplaceAll(nativeKey, "${arch}", "64")
		classifier, ok := lib.Downloads.Classifiers[nativeKey]
		if !ok {
			continue
		}
		jarPath := filepath.Join(libDir, classifier.Path)
		extractJarNatives(jarPath, nativesDir)
	}
	return nil
}

func extractJarNatives(jarPath, destDir string) {
	r, err := zip.OpenReader(jarPath)
	if err != nil {
		return
	}
	defer r.Close()

	for _, f := range r.File {
		name := f.Name
		if strings.HasSuffix(name, "/") {
			continue
		}
		if strings.Contains(name, "META-INF") {
			continue
		}
		base := filepath.Base(name)
		if !strings.HasSuffix(base, ".dll") &&
			!strings.HasSuffix(base, ".so") &&
			!strings.HasSuffix(base, ".dylib") {
			continue
		}
		destPath := filepath.Join(destDir, base)
		if _, err := os.Stat(destPath); err == nil {
			continue // already exists
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		out, err := os.Create(destPath)
		if err != nil {
			rc.Close()
			continue
		}
		io.Copy(out, rc)
		out.Close()
		rc.Close()
	}
}

func buildLibraryTasks(cfg *Config, meta *versions.VersionMeta) ([]assets.DownloadTask, string, error) {
	libDir := filepath.Join(cfg.GameDir, "libraries")
	nativesDir := filepath.Join(cfg.GameDir, "versions", meta.ID, "natives")
	if err := os.MkdirAll(nativesDir, 0755); err != nil {
		return nil, "", err
	}

	var tasks []assets.DownloadTask
	osName := currentOSName()

	for _, lib := range meta.Libraries {
		if !rulesAllow(lib.Rules) {
			continue
		}
		if lib.Downloads.Artifact != nil {
			a := lib.Downloads.Artifact
			tasks = append(tasks, assets.DownloadTask{
				URL:  a.URL,
				Path: filepath.Join(libDir, a.Path),
				SHA1: a.SHA1,
				Size: a.Size,
				Name: lib.Name,
			})
		}
		nativeKey, ok := lib.Natives[osName]
		if ok {
			nativeKey = strings.ReplaceAll(nativeKey, "${arch}", "64")
			if classifier, ok := lib.Downloads.Classifiers[nativeKey]; ok {
				tasks = append(tasks, assets.DownloadTask{
					URL:  classifier.URL,
					Path: filepath.Join(libDir, classifier.Path),
					SHA1: classifier.SHA1,
					Size: classifier.Size,
					Name: lib.Name + " natives",
				})
			}
		}
	}
	return tasks, nativesDir, nil
}

func buildClasspath(cfg *Config, meta *versions.VersionMeta) string {
	libDir := filepath.Join(cfg.GameDir, "libraries")
	sep := ":"
	if runtime.GOOS == "windows" {
		sep = ";"
	}

	var parts []string
	osName := currentOSName()

	for _, lib := range meta.Libraries {
		if !rulesAllow(lib.Rules) {
			continue
		}
		if _, ok := lib.Natives[osName]; ok {
			continue
		}
		if lib.Downloads.Artifact != nil {
			parts = append(parts, filepath.Join(libDir, lib.Downloads.Artifact.Path))
		}
	}

	clientJar := filepath.Join(cfg.GameDir, "versions", meta.ID, meta.ID+".jar")
	parts = append(parts, clientJar)
	return strings.Join(parts, sep)
}

func buildJVMArgs(cfg *Config, classpath, nativesDir string, meta *versions.VersionMeta) []string {
	args := []string{
		"-Xms512m",
		fmt.Sprintf("-Xmx%dm", cfg.AllocMax),
		"-XX:+UnlockExperimentalVMOptions",
		"-XX:+UseG1GC",
		"-XX:G1NewSizePercent=20",
		"-XX:G1ReservePercent=20",
		"-XX:MaxGCPauseMillis=50",
		"-XX:G1HeapRegionSize=32M",
		"-XX:+DisableExplicitGC",
		"-XX:+AlwaysPreTouch",
		"-XX:+PerfDisableSharedMem",
		"-Dfile.encoding=UTF-8",
		"-Djava.net.preferIPv4Stack=true",
		"-Dminecraft.api.auth.host=https://nope.invalid",
		"-Dminecraft.api.account.host=https://nope.invalid",
		"-Dminecraft.api.session.host=https://nope.invalid",
		"-Dminecraft.api.services.host=https://nope.invalid",
		"-Dminecraft.api.profiles.host=https://nope.invalid",
		fmt.Sprintf("-Djava.library.path=%s", nativesDir),
		"-cp", classpath,
	}

	args = append(args, cfg.JVMArgs...)

	if meta.Arguments != nil {
		for _, arg := range meta.Arguments.JVM {
			if s, ok := arg.(string); ok {
				args = append(args, resolveVar(s, cfg, meta, nativesDir, classpath))
			}
		}
	}

	return args
}

func buildGameArgs(cfg *Config, meta *versions.VersionMeta) []string {
	vars := map[string]string{
		"${auth_player_name}":  cfg.PlayerName,
		"${version_name}":      meta.ID,
		"${game_directory}":    cfg.GameDir,
		"${assets_root}":       filepath.Join(cfg.GameDir, "assets"),
		"${assets_index_name}": meta.AssetIndex.ID,
		"${auth_uuid}":         cfg.UUID,
		"${auth_access_token}": cfg.AccessToken,
		"${user_type}":         "legacy",
		"${version_type}":      string(meta.Type),
		"${resolution_width}":  "854",
		"${resolution_height}": "480",
		"${clientid}":          "",
		"${auth_xuid}":         "",
	}

	var args []string

	if meta.Arguments != nil {
		for _, arg := range meta.Arguments.Game {
			if s, ok := arg.(string); ok {
				args = append(args, substituteVars(s, vars))
			}
		}
	} else if meta.MinecraftArguments != "" {
		for _, part := range strings.Fields(meta.MinecraftArguments) {
			args = append(args, substituteVars(part, vars))
		}
	}

	if cfg.Fullscreen {
		args = append(args, "--fullscreen")
	}

	return args
}

func resolveVar(s string, cfg *Config, meta *versions.VersionMeta, nativesDir, classpath string) string {
	r := strings.NewReplacer(
		"${natives_directory}", nativesDir,
		"${launcher_name}", "GoLauncher",
		"${launcher_version}", "1.0.0",
		"${classpath}", classpath,
		"${game_directory}", cfg.GameDir,
		"${assets_root}", filepath.Join(cfg.GameDir, "assets"),
	)
	return r.Replace(s)
}

func substituteVars(s string, vars map[string]string) string {
	for k, v := range vars {
		s = strings.ReplaceAll(s, k, v)
	}
	return s
}

func rulesAllow(rules []versions.Rule) bool {
	if len(rules) == 0 {
		return true
	}
	allowed := false
	osName := currentOSName()
	for _, rule := range rules {
		matches := rule.OS == nil || rule.OS.Name == osName
		if matches {
			allowed = rule.Action == "allow"
		}
	}
	return allowed
}

func currentOSName() string {
	switch runtime.GOOS {
	case "windows":
		return "windows"
	case "darwin":
		return "osx"
	default:
		return "linux"
	}
}
