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
	if cfg == nil {
		return nil, fmt.Errorf("launcher config je nil")
	}
	if cfg.VersionMeta == nil {
		return nil, fmt.Errorf("version meta je nil")
	}
	if cfg.GameDir == "" {
		return nil, fmt.Errorf("GameDir je prázdný")
	}
	if cfg.PlayerName == "" {
		cfg.PlayerName = "Player"
	}
	if cfg.UUID == "" {
		cfg.UUID = "00000000000000000000000000000000"
	}
	if cfg.AccessToken == "" {
		cfg.AccessToken = "0"
	}
	if cfg.AllocMin <= 0 {
		cfg.AllocMin = 512
	}
	if cfg.AllocMax <= 0 {
		cfg.AllocMax = 2048
	}

	meta := cfg.VersionMeta

	status := func(s string) {
		if cfg.OnStatus != nil {
			cfg.OnStatus(s)
		}
	}

	requiredJava := java.JavaMajorForMC(meta.ID)
	if meta.JavaVersion != nil && meta.JavaVersion.MajorVersion > 0 {
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

	status("Připravuji složky...")
	if err := os.MkdirAll(cfg.GameDir, 0755); err != nil {
		return nil, fmt.Errorf("nepodařilo se vytvořit game dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.GameDir, "versions", meta.ID), 0755); err != nil {
		return nil, fmt.Errorf("nepodařilo se vytvořit version dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.GameDir, "libraries"), 0755); err != nil {
		return nil, fmt.Errorf("nepodařilo se vytvořit libraries dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.GameDir, "assets"), 0755); err != nil {
		return nil, fmt.Errorf("nepodařilo se vytvořit assets dir: %w", err)
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

	status("Čistím natives...")
	if err := os.RemoveAll(nativesDir); err != nil {
		return nil, fmt.Errorf("nepodařilo se vyčistit natives: %w", err)
	}
	if err := os.MkdirAll(nativesDir, 0755); err != nil {
		return nil, fmt.Errorf("nepodařilo se vytvořit natives: %w", err)
	}

	status("Rozbaluji natives...")
	if err := extractNatives(cfg, meta, nativesDir); err != nil {
		return nil, fmt.Errorf("natives extraction: %w", err)
	}

	status("Sestavuji classpath...")
	classpath := buildClasspath(cfg, meta)
	if classpath == "" {
		return nil, fmt.Errorf("classpath je prázdný")
	}

	status("Spouštím Minecraft...")
	jvmArgs := buildJVMArgs(cfg, classpath, nativesDir, meta)
	gameArgs := buildGameArgs(cfg, meta)

	args := append([]string{}, jvmArgs...)
	args = append(args, meta.MainClass)
	args = append(args, gameArgs...)

	printLaunchDebug(javaPath, args)

	cmd := exec.Command(javaPath, args...)
	cmd.Dir = cfg.GameDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start Minecraft: %w", err)
	}

	return &LaunchResult{
		Cmd:    cmd,
		Stdout: stdout,
		Stderr: stderr,
	}, nil
}

func ensureClient(cfg *Config, d *assets.Downloader) error {
	meta := cfg.VersionMeta

	if meta.Downloads.Client.URL == "" {
		return fmt.Errorf("client download URL je prázdné")
	}

	clientPath := filepath.Join(cfg.GameDir, "versions", meta.ID, meta.ID+".jar")

	task := assets.DownloadTask{
		URL:  meta.Downloads.Client.URL,
		Path: clientPath,
		SHA1: meta.Downloads.Client.SHA1,
		Size: meta.Downloads.Client.Size,
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

		nativeKey = strings.ReplaceAll(nativeKey, "${arch}", currentArchBits())

		classifier, ok := lib.Downloads.Classifiers[nativeKey]
		if !ok {
			continue
		}

		jarPath := filepath.Join(libDir, classifier.Path)

		if err := extractJarNatives(jarPath, nativesDir); err != nil {
			return fmt.Errorf("%s: %w", jarPath, err)
		}
	}

	return nil
}

func extractJarNatives(jarPath, destDir string) error {
	r, err := zip.OpenReader(jarPath)
	if err != nil {
		return err
	}
	defer r.Close()

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	for _, f := range r.File {
		name := f.Name

		if strings.HasSuffix(name, "/") {
			continue
		}

		normalized := strings.ReplaceAll(name, "\\", "/")
		if strings.HasPrefix(normalized, "META-INF/") {
			continue
		}

		base := filepath.Base(normalized)
		lower := strings.ToLower(base)

		if !strings.HasSuffix(lower, ".dll") &&
			!strings.HasSuffix(lower, ".so") &&
			!strings.HasSuffix(lower, ".dylib") &&
			!strings.HasSuffix(lower, ".jnilib") {
			continue
		}

		destPath := filepath.Join(destDir, base)

		rc, err := f.Open()
		if err != nil {
			return err
		}

		out, err := os.Create(destPath)
		if err != nil {
			rc.Close()
			return err
		}

		_, copyErr := io.Copy(out, rc)

		closeOutErr := out.Close()
		closeRcErr := rc.Close()

		if copyErr != nil {
			return copyErr
		}
		if closeOutErr != nil {
			return closeOutErr
		}
		if closeRcErr != nil {
			return closeRcErr
		}
	}

	return nil
}

func buildLibraryTasks(cfg *Config, meta *versions.VersionMeta) ([]assets.DownloadTask, string, error) {
	libDir := filepath.Join(cfg.GameDir, "libraries")
	nativesDir := filepath.Join(cfg.GameDir, "versions", meta.ID, "natives")

	if err := os.MkdirAll(libDir, 0755); err != nil {
		return nil, "", err
	}
	if err := os.MkdirAll(nativesDir, 0755); err != nil {
		return nil, "", err
	}

	var tasks []assets.DownloadTask
	osName := currentOSName()

	for _, lib := range meta.Libraries {
		if !rulesAllow(lib.Rules) {
			continue
		}

		if lib.Downloads.Artifact != nil && lib.Downloads.Artifact.URL != "" {
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
			nativeKey = strings.ReplaceAll(nativeKey, "${arch}", currentArchBits())

			if classifier, ok := lib.Downloads.Classifiers[nativeKey]; ok && classifier.URL != "" {
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
	sep := string(os.PathListSeparator)

	var parts []string
	seen := map[string]bool{}

	add := func(path string) {
		if path == "" {
			return
		}

		clean := filepath.Clean(path)

		if seen[clean] {
			return
		}

		seen[clean] = true
		parts = append(parts, clean)
	}

	for _, lib := range meta.Libraries {
		if !rulesAllow(lib.Rules) {
			continue
		}

		if lib.Downloads.Artifact != nil && lib.Downloads.Artifact.Path != "" {
			add(filepath.Join(libDir, lib.Downloads.Artifact.Path))
		}
	}

	clientJar := filepath.Join(cfg.GameDir, "versions", meta.ID, meta.ID+".jar")
	add(clientJar)

	return strings.Join(parts, sep)
}

func buildJVMArgs(cfg *Config, classpath, nativesDir string, meta *versions.VersionMeta) []string {
	args := []string{
		fmt.Sprintf("-Xms%dm", cfg.AllocMin),
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

		"-Djava.library.path=" + nativesDir,
	}

	args = append(args, cfg.JVMArgs...)

	hasClasspathFromMeta := false
	hasNativePathFromMeta := false

	if meta.Arguments != nil {
		for _, arg := range meta.Arguments.JVM {
			s, ok := arg.(string)
			if !ok {

				continue
			}

			resolved := resolveVar(s, cfg, meta, nativesDir, classpath)

			if resolved == "-cp" || resolved == "-classpath" || resolved == "${classpath}" {
				hasClasspathFromMeta = true
			}
			if strings.Contains(resolved, "java.library.path") {
				hasNativePathFromMeta = true
			}

			if strings.Contains(resolved, "java.library.path") {
				continue
			}

			args = append(args, resolved)
		}
	}

	_ = hasNativePathFromMeta

	if !hasClasspathFromMeta {
		args = append(args, "-cp", classpath)
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
		"${user_properties}":   "{}",
	}

	var args []string

	if meta.Arguments != nil {
		for _, arg := range meta.Arguments.Game {
			s, ok := arg.(string)
			if !ok {

				continue
			}

			args = append(args, substituteVars(s, vars))
		}
	} else if meta.MinecraftArguments != "" {
		mcArgs := meta.MinecraftArguments

		if !strings.Contains(mcArgs, "--userProperties") {
			mcArgs += " --userProperties {}"
		}

		for _, part := range strings.Fields(mcArgs) {
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
		"${classpath_separator}", string(os.PathListSeparator),
		"${library_directory}", filepath.Join(cfg.GameDir, "libraries"),
		"${game_directory}", cfg.GameDir,
		"${assets_root}", filepath.Join(cfg.GameDir, "assets"),
		"${assets_index_name}", meta.AssetIndex.ID,
		"${version_name}", meta.ID,
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

func currentArchBits() string {
	if runtime.GOARCH == "386" {
		return "32"
	}

	return "64"
}

func printLaunchDebug(javaPath string, args []string) {
	fmt.Println("========== GoLauncher Debug ==========")
	fmt.Println("Java:", javaPath)
	fmt.Println("Args:")

	for i, arg := range args {
		fmt.Printf("[%03d] %s\n", i, arg)
	}

	fmt.Println("======================================")
}
