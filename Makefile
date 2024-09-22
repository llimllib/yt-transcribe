GOFILES=$(shell find . -iname '*.go')
# $(info [$(GOFILES)])

LIBWHISPER:=$(shell brew --prefix libwhisper)

ifndef LIBWHISPER
$(error LIBWHISPER not set - you may need to `brew install llimllib/whisper/libwhisper`)
endif

# mac's clang doesn't allow -fopenmp, so use the homebrew clang instead. How
# ought I to do this more generally? This will only work for me, right now
# XXX
CC := /opt/homebrew/Cellar/llvm/18.1.8/bin/clang

bin/yt-transcribe: $(GOFILES)
	C_INCLUDE_PATH=$(LIBWHISPER)/include \
	LIBRARY_PATH=$(LIBWHISPER)/lib \
		go build -o bin/yt-transcribe .

.PHONY: install
install:
	C_INCLUDE_PATH=$(LIBWHISPER)/include \
	LIBRARY_PATH=$(LIBWHISPER)/lib \
		go install

.PHONY: watch
watch:
	modd

.PHONY: lint
lint:
	staticcheck ./...
	go vet ./...
