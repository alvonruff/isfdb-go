SDK := $(shell xcrun --show-sdk-path)
CGO_FLAGS := CGO_CXXFLAGS="-isysroot $(SDK) -I$(SDK)/usr/include/c++/v1"

.PHONY: all server app clean

all: server app

server:
	go build ./cmd/server/

app:
	$(CGO_FLAGS) go build ./cmd/app/

clean:
	rm -f server app
