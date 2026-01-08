GOFILES=$(shell find . -iname '*.go')
# $(info [$(GOFILES)])

LIBWHISPER:=$(shell brew --prefix libwhisper)

ifndef LIBWHISPER
$(error LIBWHISPER not set - you may need to `brew install llimllib/whisper/libwhisper`)
endif

# mac's clang doesn't allow -fopenmp, so use the homebrew clang instead. How
# ought I to do this more generally? This will only work for me, right now
# XXX
CC := /opt/homebrew/Cellar/llvm/21.1.8/bin/clang
CPPFLAGS := "-I/opt/homebrew/opt/libunwind-headers/include"
LDFLAGS := -L/opt/homebrew/opt/llvm/lib/unwind

# C_LIBRARY_PATH and LIBRARY_PATH must be set to point to directories
# containing whisper and ggml headers and libwhisper.a file, respectively
bin/yt-transcribe: $(GOFILES)
		CC=$(CC) CGO_LDFLAGS=$(LDFLAGS) go build -o bin/yt-transcribe .

.PHONY: install
install:
		go install

.PHONY: watch
watch:
	modd

.PHONY: lint
lint:
	staticcheck ./...
	go vet ./...



