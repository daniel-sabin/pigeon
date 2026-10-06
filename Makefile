APP_NAME    := Pigeon
BUILD_APP   := build/bin/$(APP_NAME).app
INSTALL_DIR ?= $(HOME)/Applications

.PHONY: build install

build:
	wails build

# Build, then replace the installed app and relaunch it.
install: build
	-osascript -e 'quit app "$(APP_NAME)"' 2>/dev/null
	rm -rf "$(INSTALL_DIR)/$(APP_NAME).app"
	ditto "$(BUILD_APP)" "$(INSTALL_DIR)/$(APP_NAME).app"
	open "$(INSTALL_DIR)/$(APP_NAME).app"
