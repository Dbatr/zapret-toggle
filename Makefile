# zapret-toggle: build — собрать приложение; install/uninstall — через sudo (системная часть + приложение).

BIN     := build/zapret-toggle
GO      ?= go
GOFLAGS ?= -trimpath
LDFLAGS ?= -s -w

# GLFW в Fyne 2.8 по умолчанию собирается с X11 и Wayland. Если заголовков Wayland нет
# (sudo dnf install wayland-devel libxkbcommon-devel), собираем только X11 — окно пойдёт через XWayland.
TAGS ?= $(shell pkg-config --exists wayland-client wayland-cursor wayland-egl xkbcommon 2>/dev/null || echo x11)

SHELLCHECK ?= shellcheck
SCRIPTS := system/firewall.sh system/run.sh scripts/install.sh scripts/uninstall.sh \
	scripts/update-flowseal.sh scripts/check-strategies.sh

.PHONY: build run test vet fmt lint check install uninstall update-flowseal clean

build:
	$(GO) build $(GOFLAGS) -tags '$(TAGS)' -ldflags '$(LDFLAGS)' -o $(BIN) ./cmd/zapret-toggle

run: build
	./$(BIN)

test:
	$(GO) test -tags '$(TAGS)' ./...

vet:
	$(GO) vet -tags '$(TAGS)' ./...

fmt:
	gofmt -w cmd internal assets

lint:
	@test -z "$$(gofmt -l cmd internal assets)" || { gofmt -l cmd internal assets; echo "gofmt: файлы выше не отформатированы"; exit 1; }
	$(SHELLCHECK) -x $(SCRIPTS)

check: lint vet test build

install:
	sudo ./scripts/install.sh

uninstall:
	sudo ./scripts/uninstall.sh

update-flowseal:
	./scripts/update-flowseal.sh

clean:
	rm -rf build
