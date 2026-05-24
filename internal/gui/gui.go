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
		return color.NRGBA{R: 6, G: 15, B: 28, A: 255}
	case theme.ColorNameForeground:
		return color.NRGBA{R: 235, G: 244, B: 255, A: 255}
	case theme.ColorNameButton:
		return color.NRGBA{R: 0, G: 70, B: 135, A: 255}
	case theme.ColorNameDisabledButton:
		return color.NRGBA{R: 18, G: 28, B: 45, A: 255}
	case theme.ColorNameInputBackground:
		return color.NRGBA{R: 8, G: 20, B: 38, A: 255}
	case theme.ColorNamePlaceHolder:
		return color.NRGBA{R: 130, G: 165, B: 210, A: 255}
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 30, G: 144, B: 255, A: 255}
	case theme.ColorNameHover:
		return color.NRGBA{R: 30, G: 144, B: 255, A: 110}
	case theme.ColorNamePressed:
		return color.NRGBA{R: 0, G: 70, B: 135, A: 230}
	case theme.ColorNameFocus:
		return color.NRGBA{R: 30, G: 144, B: 255, A: 255}
	case theme.ColorNameSelection:
		return color.NRGBA{R: 30, G: 144, B: 255, A: 180}
	case theme.ColorNameSeparator:
		return color.NRGBA{R: 0, G: 70, B: 135, A: 255}
	case theme.ColorNameShadow:
		return color.NRGBA{R: 0, G: 0, B: 0, A: 160}
	case theme.ColorNameOverlayBackground:
		return color.NRGBA{R: 5, G: 12, B: 25, A: 245}
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

	contentBox    *fyne.Container
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
	a.win.Resize(fyne.NewSize(1180, 720))
	a.win.CenterOnScreen()

	if a.cfg.Account != nil && !a.cfg.OfflineMode {
		a.win.SetContent(a.buildMainUI())
		go a.loadVersions()
	} else {
		a.win.SetContent(a.buildLoginGate())
	}

	a.win.ShowAndRun()
}

func (a *App) buildLoginGate() fyne.CanvasObject {
	title := widget.NewLabelWithStyle("GoLauncher", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabelWithStyle("Minimal blue Minecraft launcher", fyne.TextAlignCenter, fyne.TextStyle{})

	loginMicrosoftBtn := widget.NewButton("Login to Microsoft", func() {
		a.loginFromGate()
	})
	loginMicrosoftBtn.Importance = widget.HighImportance

	loginElyBtn := widget.NewButton("Login to ely.by", func() {
		dialog.ShowInformation("ely.by", "ely.by login zatím není implementovaný.", a.win)
	})

	loginLittleSkinBtn := widget.NewButton("Login to LittleSkin", func() {
		dialog.ShowInformation("LittleSkin", "LittleSkin login zatím není implementovaný.", a.win)
	})

	orLabel := widget.NewLabelWithStyle("OR", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	usernameEntry := widget.NewEntry()
	usernameEntry.SetPlaceHolder("Enter username...")
	usernameEntry.SetText(a.cfg.OfflineUsername)

	continueBtn := widget.NewButton("Continue", func() {
		name := strings.TrimSpace(usernameEntry.Text)
		if name == "" {
			dialog.ShowError(fmt.Errorf("zadej hráčské jméno"), a.win)
			return
		}

		a.cfg.OfflineUsername = name
		a.cfg.OfflineMode = true
		a.cfg.Account = nil
		configs.Save(a.cfg)

		a.win.SetContent(a.buildMainUI())
		go a.loadVersions()
	})
	continueBtn.Importance = widget.HighImportance

	centerBox := container.NewVBox(
		title,
		subtitle,
		widget.NewSeparator(),
		loginMicrosoftBtn,
		loginElyBtn,
		loginLittleSkinBtn,
		orLabel,
		usernameEntry,
		continueBtn,
	)

	card := widget.NewCard("", "", centerBox)
	cardContainer := container.NewCenter(container.NewPadded(card))

	return container.NewBorder(nil, nil, nil, nil, cardContainer)
}

func (a *App) loginFromGate() {
	if a.loginBtn != nil {
		a.loginBtn.Disable()
	}

	account, err := auth.LoginMicrosoftLoopback()
	if err != nil {
		dialog.ShowError(fmt.Errorf("MS Login selhal: %w", err), a.win)
		if a.loginBtn != nil {
			a.loginBtn.Enable()
		}
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
	configs.Save(a.cfg)

	a.win.SetContent(a.buildMainUI())
	go a.loadVersions()
}

func (a *App) buildMainUI() fyne.CanvasObject {
	a.contentBox = container.NewMax()
	a.setPage(a.buildPlayPage())

	header := a.buildTopBar()
	sidebar := a.buildSidebar()

	body := container.NewBorder(nil, nil, sidebar, nil, container.NewPadded(a.contentBox))

	return container.NewBorder(header, nil, nil, nil, body)
}

func (a *App) buildTopBar() fyne.CanvasObject {
	playBtn := widget.NewButton("Play", func() {
		a.setPage(a.buildPlayPage())
	})
	playBtn.Importance = widget.HighImportance

	editBtn := widget.NewButton("Edit", func() {
		a.setPage(a.buildEditPage())
	})

	logsBtn := widget.NewButton("Logs", func() {
		a.setPage(a.buildConsolePage())
	})

	settingsBtn := widget.NewButtonWithIcon("", theme.SettingsIcon(), func() {
		a.setPage(a.buildSettingsPage())
	})

	title := widget.NewLabelWithStyle("GoLauncher", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	bar := container.NewBorder(nil, nil, title, nil, container.NewHBox(
		settingsBtn,
		playBtn,
		editBtn,
		logsBtn,
	))

	return container.NewPadded(widget.NewCard("", "", bar))
}

func (a *App) buildSidebar() fyne.CanvasObject {
	newBtn := a.navButton("+ New", func() {
		dialog.ShowInformation("New Instance", "Instance UI připravíme v dalším kroku.", a.win)
	})
	newBtn.Importance = widget.HighImportance

	openGameDirBtn := a.navButton("Open Game Dir", func() {
		openFolder(a.cfg.GameDir)
	})

	openModsBtn := a.navButton("Open Mods", func() {
		modsDir := filepath.Join(a.cfg.GameDir, "mods")
		if err := os.MkdirAll(modsDir, 0755); err == nil {
			openFolder(modsDir)
		}
	})

	openLogsBtn := a.navButton("Open Logs", func() {
		openFolder(filepath.Join(a.cfg.GameDir, "logs"))
	})

	accountTitle := widget.NewLabel("Accounts:")
	a.accountLabel = widget.NewLabel("")
	a.updateAccountLabel()

	logoutBtn := widget.NewButton("Logout", func() {
		a.cfg.Account = nil
		a.cfg.OfflineMode = true
		configs.Save(a.cfg)
		a.win.SetContent(a.buildLoginGate())
	})

	box := container.NewVBox(
		newBtn,
		widget.NewSeparator(),
		openGameDirBtn,
		openModsBtn,
		openLogsBtn,
		layout.NewSpacer(),
		widget.NewSeparator(),
		accountTitle,
		a.accountLabel,
		logoutBtn,
	)

	card := widget.NewCard("", "", box)

	return container.NewPadded(container.NewGridWrap(fyne.NewSize(240, 620), card))
}

func (a *App) navButton(text string, tapped func()) *widget.Button {
	btn := widget.NewButton(text, tapped)
	btn.Alignment = widget.ButtonAlignLeading
	return btn
}

func (a *App) setPage(obj fyne.CanvasObject) {
	if a.contentBox == nil {
		return
	}

	a.contentBox.Objects = []fyne.CanvasObject{obj}
	a.contentBox.Refresh()
}

func (a *App) buildPlayPage() fyne.CanvasObject {
	a.versionSelect = widget.NewSelect([]string{"Načítám verze..."}, func(v string) {
		a.cfg.SelectedVersion = v
		configs.Save(a.cfg)
	})

	if a.manifest != nil {
		a.refreshVersionList()
	}

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

	snapshotCheck := widget.NewCheck("Snapshots", func(v bool) {
		a.cfg.ShowSnapshots = v
		a.refreshVersionList()
		configs.Save(a.cfg)
	})
	snapshotCheck.SetChecked(a.cfg.ShowSnapshots)

	oldCheck := widget.NewCheck("Old versions", func(v bool) {
		a.cfg.ShowOld = v
		a.refreshVersionList()
		configs.Save(a.cfg)
	})
	oldCheck.SetChecked(a.cfg.ShowOld)

	a.progressBar = widget.NewProgressBar()
	a.progressBar.Hide()

	a.statusLabel = widget.NewLabel("Ready")
	a.statusLabel.Alignment = fyne.TextAlignCenter

	a.launchBtn = widget.NewButton("Launch Minecraft", a.onLaunch)
	a.launchBtn.Importance = widget.HighImportance

	versionCard := widget.NewCard("Version", "Choose Minecraft version.", container.NewVBox(
		a.versionSelect,
	))

	memoryCard := widget.NewCard("Memory", "Recommended: 2048–4096 MB.", container.NewVBox(
		a.ramLabel,
		a.ramSlider,
	))

	filterCard := widget.NewCard("Filters", "", container.NewVBox(
		snapshotCheck,
		oldCheck,
	))

	playCard := widget.NewCard("", "", container.NewVBox(
		widget.NewLabelWithStyle("Ready to play", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Select version and launch Minecraft.", fyne.TextAlignCenter, fyne.TextStyle{}),
		widget.NewSeparator(),
		a.statusLabel,
		a.progressBar,
		a.launchBtn,
	))

	grid := container.NewGridWithColumns(2, versionCard, memoryCard)

	return container.NewVBox(
		grid,
		filterCard,
		layout.NewSpacer(),
		playCard,
	)
}

func (a *App) buildEditPage() fyne.CanvasObject {
	typeSelect := widget.NewSelect([]string{"None", "Fabric", "Forge", "Quilt"}, func(string) {})
	typeSelect.SetSelected("None")

	loaderVersionSelect := widget.NewSelect([]string{}, func(string) {})
	loaderVersionSelect.Disable()

	statusLabel := widget.NewLabel("Select loader type to manage mod loader.")
	selectedType := modloader.None

	typeSelect.OnChanged = func(v string) {
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
			statusLabel.SetText("Mod loader disabled.")
			return
		}

		mcVersion := extractVersionID(a.cfg.SelectedVersion)
		if mcVersion == "" {
			statusLabel.SetText("Select Minecraft version in Play page.")
			return
		}

		statusLabel.SetText("Loading loader versions...")
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
	}

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

	addFileBtn := widget.NewButton("Add File", func() {
		dialog.ShowInformation("Add File", "Přidání mod souboru připravíme v dalším kroku.", a.win)
	})

	downloadContentBtn := widget.NewButton("Download Content", func() {
		dialog.ShowInformation("Mod Store", "Modrinth / CurseForge mod store připravíme v dalším kroku.", a.win)
	})

	openModsBtn := widget.NewButton("Open Mods Folder", func() {
		modsDir := filepath.Join(a.cfg.GameDir, "mods")
		if err := os.MkdirAll(modsDir, 0755); err == nil {
			openFolder(modsDir)
		}
	})

	actions := container.NewVBox(
		downloadContentBtn,
		addFileBtn,
		openModsBtn,
	)

	loaderCard := widget.NewCard("Mod Loader", "Install and manage loaders.", container.NewVBox(
		widget.NewLabel("Loader type"),
		typeSelect,
		widget.NewLabel("Loader version"),
		loaderVersionSelect,
		statusLabel,
		installBtn,
	))

	modStoreCard := widget.NewCard("Built-in mod store", "Planned minimalist mod browser.", container.NewVBox(
		widget.NewLabel("Install mods, resource packs, shaders and modpacks from one place."),
		actions,
	))

	return container.NewGridWithColumns(2, loaderCard, modStoreCard)
}

func (a *App) buildSettingsPage() fyne.CanvasObject {
	gameDirEntry := widget.NewEntry()
	gameDirEntry.SetText(a.cfg.GameDir)

	jvmEntry := widget.NewEntry()
	jvmEntry.SetPlaceHolder("Auto-detect")
	jvmEntry.SetText(a.cfg.CustomJVM)

	extraJVMEntry := widget.NewEntry()
	extraJVMEntry.SetPlaceHolder("-XX:+UseG1GC")
	extraJVMEntry.SetText(strings.Join(a.cfg.ExtraJVMArgs, " "))

	saveBtn := widget.NewButton("Save Settings", func() {
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

	openDirBtn := widget.NewButton("Open Game Directory", func() {
		openFolder(a.cfg.GameDir)
	})

	openLogsBtn := widget.NewButton("Open Logs", func() {
		openFolder(filepath.Join(a.cfg.GameDir, "logs"))
	})

	form := widget.NewForm(
		widget.NewFormItem("Game directory", gameDirEntry),
		widget.NewFormItem("Java path", jvmEntry),
		widget.NewFormItem("Extra JVM args", extraJVMEntry),
	)

	card := widget.NewCard("Settings", "Launcher configuration.", container.NewVBox(
		form,
		container.NewHBox(saveBtn, openDirBtn, openLogsBtn),
	))

	return container.NewVBox(card, layout.NewSpacer())
}

func (a *App) buildConsolePage() fyne.CanvasObject {
	a.logOutput = widget.NewTextGrid()
	a.logOutput.SetText("Console is ready.\nMinecraft log is in game directory/logs.\n")

	scroll := container.NewScroll(a.logOutput)
	scroll.SetMinSize(fyne.NewSize(760, 460))

	clearBtn := widget.NewButton("Clear", func() {
		a.logOutput.SetText("")
	})

	openLogsBtn := widget.NewButton("Open Logs", func() {
		openFolder(filepath.Join(a.cfg.GameDir, "logs"))
	})

	card := widget.NewCard("Logs", "Minecraft output and launcher state.", container.NewBorder(nil, container.NewHBox(clearBtn, openLogsBtn), nil, nil, scroll))

	return container.NewVBox(card, layout.NewSpacer())
}

func (a *App) loadVersions() {
	a.setStatus("Loading versions...")

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

	a.setStatus(fmt.Sprintf("Loaded %d versions.", len(names)))
}

func (a *App) onMSLogin() {
	a.setStatus("Logging in with Microsoft...")

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
			a.setStatus("MS Login failed: " + err.Error())
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
			a.setStatus("Failed to save account: " + err.Error())
			return
		}

		a.updateAccountLabel()
		a.setStatus("Logged in as " + account.Username)
	}()
}

func (a *App) onLaunch() {
	var playerName string
	var uuid string
	var token string

	if a.cfg.OfflineMode || a.cfg.Account == nil {
		playerName = strings.TrimSpace(a.cfg.OfflineUsername)
		if playerName == "" {
			dialog.ShowError(fmt.Errorf("zadej hráčské jméno"), a.win)
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
		a.setStatus("Version list is not loaded.")
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
	a.setStatus("Preparing...")

	go func() {
		defer func() {
			a.launchBtn.Enable()
			a.progressBar.Hide()
		}()

		a.setStatus("Loading version metadata...")

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
				a.setStatus(fmt.Sprintf("Downloading... %d/%d", p.Completed, p.Total))
			}
		})

		if err != nil {
			a.setStatus("Launch failed: " + err.Error())
			return
		}

		logPath := filepath.Join(a.cfg.GameDir, "logs", "golauncher-latest.log")

		a.setStatus("Minecraft is running.")
		if a.logOutput != nil {
			a.logOutput.SetText(fmt.Sprintf("[%s] Minecraft %s launched\nPID: %d\nPlayer: %s\nLog: %s\n",
				time.Now().Format("15:04:05"),
				versionID,
				result.Cmd.Process.Pid,
				playerName,
				logPath,
			))
		}

		go func() {
			_ = result.Cmd.Wait()
			a.setStatus("Minecraft closed.")
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

	a.accountLabel.SetText("Offline")
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
