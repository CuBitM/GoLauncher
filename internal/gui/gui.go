package gui

import (
	"fmt"
	"image/color"
	"mclauncher/configs"
	"mclauncher/internal/assets"
	"mclauncher/internal/auth"
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
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type blueTheme struct{}

func (blueTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.NRGBA{R: 8, G: 12, B: 24, A: 255}
	case theme.ColorNameForeground:
		return color.NRGBA{R: 232, G: 240, B: 255, A: 255}
	case theme.ColorNameButton:
		return color.NRGBA{R: 18, G: 28, B: 52, A: 255}
	case theme.ColorNameDisabledButton:
		return color.NRGBA{R: 20, G: 24, B: 36, A: 255}
	case theme.ColorNameInputBackground:
		return color.NRGBA{R: 12, G: 18, B: 34, A: 255}
	case theme.ColorNamePlaceHolder:
		return color.NRGBA{R: 115, G: 130, B: 160, A: 255}
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 52, G: 144, B: 255, A: 255}
	case theme.ColorNameHover:
		return color.NRGBA{R: 38, G: 90, B: 170, A: 120}
	case theme.ColorNamePressed:
		return color.NRGBA{R: 35, G: 120, B: 230, A: 180}
	case theme.ColorNameFocus:
		return color.NRGBA{R: 64, G: 156, B: 255, A: 255}
	case theme.ColorNameSelection:
		return color.NRGBA{R: 38, G: 112, B: 220, A: 180}
	case theme.ColorNameSeparator:
		return color.NRGBA{R: 35, G: 48, B: 78, A: 255}
	case theme.ColorNameShadow:
		return color.NRGBA{R: 0, G: 0, B: 0, A: 120}
	case theme.ColorNameOverlayBackground:
		return color.NRGBA{R: 10, G: 14, B: 26, A: 245}
	default:
		return theme.DarkTheme().Color(name, variant)
	}
}

func (blueTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DarkTheme().Font(style)
}

func (blueTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DarkTheme().Icon(name)
}

func (blueTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 10
	case theme.SizeNameInnerPadding:
		return 8
	case theme.SizeNameText:
		return 14
	case theme.SizeNameHeadingText:
		return 24
	case theme.SizeNameSubHeadingText:
		return 18
	default:
		return theme.DarkTheme().Size(name)
	}
}

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
	loginBtn      *widget.Button
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
	a.fyneApp.Settings().SetTheme(blueTheme{})

	a.win = a.fyneApp.NewWindow("GoLauncher")
	a.win.Resize(fyne.NewSize(1120, 700))
	a.win.CenterOnScreen()
	a.win.SetContent(a.buildUI())

	go a.loadVersions()

	a.win.ShowAndRun()
}

func (a *App) buildUI() fyne.CanvasObject {
	header := a.buildHeader()

	tabs := container.NewAppTabs(
		container.NewTabItem("Launch", a.buildLaunchTab()),
		container.NewTabItem("Settings", a.buildSettingsTab()),
		container.NewTabItem("Mod Loaders", a.buildModLoadersTab()),
		container.NewTabItem("Console", a.buildConsoleTab()),
	)

	tabs.SetTabLocation(container.TabLocationTop)

	return container.NewBorder(header, nil, nil, nil, tabs)
}

func (a *App) buildHeader() fyne.CanvasObject {
	title := widget.NewLabelWithStyle("GoLauncher", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabel("Fast, clean and blue Minecraft launcher")

	a.accountLabel = widget.NewLabel("")
	a.updateAccountLabel()

	usernameEntry := widget.NewEntry()
	usernameEntry.SetPlaceHolder("Offline username")
	usernameEntry.SetText(a.cfg.OfflineUsername)

	offlineBtn := widget.NewButton("Offline", func() {
		name := strings.TrimSpace(usernameEntry.Text)
		if name == "" {
			dialog.ShowError(fmt.Errorf("zadej hráčské jméno"), a.win)
			return
		}

		a.cfg.OfflineUsername = name
		a.cfg.OfflineMode = true
		a.cfg.Account = nil

		configs.Save(a.cfg)

		a.updateAccountLabel()
		a.setStatus("Offline mód: " + name)
	})
	offlineBtn.Importance = widget.MediumImportance

	a.loginBtn = widget.NewButton("MS Login", a.onMSLogin)
	a.loginBtn.Importance = widget.HighImportance

	left := container.NewVBox(title, subtitle)

	right := container.NewHBox(
		usernameEntry,
		offlineBtn,
		a.loginBtn,
		widget.NewSeparator(),
		a.accountLabel,
	)

	return container.NewPadded(widget.NewCard("", "", container.NewBorder(nil, nil, left, right)))
}

func (a *App) buildLaunchTab() fyne.CanvasObject {
	a.versionSelect = widget.NewSelect([]string{"Načítám verze..."}, func(v string) {
		a.cfg.SelectedVersion = v
		configs.Save(a.cfg)
	})

	versionCard := widget.NewCard("Minecraft Version", "Vyber verzi hry.", container.NewVBox(
		a.versionSelect,
	))

	if a.cfg.AllocMax <= 0 {
		a.cfg.AllocMax = 2048
	}

	a.ramLabel = widget.NewLabel(fmt.Sprintf("RAM: %d MB", a.cfg.AllocMax))
	a.ramSlider = widget.NewSlider(512, 16384)
	a.ramSlider.Step = 256
	a.ramSlider.SetValue(float64(a.cfg.AllocMax))
	a.ramSlider.OnChanged = func(v float64) {
		a.cfg.AllocMax = int(v)
		a.ramLabel.SetText(fmt.Sprintf("RAM: %d MB", int(v)))
		configs.Save(a.cfg)
	}

	ramCard := widget.NewCard("Memory", "Doporučeno pro 1.16.5: 2048–4096 MB.", container.NewVBox(
		a.ramLabel,
		a.ramSlider,
	))

	snapshotCheck := widget.NewCheck("Show snapshots", func(v bool) {
		a.cfg.ShowSnapshots = v
		a.refreshVersionList()
		configs.Save(a.cfg)
	})
	snapshotCheck.SetChecked(a.cfg.ShowSnapshots)

	oldCheck := widget.NewCheck("Show old versions", func(v bool) {
		a.cfg.ShowOld = v
		a.refreshVersionList()
		configs.Save(a.cfg)
	})
	oldCheck.SetChecked(a.cfg.ShowOld)

	optionsCard := widget.NewCard("Options", "Filtry verzí.", container.NewVBox(
		snapshotCheck,
		oldCheck,
	))

	a.progressBar = widget.NewProgressBar()
	a.progressBar.Hide()

	a.statusLabel = widget.NewLabel("Připraven")
	a.statusLabel.Alignment = fyne.TextAlignCenter

	a.launchBtn = widget.NewButton("Launch Minecraft", a.onLaunch)
	a.launchBtn.Importance = widget.HighImportance

	mainGrid := container.NewGridWithColumns(2,
		versionCard,
		ramCard,
	)

	heroTitle := widget.NewLabelWithStyle("Ready to play", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	heroSubtitle := widget.NewLabelWithStyle("Vyber verzi, nastav RAM a spusť Minecraft.", fyne.TextAlignCenter, fyne.TextStyle{})

	bottomCard := widget.NewCard("", "", container.NewVBox(
		heroTitle,
		heroSubtitle,
		widget.NewSeparator(),
		a.statusLabel,
		a.progressBar,
		a.launchBtn,
	))

	return container.NewPadded(container.NewVBox(
		mainGrid,
		optionsCard,
		layout.NewSpacer(),
		bottomCard,
	))
}

func (a *App) buildSettingsTab() fyne.CanvasObject {
	gameDirEntry := widget.NewEntry()
	gameDirEntry.SetText(a.cfg.GameDir)

	jvmEntry := widget.NewEntry()
	jvmEntry.SetPlaceHolder("Auto-detect")
	jvmEntry.SetText(a.cfg.CustomJVM)

	extraJVMEntry := widget.NewEntry()
	extraJVMEntry.SetPlaceHolder("-XX:+UseG1GC")
	extraJVMEntry.SetText(strings.Join(a.cfg.ExtraJVMArgs, " "))

	saveBtn := widget.NewButton("Save settings", func() {
		a.cfg.GameDir = strings.TrimSpace(gameDirEntry.Text)
		a.cfg.CustomJVM = strings.TrimSpace(jvmEntry.Text)

		if strings.TrimSpace(extraJVMEntry.Text) != "" {
			a.cfg.ExtraJVMArgs = strings.Fields(extraJVMEntry.Text)
		} else {
			a.cfg.ExtraJVMArgs = nil
		}

		configs.Save(a.cfg)

		a.versionsMgr = versions.NewManager(a.cfg.GameDir)
		a.modInst = modloader.NewInstaller(a.cfg.GameDir)

		dialog.ShowInformation("Saved", "Nastavení uloženo.", a.win)
	})
	saveBtn.Importance = widget.HighImportance

	openDirBtn := widget.NewButton("Open game directory", func() {
		openFolder(a.cfg.GameDir)
	})

	openLogsBtn := widget.NewButton("Open logs", func() {
		openFolder(filepath.Join(a.cfg.GameDir, "logs"))
	})

	form := widget.NewForm(
		widget.NewFormItem("Game directory", gameDirEntry),
		widget.NewFormItem("Java path", jvmEntry),
		widget.NewFormItem("Extra JVM args", extraJVMEntry),
	)

	settingsCard := widget.NewCard("Settings", "Základní nastavení launcheru.", container.NewVBox(
		form,
		container.NewHBox(saveBtn, openDirBtn, openLogsBtn),
	))

	return container.NewPadded(settingsCard)
}

func (a *App) buildModLoadersTab() fyne.CanvasObject {
	statusLabel := widget.NewLabel("Vyber Minecraft verzi v Launch tabu.")
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
			loaderVersionSelect.Options = nil
			loaderVersionSelect.ClearSelected()
			loaderVersionSelect.Disable()
			statusLabel.SetText("Mod loader vypnutý.")
			return
		}

		mcVersion := extractVersionID(a.cfg.SelectedVersion)
		if mcVersion == "" {
			statusLabel.SetText("Vyber MC verzi v Launch tabu.")
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
			case modloader.Quilt:
				loaders = nil
				err = fmt.Errorf("Quilt zatím není implementovaný")
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
			statusLabel.SetText(fmt.Sprintf("Nalezeno %d verzí.", len(loaders)))
		}()
	})

	var installBtn *widget.Button

	installBtn = widget.NewButton("Install loader", func() {
		if selectedType == modloader.None {
			statusLabel.SetText("Vyber mod loader.")
			return
		}

		mcVersion := extractVersionID(a.cfg.SelectedVersion)
		loaderVer := loaderVersionSelect.Selected

		if mcVersion == "" || loaderVer == "" {
			statusLabel.SetText("Vyber MC verzi a loader verzi.")
			return
		}

		installBtn.Disable()
		statusLabel.SetText("Instaluji...")

		go func() {
			defer installBtn.Enable()

			var err error

			switch selectedType {
			case modloader.Fabric:
				err = a.modInst.InstallFabric(mcVersion, loaderVer, func(msg string) {
					statusLabel.SetText(msg)
				})
			case modloader.Forge:
				statusLabel.SetText("Forge auto install připravíme v dalším kroku.")
				return
			case modloader.Quilt:
				statusLabel.SetText("Quilt zatím není implementovaný.")
				return
			}

			if err != nil {
				statusLabel.SetText("Chyba: " + err.Error())
				return
			}

			statusLabel.SetText("Nainstalováno. Obnovuji verze...")
			go a.loadVersions()
		}()
	})
	installBtn.Importance = widget.HighImportance

	openModsBtn := widget.NewButton("Open mods folder", func() {
		modsDir := filepath.Join(a.cfg.GameDir, "mods")
		if err := os.MkdirAll(modsDir, 0755); err == nil {
			openFolder(modsDir)
		}
	})

	card := widget.NewCard("Mod Loaders", "Instalace mod loaderů.", container.NewVBox(
		widget.NewLabel("Loader type"),
		typeSelect,
		widget.NewLabel("Loader version"),
		loaderVersionSelect,
		statusLabel,
		container.NewHBox(installBtn, openModsBtn),
	))

	return container.NewPadded(card)
}

func (a *App) buildConsoleTab() fyne.CanvasObject {
	a.logOutput = widget.NewTextGrid()
	a.logOutput.SetText("Console je připravená.\nLog Minecraftu najdeš v game directory/logs.\n")

	scroll := container.NewScroll(a.logOutput)
	scroll.SetMinSize(fyne.NewSize(900, 460))

	clearBtn := widget.NewButton("Clear", func() {
		a.logOutput.SetText("")
	})

	openLogsBtn := widget.NewButton("Open logs", func() {
		openFolder(filepath.Join(a.cfg.GameDir, "logs"))
	})

	return container.NewPadded(container.NewBorder(nil, container.NewHBox(clearBtn, openLogsBtn), nil, nil, scroll))
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
	if a.manifest == nil || a.versionSelect == nil {
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

	a.setStatus(fmt.Sprintf("Načteno %d verzí.", len(names)))
}

func (a *App) onMSLogin() {
	a.setStatus("Přihlašuji přes Microsoft...")

	if a.loginBtn != nil {
		a.loginBtn.Disable()
	}

	go func() {
		defer func() {
			if a.loginBtn != nil {
				a.loginBtn.Enable()
			}
		}()

		account, err := auth.LoginMicrosoftLoopback()
		if err != nil {
			a.setStatus("MS Login selhal: " + err.Error())
			return
		}

		a.cfg.Account = &configs.AccountConfig{
			Username:     account.Username,
			UUID:         account.UUID,
			AccessToken:  account.AccessToken,
			RefreshToken: account.RefreshToken,
		}

		a.cfg.OfflineMode = false
		a.cfg.OfflineUsername = account.Username

		if err := configs.Save(a.cfg); err != nil {
			a.setStatus("Nepodařilo se uložit účet: " + err.Error())
			return
		}

		a.updateAccountLabel()
		a.setStatus("Přihlášeno jako " + account.Username)
	}()
}

func (a *App) onLaunch() {
	var playerName string
	var uuid string
	var token string

	if a.cfg.OfflineMode || a.cfg.Account == nil {
		playerName = strings.TrimSpace(a.cfg.OfflineUsername)
		if playerName == "" {
			dialog.ShowError(fmt.Errorf("zadej hráčské jméno nahoře"), a.win)
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
		a.setStatus("Seznam verzí není načten.")
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
	a.progressBar.SetValue(0)
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
			Fullscreen:  false,
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

		logPath := filepath.Join(a.cfg.GameDir, "logs", "golauncher-latest.log")

		a.setStatus("Minecraft běží.")
		a.logOutput.SetText(fmt.Sprintf("[%s] Minecraft %s spuštěn\nPID: %d\nPlayer: %s\nLog: %s\n",
			time.Now().Format("15:04:05"),
			versionID,
			result.Cmd.Process.Pid,
			playerName,
			logPath,
		))

		go func() {
			_ = result.Cmd.Wait()
			a.setStatus("Minecraft ukončen.")
		}()
	}()
}

func (a *App) setStatus(msg string) {
	if a.statusLabel != nil {
		a.statusLabel.SetText(msg)
	}
}

func (a *App) updateAccountLabel() {
	if a.accountLabel == nil {
		return
	}

	if a.cfg.Account != nil && !a.cfg.OfflineMode {
		a.accountLabel.SetText("Online: " + a.cfg.Account.Username)
		return
	}

	if strings.TrimSpace(a.cfg.OfflineUsername) != "" {
		a.accountLabel.SetText("Offline: " + a.cfg.OfflineUsername)
		return
	}

	a.accountLabel.SetText("Offline mode")
}

func openFolder(path string) {
	if path == "" {
		return
	}

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

func offlineUUID(username string) string {
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
