package modloader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func (i *Installer) InstallForgeAuto(javaPath string, mcVersion string, forgeVersion string, gameDir string, status func(string)) error {
	if javaPath == "" {
		return fmt.Errorf("java path je prázdný")
	}

	if mcVersion == "" {
		return fmt.Errorf("minecraft verze je prázdná")
	}

	if forgeVersion == "" {
		return fmt.Errorf("forge verze je prázdná")
	}

	if gameDir == "" {
		return fmt.Errorf("gameDir je prázdný")
	}

	if status != nil {
		status("Připravuji Forge installer...")
	}

	tmpDir := filepath.Join(os.TempDir(), "golauncher-forge")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return err
	}

	fileName := fmt.Sprintf("forge-%s-%s-installer.jar", mcVersion, forgeVersion)
	installerPath := filepath.Join(tmpDir, fileName)
	url := fmt.Sprintf("https://maven.minecraftforge.net/net/minecraftforge/forge/%s-%s/%s", mcVersion, forgeVersion, fileName)

	if status != nil {
		status("Stahuji Forge installer...")
	}

	if err := downloadFile(url, installerPath); err != nil {
		return err
	}

	if status != nil {
		status("Spouštím Forge installer...")
	}

	cmd := exec.Command(javaPath, "-jar", installerPath, "--installClient", gameDir)
	cmd.Dir = gameDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("forge installer failed: %w\n%s", err, string(output))
	}

	if status != nil {
		status("Forge nainstalován.")
	}

	return nil
}

func (i *Installer) InstallNeoForgeAuto(javaPath string, neoForgeVersion string, gameDir string, status func(string)) error {
	if javaPath == "" {
		return fmt.Errorf("java path je prázdný")
	}

	if neoForgeVersion == "" {
		return fmt.Errorf("neoforge verze je prázdná")
	}

	if gameDir == "" {
		return fmt.Errorf("gameDir je prázdný")
	}

	if status != nil {
		status("Připravuji NeoForge installer...")
	}

	tmpDir := filepath.Join(os.TempDir(), "golauncher-neoforge")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return err
	}

	fileName := fmt.Sprintf("neoforge-%s-installer.jar", neoForgeVersion)
	installerPath := filepath.Join(tmpDir, fileName)
	url := fmt.Sprintf("https://maven.neoforged.net/releases/net/neoforged/neoforge/%s/%s", neoForgeVersion, fileName)

	if status != nil {
		status("Stahuji NeoForge installer...")
	}

	if err := downloadFile(url, installerPath); err != nil {
		return err
	}

	if status != nil {
		status("Spouštím NeoForge installer...")
	}

	cmd := exec.Command(javaPath, "-jar", installerPath, "--installClient", gameDir)
	cmd.Dir = gameDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("neoforge installer failed: %w\n%s", err, string(output))
	}

	if status != nil {
		status("NeoForge nainstalován.")
	}

	return nil
}

func downloadFile(url string, path string) error {
	client := &http.Client{
		Timeout: 2 * time.Minute,
	}

	resp, err := client.Get(url)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	tmp := path + ".tmp"

	f, err := os.Create(tmp)
	if err != nil {
		return err
	}

	_, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()

	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}

	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}

	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}

	return nil
}
