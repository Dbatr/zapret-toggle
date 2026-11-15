# zapret-toggle: build — собрать приложение; install/uninstall — через sudo (системная часть + приложение).

BIN     := build/zapret-toggle
GO      ?= go
GOFLAGS ?= -trimpath
LDFLAGS ?= -s -w

# Если Go старее, чем требует go.mod, он сам скачает нужную версию (умеет с Go 1.21). В некоторых
# дистрибутивах (Fedora) по умолчанию стоит GOTOOLCHAIN=local, и сборка просто падает — поэтому auto.
# Своё значение из окружения (например, GOTOOLCHAIN=local) имеет приоритет.
export GOTOOLCHAIN ?= auto

# Бэкенды окна. GLFW в Fyne 2.8 без тегов собирает и X11, и Wayland — нужны заголовки обоих.
# scripts/build-deps.sh смотрит (через pkg-config), что установлено:
#   X11 и Wayland — без тегов: в Wayland-сессии окно нативное, в X11 — обычное;
#   только X11    — x11: в Wayland-сессии окно пойдёт через XWayland;
#   только Wayland — wayland: чистый Wayland, бинарник не зависит от X11-библиотек;
#   ничего        — `make build` остановится с подсказкой, какие пакеты поставить.
# Явно: make build TAGS=x11 | TAGS=wayland | TAGS= (оба бэкенда).
ifeq ($(origin TAGS),undefined)
TAGS := $(shell scripts/build-deps.sh tags 2>/dev/null || echo none)
endif

# GLFW подключает заголовки X11 и Wayland без pkg-config, а в openSUSE заголовки Wayland лежат
# в /usr/include/wayland. Такие пути добавляем в CGO_CFLAGS сами; где всё в /usr/include (Fedora,
# Debian, Arch), список пуст и CGO_CFLAGS не трогается. «-O2 -g» — значение Go по умолчанию.
BACKEND_CFLAGS := $(strip $(shell scripts/build-deps.sh cflags '$(TAGS)' 2>/dev/null))
ifneq ($(BACKEND_CFLAGS),)
export CGO_CFLAGS := $(or $(CGO_CFLAGS),-O2 -g) $(BACKEND_CFLAGS)
endif

SHELLCHECK ?= shellcheck
SCRIPTS := system/firewall.sh system/run.sh scripts/install.sh scripts/uninstall.sh \
	scripts/update-flowseal.sh scripts/check-strategies.sh scripts/platform.sh \
	scripts/build-deps.sh scripts/test-scripts.sh

.PHONY: build run deps test vet fmt lint check install uninstall update-flowseal clean

# Go, компилятор C, pkg-config, OpenGL и заголовки под выбранные теги; нет — список и команда установки.
deps:
	@GO='$(GO)' scripts/build-deps.sh check '$(TAGS)' >/dev/null

build: deps
	$(GO) build $(GOFLAGS) -tags '$(TAGS)' -ldflags '$(LDFLAGS)' -o $(BIN) ./cmd/zapret-toggle

run: build
	./$(BIN)

test: deps
	scripts/test-scripts.sh
	$(GO) test -tags '$(TAGS)' ./...

vet: deps
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
