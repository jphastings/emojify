# emojify

Maps a short passage of text (≤300 characters) to the emoji that best fit its
meaning, via nearest-neighbour search over sentence embeddings.

## Build

Default build (ONNX Runtime, cgo, needs `libonnxruntime` installed — e.g.
`brew install onnxruntime` on macOS):

    ./scripts/fetch-onnx-model.sh   # once, downloads data/model.onnx + data/vocab.txt
    go build -tags onnx ./cmd/emojify

Static, cgo-free build (pure-Go GloVe-averaging embedder — lower quality, no
native dependency, the only option on an ARMv6 original Pi Zero):

    CGO_ENABLED=0 go build -tags '!onnx' ./cmd/emojify

## Run

    echo "what a beautiful day" | ./emojify
    ./emojify text "the deploy failed again" --limit 3
    ./emojify text --json "quiet morning, strong coffee"
    ./emojify server --addr :8080

Exit codes: `0` output produced, `1` error, `2` nothing cleared `--min-score`.

## Rebuild the index

    go run -tags onnx ./cmd/emojify index build          # data/index.bin
    go run ./cmd/emojify index build --out data/index_static.bin  # data/index_static.bin
    go run ./cmd/emojify index inspect 🌞                  # blob + nearest neighbours

## Docker

    ./scripts/fetch-onnx-model.sh
    docker build -t emojify .
    docker run -p 8080:8080 emojify

## Cross-compiling for arm64 (Raspberry Pi, etc.)

cgo + a different architecture means either building on the target, under
QEMU, or with a cross toolchain. Docker buildx is the least painful:

    docker buildx build --platform linux/arm64 -t emojify:arm64 .

For boards where installing ONNX Runtime isn't worth it, cross-compile the
static build instead — this produces a single file with no runtime
dependency at all:

    GOOS=linux GOARCH=arm CGO_ENABLED=0 go build -tags '!onnx' -o emojify-arm ./cmd/emojify

## Benchmarking on real target hardware

This repo's `go test -bench BenchmarkSuggest` (see `rank_bench_test.go`) only
ran on the dev machine during implementation — no physical target SBC (e.g. a
Pi Zero 2 W) was reachable from that environment. To benchmark on real
hardware: cross-compile as above, copy the binary (plus `libonnxruntime.so`
and `data/model.onnx`/`data/vocab.txt` for the onnx build) to the board, and
run the same command there:

    go test -tags onnx ./... -bench BenchmarkSuggest -benchtime 10x -run '^$'

## Server

`GET /xrpc/me.byjp.emojify.suggestEmojis?text=...&limit=3` — see
`lexicons/me.byjp.emojify.suggestEmojis.json` for the full schema (served at
`GET /lexicons/me.byjp.emojify.suggestEmojis.json`). Also: `GET /healthz`,
`GET /xrpc/_health`.

The endpoint is open/anonymous by design (no auth) but rate-limited per-IP
and bounded in concurrency — see `server/limits.go`. Requests are logged by
result and latency only; input text is never logged.
