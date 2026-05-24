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
		return color.NRGBA{R: 5, G: 12, B: 24, A: 255}
	case theme.ColorNameForeground:
		return color.NRGBA{R: 235, G: 244, B: 255, A: 255}
	case theme.ColorNameButton:
		return color.NRGBA{R: 0, G: 70, B: 135, A: 255}
	case theme.ColorNameDisabledButton:
		return color.NRGBA{R: 16, G: 26, B: 42, A: 255}
	case theme.ColorNameInputBackground:
		return color.NRGBA{R: 7, G: 18, B: 35, A: 255}
	case theme.ColorNamePlaceHolder:
		return color.NRGBA{R: 125, G: 165, B: 215, A: 255}
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 30, G: 144, B: 255, A: 255}
	case theme.ColorNameHover:
		return color.NRGBA{R: 30, G: 144, B: 255, A: 115}
	case theme.ColorNamePressed:
		return color.NRGBA{R: 0, G: 70, B: 135, A: 235}
	case theme.ColorNameFocus:
		return color.NRGBA{R: 30, G: 144, B: 255, A: 255}
	case theme.ColorNameSelection:
		return color.NRGBA{R: 30, G: 144, B: 255, A: 180}
	case theme.ColorNameSeparator:
		return color.NRGBA{R: 0, G: 70, B: 135, A: 255}
	case theme.ColorNameShadow:
		return color.NRGBA{R: 0, G: 0, B: 0, A: 170}
	case theme.ColorNameOverlayBackground:
		return color.NRGBA{R: 5, G: 12, B: 24, A: 245}
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

	lang          string
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
	gateLoginBtn  *widget.Button
}

func NewApp() *App {
	cfg, _ := configs.Load()

	return &App{
		cfg:         cfg,
		versionsMgr: versions.NewManager(cfg.GameDir),
		modInst:     modloader.NewInstaller(cfg.GameDir),
		lang:        "en",
	}
}

func (a *App) Run() {
	a.fyneApp = app.New()
	a.fyneApp.Settings().SetTheme(blueTheme{})

	a.win = a.fyneApp.NewWindow("GoLauncher")
	a.win.Resize(fyne.NewSize(1180, 720))
	a.win.CenterOnScreen()
	a.win.SetContent(a.buildLanguageGate())

	a.win.ShowAndRun()
}

func (a *App) tr(key string) string {
	translations := map[string]map[string]string{
		"en": {
			"language":              "Choose language",
			"english":               "English",
			"czech":                 "Čeština",
			"russian":               "Русский",
			"fast":                  "Fast Minecraft launcher",
			"loginMicrosoft":        "Login to Microsoft",
			"loginEly":              "Login to ely.by",
			"loginLittleSkin":       "Login to LittleSkin",
			"or":                    "OR",
			"usernamePlaceholder":   "Enter username...",
			"continue":              "Continue",
			"emptyUsername":         "Enter username",
			"elyNotImplemented":     "ely.by login is not implemented yet.",
			"littleNotImplemented":  "LittleSkin login is not implemented yet.",
			"play":                  "Play",
			"edit":                  "Edit",
			"logs":                  "Logs",
			"new":                   "+ New",
			"newInstance":           "Instance UI will be added later.",
			"openGameDir":           "Open Game Dir",
			"openMods":              "Open Mods",
			"openLogs":              "Open Logs",
			"accounts":              "Accounts:",
			"logout":                "Logout",
			"version":               "Version",
			"versionDesc":           "Choose Minecraft version.",
			"memory":                "Memory",
			"memoryDesc":            "Recommended: 2048–4096 MB.",
			"snapshots":             "Snapshots",
			"oldVersions":           "Old versions",
			"filters":               "Filters",
			"ready":                 "Ready",
			"readyToPlay":           "Ready to play",
			"readyDesc":             "Select version and launch Minecraft.",
			"launch":                "Launch Minecraft",
			"settings":              "Settings",
			"settingsDesc":          "Launcher configuration.",
			"gameDirectory":         "Game directory",
			"javaPath":              "Java path",
			"extraJvmArgs":          "Extra JVM args",
			"saveSettings":          "Save Settings",
			"saved":                 "Saved",
			"savedText":             "Settings saved.",
			"loaderType":            "Loader type",
			"loaderVersion":         "Loader version",
			"modLoader":             "Mod Loader",
			"modLoaderDesc":         "Install and manage loaders.",
			"installLoader":         "Install loader",
			"downloadContent":       "Download Content",
			"addFile":               "Add File",
			"openModsFolder":        "Open Mods Folder",
			"modStore":              "Built-in mod store",
			"modStoreDesc":          "Minimalist mod browser planned.",
			"modStoreText":          "Install mods, resource packs, shaders and modpacks from one place.",
			"consoleReady":          "Console is ready.\nMinecraft log is in game directory/logs.\n",
			"clear":                 "Clear",
			"loadingVersions":       "Loading versions...",
			"loadedVersions":        "Loaded %d versions.",
			"loginRunning":          "Logging in with Microsoft...",
			"loginFailed":           "MS Login failed: %s",
			"loggedIn":              "Logged in as %s",
			"saveAccountFailed":     "Failed to save account: %s",
			"offline":               "Offline: %s",
			"online":                "Online: %s",
			"offlineShort":          "Offline",
			"versionNotLoaded":      "Version list is not loaded.",
			"versionNotFound":       "Version %s not found",
			"noVersion":             "No version selected",
			"preparing":             "Preparing...",
			"loadingMeta":           "Loading version metadata...",
			"downloading":           "Downloading... %d/%d",
			"launchFailed":          "Launch failed: %s",
			"running":               "Minecraft is running.",
			"closed":                "Minecraft closed.",
			"selectLoader":          "Select loader type to manage mod loader.",
			"loaderDisabled":        "Mod loader disabled.",
			"selectVersionPlay":     "Select Minecraft version in Play page.",
			"loadingLoaderVersions": "Loading loader versions...",
			"noLoaders":             "No versions for MC %s",
			"foundLoaders":          "Found %d versions.",
			"chooseLoader":          "Choose mod loader.",
			"chooseVersionLoader":   "Choose MC version and loader version.",
			"installing":            "Installing...",
			"installed":             "Installed. Refreshing versions...",
			"forgeLater":            "Forge auto install will be added later.",
			"quiltLater":            "Quilt is not implemented yet.",
			"addFileLater":          "Adding mod files will be added later.",
			"storeLater":            "Modrinth / CurseForge store will be added later.",
		},
		"cs": {
			"language":              "Vyber jazyk",
			"english":               "English",
			"czech":                 "Čeština",
			"russian":               "Русский",
			"fast":                  "Rychlý Minecraft launcher",
			"loginMicrosoft":        "Přihlásit přes Microsoft",
			"loginEly":              "Přihlásit přes ely.by",
			"loginLittleSkin":       "Přihlásit přes LittleSkin",
			"or":                    "NEBO",
			"usernamePlaceholder":   "Zadej jméno...",
			"continue":              "Pokračovat",
			"emptyUsername":         "Zadej hráčské jméno",
			"elyNotImplemented":     "ely.by login zatím není implementovaný.",
			"littleNotImplemented":  "LittleSkin login zatím není implementovaný.",
			"play":                  "Hrát",
			"edit":                  "Upravit",
			"logs":                  "Logy",
			"new":                   "+ Nová",
			"newInstance":           "Instance UI připravíme později.",
			"openGameDir":           "Otevřít game dir",
			"openMods":              "Otevřít mods",
			"openLogs":              "Otevřít logy",
			"accounts":              "Účty:",
			"logout":                "Odhlásit",
			"version":               "Verze",
			"versionDesc":           "Vyber Minecraft verzi.",
			"memory":                "Paměť",
			"memoryDesc":            "Doporučeno: 2048–4096 MB.",
			"snapshots":             "Snapshoty",
			"oldVersions":           "Staré verze",
			"filters":               "Filtry",
			"ready":                 "Připraven",
			"readyToPlay":           "Připraveno ke hraní",
			"readyDesc":             "Vyber verzi a spusť Minecraft.",
			"launch":                "Spustit Minecraft",
			"settings":              "Nastavení",
			"settingsDesc":          "Konfigurace launcheru.",
			"gameDirectory":         "Game directory",
			"javaPath":              "Java path",
			"extraJvmArgs":          "Extra JVM args",
			"saveSettings":          "Uložit nastavení",
			"saved":                 "Uloženo",
			"savedText":             "Nastavení uloženo.",
			"loaderType":            "Typ loaderu",
			"loaderVersion":         "Verze loaderu",
			"modLoader":             "Mod Loader",
			"modLoaderDesc":         "Instalace a správa loaderů.",
			"installLoader":         "Instalovat loader",
			"downloadContent":       "Stáhnout obsah",
			"addFile":               "Přidat soubor",
			"openModsFolder":        "Otevřít mods složku",
			"modStore":              "Vestavěný mod store",
			"modStoreDesc":          "Minimalistický prohlížeč modů v plánu.",
			"modStoreText":          "Instaluj mody, resource packy, shadery a modpacky z jednoho místa.",
			"consoleReady":          "Console je připravená.\nMinecraft log najdeš v game directory/logs.\n",
			"clear":                 "Smazat",
			"loadingVersions":       "Načítám verze...",
			"loadedVersions":        "Načteno %d verzí.",
			"loginRunning":          "Přihlašuji přes Microsoft...",
			"loginFailed":           "MS Login selhal: %s",
			"loggedIn":              "Přihlášeno jako %s",
			"saveAccountFailed":     "Nepodařilo se uložit účet: %s",
			"offline":               "Offline: %s",
			"online":                "Online: %s",
			"offlineShort":          "Offline",
			"versionNotLoaded":      "Seznam verzí není načten.",
			"versionNotFound":       "Verze %s nenalezena",
			"noVersion":             "Žádná verze není vybrána",
			"preparing":             "Připravuji...",
			"loadingMeta":           "Načítám metadata verze...",
			"downloading":           "Stahuji... %d/%d",
			"launchFailed":          "Spuštění selhalo: %s",
			"running":               "Minecraft běží.",
			"closed":                "Minecraft ukončen.",
			"selectLoader":          "Vyber typ loaderu pro správu mod loaderu.",
			"loaderDisabled":        "Mod loader vypnutý.",
			"selectVersionPlay":     "Vyber Minecraft verzi v Play stránce.",
			"loadingLoaderVersions": "Načítám loader verze...",
			"noLoaders":             "Žádné verze pro MC %s",
			"foundLoaders":          "Nalezeno %d verzí.",
			"chooseLoader":          "Vyber mod loader.",
			"chooseVersionLoader":   "Vyber MC verzi a loader verzi.",
			"installing":            "Instaluji...",
			"installed":             "Nainstalováno. Obnovuji verze...",
			"forgeLater":            "Forge auto install připravíme později.",
			"quiltLater":            "Quilt zatím není implementovaný.",
			"addFileLater":          "Přidání mod souborů připravíme později.",
			"storeLater":            "Modrinth / CurseForge store připravíme později.",
		},
		"ru": {
			"language":              "Выберите язык",
			"english":               "English",
			"czech":                 "Čeština",
			"russian":               "Русский",
			"fast":                  "Быстрый Minecraft лаунчер",
			"loginMicrosoft":        "Войти через Microsoft",
			"loginEly":              "Войти через ely.by",
			"loginLittleSkin":       "Войти через LittleSkin",
			"or":                    "ИЛИ",
			"usernamePlaceholder":   "Введите ник...",
			"continue":              "Продолжить",
			"emptyUsername":         "Введите ник",
			"elyNotImplemented":     "ely.by login пока не реализован.",
			"littleNotImplemented":  "LittleSkin login пока не реализован.",
			"play":                  "Играть",
			"edit":                  "Изменить",
			"logs":                  "Логи",
			"new":                   "+ Новая",
			"newInstance":           "Интерфейс инстансов будет добавлен позже.",
			"openGameDir":           "Открыть game dir",
			"openMods":              "Открыть mods",
			"openLogs":              "Открыть логи",
			"accounts":              "Аккаунты:",
			"logout":                "Выйти",
			"version":               "Версия",
			"versionDesc":           "Выберите версию Minecraft.",
			"memory":                "Память",
			"memoryDesc":            "Рекомендуется: 2048–4096 MB.",
			"snapshots":             "Снапшоты",
			"oldVersions":           "Старые версии",
			"filters":               "Фильтры",
			"ready":                 "Готово",
			"readyToPlay":           "Готово к игре",
			"readyDesc":             "Выберите версию и запустите Minecraft.",
			"launch":                "Запустить Minecraft",
			"settings":              "Настройки",
			"settingsDesc":          "Настройки лаунчера.",
			"gameDirectory":         "Game directory",
			"javaPath":              "Java path",
			"extraJvmArgs":          "Extra JVM args",
			"saveSettings":          "Сохранить",
			"saved":                 "Сохранено",
			"savedText":             "Настройки сохранены.",
			"loaderType":            "Тип loaderu",
			"loaderVersion":         "Версия loaderu",
			"modLoader":             "Mod Loader",
			"modLoaderDesc":         "Установка и управление loader.",
			"installLoader":         "Установить loader",
			"downloadContent":       "Скачать контент",
			"addFile":               "Добавить файл",
			"openModsFolder":        "Открыть mods папку",
			"modStore":              "Встроенный mod store",
			"modStoreDesc":          "Минималистичный браузер модов в планах.",
			"modStoreText":          "Устанавливайте моды, resource packs, shaders и modpacks из одного места.",
			"consoleReady":          "Console готова.\nMinecraft log находится в game directory/logs.\n",
			"clear":                 "Очистить",
			"loadingVersions":       "Загрузка версий...",
			"loadedVersions":        "Загружено %d версий.",
			"loginRunning":          "Вход через Microsoft...",
			"loginFailed":           "MS Login failed: %s",
			"loggedIn":              "Вход выполнен как %s",
			"saveAccountFailed":     "Не удалось сохранить аккаунт: %s",
			"offline":               "Offline: %s",
			"online":                "Online: %s",
			"offlineShort":          "Offline",
			"versionNotLoaded":      "Список версий не загружен.",
			"versionNotFound":       "Версия %s не найдена",
			"noVersion":             "Версия не выбрана",
			"preparing":             "Подготовка...",
			"loadingMeta":           "Загрузка metadata версии...",
			"downloading":           "Загрузка... %d/%d",
			"launchFailed":          "Ошибка запуска: %s",
			"running":               "Minecraft запущен.",
			"closed":                "Minecraft закрыт.",
			"selectLoader":          "Выберите тип loader.",
			"loaderDisabled":        "Mod loader выключен.",
			"selectVersionPlay":     "Выберите Minecraft версию на Play странице.",
			"loadingLoaderVersions": "Загрузка loader версий...",
			"noLoaders":             "Нет версий для MC %s",
			"foundLoaders":          "Найдено %d версий.",
			"chooseLoader":          "Выберите mod loader.",
			"chooseVersionLoader":   "Выберите MC версию и loader версию.",
			"installing":            "Установка...",
			"installed":             "Установлено. Обновление версий...",
			"forgeLater":            "Forge auto install будет добавлен позже.",
			"quiltLater":            "Quilt пока не реализован.",
			"addFileLater":          "Добавление mod файлов будет позже.",
			"storeLater":            "Modrinth / CurseForge store будет позже.",
		},
	}

	if translations[a.lang] == nil {
		a.lang = "en"
	}

	if v, ok := translations[a.lang][key]; ok {
		return v
	}

	return translations["en"][key]
}

func (a *App) buildLanguageGate() fyne.CanvasObject {
	title := widget.NewLabelWithStyle("GoLauncher", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	choose := widget.NewLabelWithStyle(a.tr("language"), fyne.TextAlignCenter, fyne.TextStyle{})

	enBtn := widget.NewButton(a.tr("english"), func() {
		a.lang = "en"
		a.win.SetContent(a.buildLoginGate())
	})
	enBtn.Importance = widget.HighImportance

	csBtn := widget.NewButton(a.tr("czech"), func() {
		a.lang = "cs"
		a.win.SetContent(a.buildLoginGate())
	})
	csBtn.Importance = widget.HighImportance

	ruBtn := widget.NewButton(a.tr("russian"), func() {
		a.lang = "ru"
		a.win.SetContent(a.buildLoginGate())
	})
	ruBtn.Importance = widget.HighImportance

	box := container.NewVBox(
		title,
		widget.NewSeparator(),
		choose,
		enBtn,
		csBtn,
		ruBtn,
	)

	card := widget.NewCard("", "", box)
	return container.NewCenter(container.NewPadded(card))
}

func (a *App) buildLoginGate() fyne.CanvasObject {
	title := widget.NewLabelWithStyle(a.tr("fast"), fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	a.gateLoginBtn = widget.NewButton(a.tr("loginMicrosoft"), func() {
		a.loginFromGate()
	})
	a.gateLoginBtn.Importance = widget.HighImportance

	loginElyBtn := widget.NewButton(a.tr("loginEly"), func() {
		dialog.ShowInformation("ely.by", a.tr("elyNotImplemented"), a.win)
	})

	loginLittleSkinBtn := widget.NewButton(a.tr("loginLittleSkin"), func() {
		dialog.ShowInformation("LittleSkin", a.tr("littleNotImplemented"), a.win)
	})

	orLabel := widget.NewLabelWithStyle(a.tr("or"), fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	usernameEntry := widget.NewEntry()
	usernameEntry.SetPlaceHolder(a.tr("usernamePlaceholder"))
	usernameEntry.SetText(a.cfg.OfflineUsername)

	continueBtn := widget.NewButton(a.tr("continue"), func() {
		name := strings.TrimSpace(usernameEntry.Text)
		if name == "" {
			dialog.ShowError(fmt.Errorf(a.tr("emptyUsername")), a.win)
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
		widget.NewSeparator(),
		a.gateLoginBtn,
		loginElyBtn,
		loginLittleSkinBtn,
		orLabel,
		usernameEntry,
		continueBtn,
	)

	card := widget.NewCard("", "", centerBox)
	return container.NewCenter(container.NewPadded(card))
}

func (a *App) loginFromGate() {
	if a.gateLoginBtn != nil {
		a.gateLoginBtn.Disable()
		a.gateLoginBtn.SetText(a.tr("loginRunning"))
	}

	go func() {
		account, err := auth.LoginMicrosoftLoopback()
		if err != nil {
			if a.gateLoginBtn != nil {
				a.gateLoginBtn.Enable()
				a.gateLoginBtn.SetText(a.tr("loginMicrosoft"))
			}

			dialog.ShowError(fmt.Errorf(a.tr("loginFailed"), err.Error()), a.win)
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
			if a.gateLoginBtn != nil {
				a.gateLoginBtn.Enable()
				a.gateLoginBtn.SetText(a.tr("loginMicrosoft"))
			}

			dialog.ShowError(fmt.Errorf(a.tr("saveAccountFailed"), err.Error()), a.win)
			return
		}

		a.win.SetContent(a.buildMainUI())
		go a.loadVersions()
	}()
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
	playBtn := widget.NewButton(a.tr("play"), func() {
		a.setPage(a.buildPlayPage())
	})
	playBtn.Importance = widget.HighImportance

	editBtn := widget.NewButton(a.tr("edit"), func() {
		a.setPage(a.buildEditPage())
	})

	logsBtn := widget.NewButton(a.tr("logs"), func() {
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
	newBtn := a.navButton(a.tr("new"), func() {
		dialog.ShowInformation("New Instance", a.tr("newInstance"), a.win)
	})
	newBtn.Importance = widget.HighImportance

	openGameDirBtn := a.navButton(a.tr("openGameDir"), func() {
		openFolder(a.cfg.GameDir)
	})

	openModsBtn := a.navButton(a.tr("openMods"), func() {
		modsDir := filepath.Join(a.cfg.GameDir, "mods")
		if err := os.MkdirAll(modsDir, 0755); err == nil {
			openFolder(modsDir)
		}
	})

	openLogsBtn := a.navButton(a.tr("openLogs"), func() {
		openFolder(filepath.Join(a.cfg.GameDir, "logs"))
	})

	accountTitle := widget.NewLabel(a.tr("accounts"))
	a.accountLabel = widget.NewLabel("")
	a.updateAccountLabel()

	logoutBtn := widget.NewButton(a.tr("logout"), func() {
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
	a.versionSelect = widget.NewSelect([]string{"..."}, func(v string) {
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

	snapshotCheck := widget.NewCheck(a.tr("snapshots"), func(v bool) {
		a.cfg.ShowSnapshots = v
		a.refreshVersionList()
		configs.Save(a.cfg)
	})
	snapshotCheck.SetChecked(a.cfg.ShowSnapshots)

	oldCheck := widget.NewCheck(a.tr("oldVersions"), func(v bool) {
		a.cfg.ShowOld = v
		a.refreshVersionList()
		configs.Save(a.cfg)
	})
	oldCheck.SetChecked(a.cfg.ShowOld)

	a.progressBar = widget.NewProgressBar()
	a.progressBar.Hide()

	a.statusLabel = widget.NewLabel(a.tr("ready"))
	a.statusLabel.Alignment = fyne.TextAlignCenter

	a.launchBtn = widget.NewButton(a.tr("launch"), a.onLaunch)
	a.launchBtn.Importance = widget.HighImportance

	versionCard := widget.NewCard(a.tr("version"), a.tr("versionDesc"), container.NewVBox(
		a.versionSelect,
	))

	memoryCard := widget.NewCard(a.tr("memory"), a.tr("memoryDesc"), container.NewVBox(
		a.ramLabel,
		a.ramSlider,
	))

	filterCard := widget.NewCard(a.tr("filters"), "", container.NewVBox(
		snapshotCheck,
		oldCheck,
	))

	playCard := widget.NewCard("", "", container.NewVBox(
		widget.NewLabelWithStyle(a.tr("readyToPlay"), fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle(a.tr("readyDesc"), fyne.TextAlignCenter, fyne.TextStyle{}),
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

	statusLabel := widget.NewLabel(a.tr("selectLoader"))
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
			statusLabel.SetText(a.tr("loaderDisabled"))
			return
		}

		mcVersion := extractVersionID(a.cfg.SelectedVersion)
		if mcVersion == "" {
			statusLabel.SetText(a.tr("selectVersionPlay"))
			return
		}

		statusLabel.SetText(a.tr("loadingLoaderVersions"))
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
				err = fmt.Errorf(a.tr("quiltLater"))
			}

			if err != nil {
				statusLabel.SetText("Error: " + err.Error())
				return
			}

			if len(loaders) == 0 {
				statusLabel.SetText(fmt.Sprintf(a.tr("noLoaders"), mcVersion))
				return
			}

			loaderVersionSelect.Options = loaders
			loaderVersionSelect.SetSelected(loaders[0])
			loaderVersionSelect.Enable()
			statusLabel.SetText(fmt.Sprintf(a.tr("foundLoaders"), len(loaders)))
		}()
	}

	var installBtn *widget.Button

	installBtn = widget.NewButton(a.tr("installLoader"), func() {
		if selectedType == modloader.None {
			statusLabel.SetText(a.tr("chooseLoader"))
			return
		}

		mcVersion := extractVersionID(a.cfg.SelectedVersion)
		loaderVer := loaderVersionSelect.Selected

		if mcVersion == "" || loaderVer == "" {
			statusLabel.SetText(a.tr("chooseVersionLoader"))
			return
		}

		installBtn.Disable()
		statusLabel.SetText(a.tr("installing"))

		go func() {
			defer installBtn.Enable()

			var err error

			switch selectedType {
			case modloader.Fabric:
				err = a.modInst.InstallFabric(mcVersion, loaderVer, func(msg string) {
					statusLabel.SetText(msg)
				})
			case modloader.Forge:
				statusLabel.SetText(a.tr("forgeLater"))
				return
			case modloader.Quilt:
				statusLabel.SetText(a.tr("quiltLater"))
				return
			}

			if err != nil {
				statusLabel.SetText("Error: " + err.Error())
				return
			}

			statusLabel.SetText(a.tr("installed"))
			go a.loadVersions()
		}()
	})
	installBtn.Importance = widget.HighImportance

	addFileBtn := widget.NewButton(a.tr("addFile"), func() {
		dialog.ShowInformation(a.tr("addFile"), a.tr("addFileLater"), a.win)
	})

	downloadContentBtn := widget.NewButton(a.tr("downloadContent"), func() {
		dialog.ShowInformation(a.tr("modStore"), a.tr("storeLater"), a.win)
	})

	openModsBtn := widget.NewButton(a.tr("openModsFolder"), func() {
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

	loaderCard := widget.NewCard(a.tr("modLoader"), a.tr("modLoaderDesc"), container.NewVBox(
		widget.NewLabel(a.tr("loaderType")),
		typeSelect,
		widget.NewLabel(a.tr("loaderVersion")),
		loaderVersionSelect,
		statusLabel,
		installBtn,
	))

	modStoreCard := widget.NewCard(a.tr("modStore"), a.tr("modStoreDesc"), container.NewVBox(
		widget.NewLabel(a.tr("modStoreText")),
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

	saveBtn := widget.NewButton(a.tr("saveSettings"), func() {
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

		dialog.ShowInformation(a.tr("saved"), a.tr("savedText"), a.win)
	})
	saveBtn.Importance = widget.HighImportance

	openDirBtn := widget.NewButton(a.tr("openGameDir"), func() {
		openFolder(a.cfg.GameDir)
	})

	openLogsBtn := widget.NewButton(a.tr("openLogs"), func() {
		openFolder(filepath.Join(a.cfg.GameDir, "logs"))
	})

	form := widget.NewForm(
		widget.NewFormItem(a.tr("gameDirectory"), gameDirEntry),
		widget.NewFormItem(a.tr("javaPath"), jvmEntry),
		widget.NewFormItem(a.tr("extraJvmArgs"), extraJVMEntry),
	)

	card := widget.NewCard(a.tr("settings"), a.tr("settingsDesc"), container.NewVBox(
		form,
		container.NewHBox(saveBtn, openDirBtn, openLogsBtn),
	))

	return container.NewVBox(card, layout.NewSpacer())
}

func (a *App) buildConsolePage() fyne.CanvasObject {
	a.logOutput = widget.NewTextGrid()
	a.logOutput.SetText(a.tr("consoleReady"))

	scroll := container.NewScroll(a.logOutput)
	scroll.SetMinSize(fyne.NewSize(760, 460))

	clearBtn := widget.NewButton(a.tr("clear"), func() {
		a.logOutput.SetText("")
	})

	openLogsBtn := widget.NewButton(a.tr("openLogs"), func() {
		openFolder(filepath.Join(a.cfg.GameDir, "logs"))
	})

	card := widget.NewCard(a.tr("logs"), "", container.NewBorder(nil, container.NewHBox(clearBtn, openLogsBtn), nil, nil, scroll))

	return container.NewVBox(card, layout.NewSpacer())
}

func (a *App) loadVersions() {
	a.setStatus(a.tr("loadingVersions"))

	manifest, err := a.versionsMgr.FetchManifest()
	if err != nil {
		a.setStatus("Error: " + err.Error())
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

	a.setStatus(fmt.Sprintf(a.tr("loadedVersions"), len(names)))
}

func (a *App) onMSLogin() {
	a.setStatus(a.tr("loginRunning"))

	if a.loginBtn != nil {
		a.loginBtn.Disable()
		a.loginBtn.SetText(a.tr("loginRunning"))
	}

	go func() {
		account, err := auth.LoginMicrosoftLoopback()
		if err != nil {
			if a.loginBtn != nil {
				a.loginBtn.Enable()
				a.loginBtn.SetText(a.tr("loginMicrosoft"))
			}

			a.setStatus(fmt.Sprintf(a.tr("loginFailed"), err.Error()))
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
			if a.loginBtn != nil {
				a.loginBtn.Enable()
				a.loginBtn.SetText(a.tr("loginMicrosoft"))
			}

			a.setStatus(fmt.Sprintf(a.tr("saveAccountFailed"), err.Error()))
			return
		}

		if a.loginBtn != nil {
			a.loginBtn.Enable()
			a.loginBtn.SetText(a.tr("loginMicrosoft"))
		}

		a.updateAccountLabel()
		a.setStatus(fmt.Sprintf(a.tr("loggedIn"), account.Username))
	}()
}

func (a *App) onLaunch() {
	var playerName string
	var uuid string
	var token string

	if a.cfg.OfflineMode || a.cfg.Account == nil {
		playerName = strings.TrimSpace(a.cfg.OfflineUsername)
		if playerName == "" {
			dialog.ShowError(fmt.Errorf(a.tr("emptyUsername")), a.win)
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
		dialog.ShowError(fmt.Errorf(a.tr("noVersion")), a.win)
		return
	}

	versionID := extractVersionID(selected)

	if a.manifest == nil {
		a.setStatus(a.tr("versionNotLoaded"))
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
		dialog.ShowError(fmt.Errorf(a.tr("versionNotFound"), versionID), a.win)
		return
	}

	a.launchBtn.Disable()
	a.progressBar.Show()
	a.progressBar.SetValue(0)
	a.setStatus(a.tr("preparing"))

	go func() {
		defer func() {
			a.launchBtn.Enable()
			a.progressBar.Hide()
		}()

		a.setStatus(a.tr("loadingMeta"))

		meta, err := a.versionsMgr.FetchVersionMeta(entry)
		if err != nil {
			a.setStatus("Error: " + err.Error())
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
				a.setStatus(fmt.Sprintf(a.tr("downloading"), p.Completed, p.Total))
			}
		})

		if err != nil {
			a.setStatus(fmt.Sprintf(a.tr("launchFailed"), err.Error()))
			return
		}

		logPath := filepath.Join(a.cfg.GameDir, "logs", "golauncher-latest.log")

		a.setStatus(a.tr("running"))
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
			a.setStatus(a.tr("closed"))
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
		a.accountLabel.SetText(fmt.Sprintf(a.tr("online"), a.cfg.Account.Username))
		return
	}

	if strings.TrimSpace(a.cfg.OfflineUsername) != "" {
		a.accountLabel.SetText(fmt.Sprintf(a.tr("offline"), a.cfg.OfflineUsername))
		return
	}

	a.accountLabel.SetText(a.tr("offlineShort"))
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
