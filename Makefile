.PHONY: build dev test clean release

# Generate Windows resource file (syso)
StarCore-res.syso: build/windows/icon.ico
	@which windres > /dev/null 2>&1 || (echo "windres not found, skipping syso generation" && touch StarCore-res.syso)
	@windres -i build/StarCore.rc -o StarCore-res.syso 2>/dev/null || touch StarCore-res.syso

# Development mode (hot reload)
dev:
	wails dev

# Build for current platform
build: StarCore-res.syso
	wails build

# Build for Windows
build-windows:
	GOOS=windows GOARCH=amd64 wails build -platform windows -arch amd64

# Run tests
test:
	go test ./internal/... -v

# Clean build artifacts
clean:
	rm -f StarCore-res.syso
	rm -rf build/bin

# Create NSIS installer (requires makensis + nsProcess plugin)
# Run after build-windows
installer: build-windows
	cp build/bin/StarCore.exe build/StarCore.exe
	@echo "Creating NSIS installer..."
	cd build && makensis installer.nsi
	@echo "Done: build/StarCore-Setup-1.0.0.exe"

# Release: build + zip
release: build-windows
	@echo "Creating release package..."
	cd build/bin && zip -r ../../StarCore-windows-amd64.zip StarCore.exe
	@echo "Release: build/bin/StarCore-windows-amd64.zip"
