package gui

import (
	"fmt"
	"mclauncher/configs"
	"mclauncher/internal/assets"
	"mclauncher/internal/launcher"
	"mclauncher/internal/modloader"
	"mclauncher/internal/versions"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"image/color"
)

type App struct {
	fyneApp     fyne.App
	win         fyne.Window
	cfg         *configs.LauncherConfig
	versionsMgr *versions.Manager
	manifest    *versions.VersionManifest
	modInst     *modloader.Installer

	versionSelect *widget.Select
	statusLabel   *widget.Label
	progressBar   *widget.ProgressBar
	launchBtn     *widget.Button
	logOutput     *widget.TextGrid
	accountLabel  *widget.Label
	ramSlider     *widget.Slider
	ramLabel      *widget.Label
}

func NewApp() *App {
	cfg, _ := configs.Load()
	return &App{
		cfg:         cfg,
		versionsMgr: versions.NewManager(cfg.GameDir),
		modInst:     modloader.NewInstaller(cfg.GameDir),
	}
}

func (a *App) Run() {
	a.fyneApp = app.New()
	a.fyneApp.Settings().SetTheme(theme.DarkTheme())

	a.win = a.fyneApp.NewWindow("GoLauncher — Minecraft")
	a.win.Resize(fyne.NewSize(900, 580))
	a.win.SetFixedSize(true)

	content := a.buildUI()
	a.win.SetContent(content)

	go a.loadVersions()

	a.win.ShowAndRun()
}

func (a *App) buildUI() fyne.CanvasObject {
	header := a.buildHeader()
	tabs := container.NewAppTabs(
		container.NewTabItem("🚀 Launch", a.buildLaunchTab()),
		container.NewTabItem("⚙️  Settings", a.buildSettingsTab()),
		container.NewTabItem("🧩 Mod Loaders", a.buildModLoadersTab()),
		container.NewTabItem("📋 Console", a.buildConsoleTab()),
	)
	return container.NewBorder(header, nil, nil, nil, tabs)
}

func (a *App) buildHeader() fyne.CanvasObject {
	bg := canvas.NewRectangle(color.NRGBA{R: 20, G: 20, B: 30, A: 255})
	bg.SetMinSize(fyne.NewSize(900, 60))

	title := canvas.NewText("⛏  GoLauncher", color.NRGBA{R: 80, G: 200, B: 80, A: 255})
	title.TextSize = 22
	title.TextStyle = fyne.TextStyle{Bold: true}

	subtitle := canvas.NewText("Ultra-fast Minecraft Launcher", color.NRGBA{R: 150, G: 150, B: 160, A: 255})
	subtitle.TextSize = 11

	a.accountLabel = widget.NewLabel("Offline mode")

	usernameEntry := widget.NewEntry()
	usernameEntry.SetPlaceHolder("Hráčské jméno...")
	usernameEntry.SetText(a.cfg.OfflineUsername)

	loginBtn := widget.NewButton("🔑 MS Login", a.onMSLogin)
	offlineBtn := widget.NewButton("▶ Offline", func() {
		name := strings.TrimSpace(usernameEntry.Text)
		if name == "" {
			dialog.ShowError(fmt.Errorf("zadej hráčské jméno"), a.win)
			return
		}
		a.cfg.OfflineUsername = name
		a.cfg.OfflineMode = true
		a.cfg.Account = nil
		configs.Save(a.cfg)
		a.accountLabel.SetText("Offline: " + name)
		a.setStatus("Offline mód — jméno: " + name)
	})

	left := container.NewVBox(title, subtitle)
	right := container.NewHBox(usernameEntry, offlineBtn, widget.NewLabel("|"), a.accountLabel, loginBtn)

	return container.NewStack(
		bg,
		container.NewPadded(container.NewBorder(nil, nil, left, right)),
	)
}

func (a *App) buildLaunchTab() fyne.CanvasObject {
	a.versionSelect = widget.NewSelect([]string{"Načítám verze..."}, func(v string) {
		a.cfg.SelectedVersion = v
		configs.Save(a.cfg)
	})

	versionRow := container.NewBorder(nil, nil, widget.NewLabel("Verze:"), nil, a.versionSelect)

	// RAM Slider
	a.ramLabel = widget.NewLabel(fmt.Sprintf("RAM: %d MB", a.cfg.AllocMax))
	a.ramSlider = widget.NewSlider(512, 16384)
	a.ramSlider.Step = 256
	a.ramSlider.SetValue(float64(a.cfg.AllocMax))
	a.ramSlider.OnChanged = func(v float64) {
		a.cfg.AllocMax = int(v)
		a.ramLabel.SetText(fmt.Sprintf("RAM: %d MB", int(v)))
		configs.Save(a.cfg)
	}
	ramRow := container.NewBorder(nil, nil, a.ramLabel, nil, a.ramSlider)

	// Fullscreen checkbox
	fullscreenCheck := widget.NewCheck("Fullscreen", func(v bool) {
		a.cfg.Fullscreen = v
		configs.Save(a.cfg)
	})
	fullscreenCheck.SetChecked(a.cfg.Fullscreen)

	// Snapshots / old
	snapshotCheck := widget.NewCheck("Snapshots", func(v bool) {
		a.cfg.ShowSnapshots = v
		a.refreshVersionList()
		configs.Save(a.cfg)
	})
	snapshotCheck.SetChecked(a.cfg.ShowSnapshots)

	oldCheck := widget.NewCheck("Staré verze", func(v bool) {
		a.cfg.ShowOld = v
		a.refreshVersionList()
		configs.Save(a.cfg)
	})
	oldCheck.SetChecked(a.cfg.ShowOld)

	checksRow := container.NewHBox(fullscreenCheck, snapshotCheck, oldCheck)

	a.progressBar = widget.NewProgressBar()
	a.progressBar.Hide()

	a.statusLabel = widget.NewLabel("Připraven")
	a.statusLabel.Alignment = fyne.TextAlignCenter

	a.launchBtn = widget.NewButton("▶  SPUSTIT", a.onLaunch)
	a.launchBtn.Importance = widget.HighImportance

	return container.NewPadded(container.NewVBox(
		versionRow,
		ramRow,
		checksRow,
		layout.NewSpacer(),
		a.statusLabel,
		a.progressBar,
		a.launchBtn,
	))
}

func (a *App) buildSettingsTab() fyne.CanvasObject {
	gameDirEntry := widget.NewEntry()
	gameDirEntry.SetText(a.cfg.GameDir)

	jvmEntry := widget.NewEntry()
	jvmEntry.SetPlaceHolder("Auto-detect (doporučeno)")
	jvmEntry.SetText(a.cfg.CustomJVM)

	extraJVMEntry := widget.NewEntry()
	extraJVMEntry.SetPlaceHolder("-XX:+UseZGC -Dfml.ignoreInvalidMinecraftCertificates=true")
	extraJVMEntry.SetText(strings.Join(a.cfg.ExtraJVMArgs, " "))

	saveBtn := widget.NewButton("Uložit nastavení", func() {
		a.cfg.GameDir = gameDirEntry.Text
		a.cfg.CustomJVM = jvmEntry.Text
		if extraJVMEntry.Text != "" {
			a.cfg.ExtraJVMArgs = strings.Fields(extraJVMEntry.Text)
		} else {
			a.cfg.ExtraJVMArgs = nil
		}
		configs.Save(a.cfg)
		dialog.ShowInformation("Uloženo", "Nastavení uloženo!", a.win)
	})

	openDirBtn := widget.NewButton("📂 Otevřít game dir", func() {
		openFolder(a.cfg.GameDir)
	})

	form := widget.NewForm(
		widget.NewFormItem("Game Directory", gameDirEntry),
		widget.NewFormItem("Java Path", jvmEntry),
		widget.NewFormItem("Extra JVM Args", extraJVMEntry),
	)

	return container.NewPadded(container.NewVBox(form, saveBtn, openDirBtn))
}

func (a *App) buildModLoadersTab() fyne.CanvasObject {
	statusLabel := widget.NewLabel("Vyber verzi v Launch tabu nejdřív")
	loaderVersionSelect := widget.NewSelect([]string{}, func(string) {})
	loaderVersionSelect.Disable()

	var selectedType modloader.Type = modloader.None

	typeSelect := widget.NewSelect([]string{"None", "Fabric", "Forge", "Quilt"}, func(v string) {
		switch v {
		case "Fabric":
			selectedType = modloader.Fabric
		case "Forge":
			selectedType = modloader.Forge
		case "Quilt":
			selectedType = modloader.Quilt
		default:
			selectedType = modloader.None
			return
		}

		mcVersion := extractVersionID(a.cfg.SelectedVersion)
		if mcVersion == "" {
			statusLabel.SetText("Vyber MC verzi v Launch tabu")
			return
		}

		statusLabel.SetText("Načítám loader verze...")
		loaderVersionSelect.Disable()

		go func() {
			var loaders []string
			var err error
			switch selectedType {
			case modloader.Fabric:
				loaders, err = a.modInst.FetchFabricLoaders(mcVersion)
			case modloader.Forge:
				loaders, err = a.modInst.FetchForgeVersions(mcVersion)
			}
			if err != nil {
				statusLabel.SetText("Chyba: " + err.Error())
				return
			}
			if len(loaders) == 0 {
				statusLabel.SetText("Žádné verze pro MC " + mcVersion)
				return
			}
			loaderVersionSelect.Options = loaders
			loaderVersionSelect.SetSelected(loaders[0])
			loaderVersionSelect.Enable()
			statusLabel.SetText(fmt.Sprintf("Nalezeno %d verzí", len(loaders)))
		}()
	})

	installBtn := widget.NewButton("Instalovat", func() {
		if selectedType == modloader.None {
			return
		}
		mcVersion := extractVersionID(a.cfg.SelectedVersion)
		loaderVer := loaderVersionSelect.Selected
		if mcVersion == "" || loaderVer == "" {
			return
		}
		go func() {
			var err error
			switch selectedType {
			case modloader.Fabric:
				err = a.modInst.InstallFabric(mcVersion, loaderVer, func(msg string) {
					statusLabel.SetText(msg)
				})
			case modloader.Forge:
				statusLabel.SetText("Forge: použij oficiální Forge installer JAR")
				return
			}
			if err != nil {
				statusLabel.SetText("Chyba: " + err.Error())
			} else {
				statusLabel.SetText("Nainstalováno! Obnov seznam verzí.")
				go a.loadVersions()
			}
		}()
	})
	installBtn.Importance = widget.HighImportance

	// Opravený open mods folder
	openModsBtn := widget.NewButton("📂 Otevřít Mods složku", func() {
		modsDir := filepath.Join(a.cfg.GameDir, "mods")
		if err := os.MkdirAll(modsDir, 0755); err == nil {
			openFolder(modsDir)
		}
	})

	return container.NewPadded(container.NewVBox(
		widget.NewLabel("Mod Loader:"),
		typeSelect,
		widget.NewLabel("Loader verze:"),
		loaderVersionSelect,
		statusLabel,
		installBtn,
		widget.NewSeparator(),
		openModsBtn,
	))
}

func (a *App) buildConsoleTab() fyne.CanvasObject {
	a.logOutput = widget.NewTextGrid()
	a.logOutput.SetText("Výstup konzole se zobrazí po spuštění hry...\n")

	scroll := container.NewScroll(a.logOutput)
	scroll.SetMinSize(fyne.NewSize(860, 400))

	clearBtn := widget.NewButton("Smazat", func() {
		a.logOutput.SetText("")
	})

	return container.NewPadded(container.NewBorder(nil, clearBtn, nil, nil, scroll))
}

func (a *App) loadVersions() {
	a.setStatus("Načítám seznam verzí...")
	manifest, err := a.versionsMgr.FetchManifest()
	if err != nil {
		a.setStatus("Chyba: " + err.Error())
		return
	}
	a.manifest = manifest
	a.refreshVersionList()
}

func (a *App) refreshVersionList() {
	if a.manifest == nil {
		return
	}
	filtered := a.versionsMgr.FilterVersions(a.manifest, a.cfg.ShowSnapshots, a.cfg.ShowOld)
	var names []string
	for _, v := range filtered {
		label := fmt.Sprintf("[%s] %s", v.Type, v.ID)
		names = append(names, label)
	}
	a.versionSelect.Options = names
	if a.cfg.SelectedVersion != "" {
		for _, n := range names {
			if strings.Contains(n, extractVersionID(a.cfg.SelectedVersion)) {
				a.versionSelect.SetSelected(n)
				break
			}
		}
	}
	if a.versionSelect.Selected == "" && len(names) > 0 {
		a.versionSelect.SetSelected(names[0])
	}
	a.setStatus(fmt.Sprintf("Načteno %d verzí", len(names)))
}

func (a *App) onMSLogin() {
	dialog.ShowInformation("MS Login", "Microsoft login zatím není implementován v této verzi.\nPoužij Offline mód.", a.win)
}

func (a *App) onLaunch() {
	// Offline nebo MS účet
	var playerName, uuid, token string

	if a.cfg.OfflineMode || a.cfg.Account == nil {
		playerName = strings.TrimSpace(a.cfg.OfflineUsername)
		if playerName == "" {
			dialog.ShowError(fmt.Errorf("zadej hráčské jméno v hlavičce"), a.win)
			return
		}
		uuid = offlineUUID(playerName)
		token = "0"
	} else {
		playerName = a.cfg.Account.Username
		uuid = a.cfg.Account.UUID
		token = a.cfg.Account.AccessToken
	}

	selected := a.versionSelect.Selected
	if selected == "" {
		dialog.ShowError(fmt.Errorf("žádná verze není vybrána"), a.win)
		return
	}

	versionID := extractVersionID(selected)
	if a.manifest == nil {
		a.setStatus("Seznam verzí není načten")
		return
	}

	var entry versions.VersionEntry
	found := false
	for _, v := range a.manifest.Versions {
		if v.ID == versionID {
			entry = v
			found = true
			break
		}
	}
	if !found {
		dialog.ShowError(fmt.Errorf("verze %s nenalezena", versionID), a.win)
		return
	}

	a.launchBtn.Disable()
	a.progressBar.Show()
	a.setStatus("Připravuji...")

	go func() {
		defer func() {
			a.launchBtn.Enable()
			a.progressBar.Hide()
		}()

		a.setStatus("Načítám metadata verze...")
		meta, err := a.versionsMgr.FetchVersionMeta(entry)
		if err != nil {
			a.setStatus("Chyba: " + err.Error())
			return
		}

		cfg := &launcher.Config{
			GameDir:     a.cfg.GameDir,
			PlayerName:  playerName,
			UUID:        uuid,
			AccessToken: token,
			Version:     entry,
			VersionMeta: meta,
			JVM:         a.cfg.CustomJVM,
			JVMArgs:     a.cfg.ExtraJVMArgs,
			AllocMin:    512,
			AllocMax:    a.cfg.AllocMax,
			Fullscreen:  a.cfg.Fullscreen,
			OnStatus:    a.setStatus,
		}

		result, err := launcher.Launch(cfg, func(p assets.Progress) {
			if p.Total > 0 {
				pct := float64(p.Completed) / float64(p.Total)
				a.progressBar.SetValue(pct)
				a.setStatus(fmt.Sprintf("Stahuji... %d/%d", p.Completed, p.Total))
			}
		})

		if err != nil {
			a.setStatus("Spuštění selhalo: " + err.Error())
			return
		}

		a.setStatus("Minecraft běží! 🎮")
		a.logOutput.SetText(fmt.Sprintf("[%s] Minecraft %s spuštěn (PID %d)\n",
			time.Now().Format("15:04:05"), versionID, result.Cmd.Process.Pid))

		go streamOutput(result, a.logOutput)
	}()
}

func streamOutput(result *launcher.LaunchResult, grid *widget.TextGrid) {
	buf := make([]byte, 4096)
	var log strings.Builder
	for {
		n, err := result.Stderr.Read(buf)
		if n > 0 {
			log.Write(buf[:n])
			grid.SetText(log.String())
		}
		if err != nil {
			break
		}
	}
	result.Cmd.Wait()
}

func (a *App) setStatus(msg string) {
	a.statusLabel.SetText(msg)
}

// openFolder otevře složku v průzkumníku souborů
func openFolder(path string) {
	switch runtime.GOOS {
	case "windows":
		exec.Command("explorer", path).Start()
	case "darwin":
		exec.Command("open", path).Start()
	default:
		exec.Command("xdg-open", path).Start()
	}
}

func extractVersionID(selected string) string {
	parts := strings.SplitN(selected, "] ", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return selected
}

// offlineUUID generuje konzistentní UUID pro offline hráče (stejně jako vanilla)
func offlineUUID(username string) string {
	// Jednoduchý deterministický UUID z jména
	hash := uint64(0xcbf29ce484222325)
	for _, c := range "OfflinePlayer:" + username {
		hash ^= uint64(c)
		hash *= 0x100000001b3
	}
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		hash&0xffffffff,
		(hash>>32)&0xffff,
		0x3000|((hash>>48)&0x0fff),
		0x8000|((hash>>52)&0x3fff),
		hash&0xffffffffffff,
	)
}
