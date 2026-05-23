# GoLauncher — Ultra-fast Minecraft Launcher in Go

Launcher napsaný v Go s Fyne GUI. Rychlý, spolehlivý, cross-platform.

## Features

- ✅ **Všechny vanilla verze** — dynamicky z Mojang manifestu
- ✅ **Fabric support** — automatický install přes Fabric Meta API
- ✅ **Forge support** — detekce verzí (install přes oficiální JAR)
- ✅ **Microsoft OAuth2** — přihlášení přes Microsoft účet
- ✅ **Paralelní download** — 32 simultánních downloadů (goroutines)
- ✅ **SHA1 verifikace** — kontrola integrity každého souboru
- ✅ **JVM auto-detect** — najde Javu automaticky
- ✅ **FPS JVM args** — G1GC tuning pro minimální GC pauses
- ✅ **Cross-platform** — Windows / Linux / macOS

## Požadavky

- Go 1.21+
- GCC (pro Fyne CGO)
  - Windows: [TDM-GCC](https://jmeubank.github.io/tdm-gcc/)
  - Linux: `sudo apt install gcc`
  - macOS: `xcode-select --install`
- Java (8, 17, nebo 21 podle verze MC)

## Instalace

```bash
# Klonovat / rozbalit projekt
cd mclauncher

# Stáhnout závislosti
go mod tidy

# Spustit
go run ./cmd/

# Nebo sestavit
chmod +x build.sh
./build.sh          # current platform
./build.sh windows  # cross-compile na Windows
./build.sh linux
./build.sh darwin
```

## Přihlášení

1. Klikni **Login with Microsoft**
2. Otevře se prohlížeč → přihlas se Microsoft účtem
3. Po přihlášení zkopíruj hodnotu `code=` z URL adresy
4. Vlož do dialogu → potvrď

## Struktura projektu

```
mclauncher/
├── cmd/main.go              # Entry point
├── configs/config.go        # Uložení nastavení (~/.golauncher/config.json)
├── internal/
│   ├── auth/auth.go         # Microsoft OAuth2 → Xbox → Minecraft auth
│   ├── assets/downloader.go # Paralelní stahování assets + knihoven
│   ├── java/java.go         # Detekce JVM instalací
│   ├── launcher/launcher.go # Sestavení JVM args + spuštění procesu
│   ├── modloader/           # Fabric / Forge / Quilt
│   ├── versions/            # Mojang version manifest + parsování
│   └── gui/gui.go           # Fyne GUI (okno, taby, dialogy)
└── build.sh                 # Build script
```

## JVM Optimalizace (zabudované)

```
-XX:+UseG1GC
-XX:G1NewSizePercent=20
-XX:G1ReservePercent=20
-XX:MaxGCPauseMillis=50        ← max 50ms GC pause = smooth FPS
-XX:G1HeapRegionSize=32M
-XX:+DisableExplicitGC
-XX:+AlwaysPreTouch
```

Tyto args jsou inspirované Lithium/Aikar's flags — standardní základ pro smooth MC.

## Fabric Install

1. Vyber MC verzi v Launch tabu
2. Přejdi na **Mod Loaders** tab
3. Vyber **Fabric** → vyber loader verzi → **Install**
4. Vrať se do Launch → vyber fabric verzi → spusť

## Forge Install

Forge vyžaduje spuštění vlastního JAR installeru:
1. Stáhni installer z https://files.minecraftforge.net
2. `java -jar forge-installer.jar --installClient`
3. GoLauncher pak detekuje nainstalované Forge verze automaticky
