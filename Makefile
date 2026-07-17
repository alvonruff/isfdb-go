UNAME := $(shell uname)

ifeq ($(UNAME), Darwin)
  SDK := $(shell xcrun --show-sdk-path)
  CGO_FLAGS := CGO_CXXFLAGS="-isysroot $(SDK) -I$(SDK)/usr/include/c++/v1"
else
  # Modern distros ship webkit2gtk-4.1 rather than 4.0.
  # Generate a shim .pc file so the webview library's pkg-config lookup succeeds.
  PKGSHIM := /tmp/isfdb-pkgconfig
  $(shell mkdir -p $(PKGSHIM) && printf \
    'Name: webkit2gtk-4.0\nDescription: shim\nVersion: 2.0\nRequires: webkit2gtk-4.1\n' \
    > $(PKGSHIM)/webkit2gtk-4.0.pc)
  CGO_FLAGS := PKG_CONFIG_PATH=$(PKGSHIM)
endif

.PHONY: all server app clean

all: server app

server:
	go build ./cmd/server/

app:
	$(CGO_FLAGS) go build ./cmd/app/

clean:
	rm -f server app
