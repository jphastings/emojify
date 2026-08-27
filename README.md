# emojify

Maps a short passage of text (≤600 characters) to the emoji that best fit its
meaning, via nearest-neighbour search over sentence embeddings.

## Build

Default build (ONNX Runtime, cgo, needs `libonnxruntime` installed — e.g.
`brew install onnxruntime` on macOS):

    ./scripts/fetch-onnx-model.sh   # once, downloads data/model.onnx + data/vocab.txt
    go build -tags onnx ./cmd/emojify

Static, cgo-free build (pure-Go GloVe-averaging embedder — lower quality, no
native dependency, the only option on an ARMv6 original Pi Zero):

    CGO_ENABLED=0 go build ./cmd/emojify

## Install

    brew install jphastings/tools/emojify                      # macOS
    go install github.com/jphastings/emojify/cmd/emojify@latest

Both ship the pure-Go embedder: small, dependency-free, and less sharp on
subtle phrasing than the ONNX model. For that one, run the container (see
Docker below) or build with `-tags onnx`.

The cask is published to [jphastings/homebrew-tools](https://github.com/jphastings/homebrew-tools)
on each release — see [docs/homebrew-tap-setup.md](docs/homebrew-tap-setup.md)
for how that's wired up.

## Run

    echo "what a beautiful day" | ./emojify
    ./emojify text "the deploy failed again" --limit 3
    ./emojify text --json "quiet morning, strong coffee"
    ./emojify server --addr :8080

Exit codes: `0` output produced, `1` error, `2` nothing cleared `--min-score`.

## Rebuild the index

    go run -tags onnx ./cmd/emojify index build   # -> data/index.bin
    go run ./cmd/emojify index build              # -> data/index_static.bin
    go run ./cmd/emojify index inspect 🌞          # blob + nearest neighbours

`--out` defaults to whichever index the active build tag actually embeds, so
each build regenerates its own; `--include-flags` adds country/region flags.
Both indexes are committed artefacts — rebuild deliberately, then commit.

## Docker

    docker build -t emojify .
    docker run -p 8080:8080 emojify

(The Dockerfile fetches the model/vocab itself during the build — unlike the
local Go build above, no separate `fetch-onnx-model.sh` step is needed.)

Pre-built multi-arch images (`linux/amd64`, `linux/arm64`) are published to
`ghcr.io/jphastings/emojify` on every tagged release — see "Releasing"
below. Measured resident memory under load: ~90MB; 256MB is a comfortable
minimum for the container, 512MB for headroom.

## Releasing

Pushing a tag matching `v*` (e.g. `v0.1.0`) triggers `.github/workflows/release.yml`:
[goreleaser](https://goreleaser.com) cross-compiles the static (`!onnx`)
binary for linux/darwin/windows × amd64/arm64, plus linux/arm (`GOARM` 6 and
7, for the Pi Zero family), and publishes them as a GitHub Release with a
changelog and checksums. Separately, the `onnx` (production) image is built
natively per architecture (no QEMU — see the workflow's comments for why)
and published to `ghcr.io/jphastings/emojify` tagged with the version,
the `major.minor`, and `latest`.

    git tag v0.1.0
    git push origin v0.1.0

To dry-run the binary build locally without publishing anything:

    goreleaser release --snapshot --clean

## Cross-compiling for arm64 (Raspberry Pi, etc.)

cgo + a different architecture means either building on the target, under
QEMU, or with a cross toolchain. Docker buildx is the least painful:

    docker buildx build --platform linux/arm64 -t emojify:arm64 .

For boards where installing ONNX Runtime isn't worth it, cross-compile the
static build instead — this produces a single file with no runtime
dependency at all:

    GOOS=linux GOARCH=arm CGO_ENABLED=0 go build -o emojify-arm ./cmd/emojify

## Benchmarking on real target hardware

This repo's `go test -bench BenchmarkSuggest` (see `rank_bench_test.go`) only
ran on the dev machine during implementation — no physical target SBC (e.g. a
Pi Zero 2 W) was reachable from that environment. To benchmark on real
hardware: cross-compile as above, copy the binary (plus `libonnxruntime.so`
and `data/model.onnx`/`data/vocab.txt` for the onnx build) to the board, and
run the same command there:

    go test -tags onnx ./... -bench BenchmarkSuggest -benchtime 10x -run '^$'

## Server

`GET /xrpc/me.byjp.emojify.suggestEmoji?text=...&limit=3` — see
`lexicons/me/byjp/emojify/suggestEmoji.json` for the full schema (served at
`GET /lexicons/me.byjp.emojify.suggestEmoji.json`). Also: `GET /healthz`,
`GET /xrpc/_health`, and a small demo page at `GET /`.

`score` is an integer 0–1000 (per-mille, *not* basis points), and is a
similarity adjusted by a generic-emoji penalty rather than a raw cosine.

The endpoint is open/anonymous by design (no auth) but rate-limited per-IP
and bounded in concurrency — see `server/limits.go`. Requests are logged by
result and latency only; input text is never logged.

## Limits and caveats

- **Input is capped at 600 characters** (counted in Unicode code points, not
  grapheme clusters — a ZWJ emoji counts as several). Over that is rejected
  with `TextTooLong`.
- **The model reads at most 256 tokens** — roughly 1300 characters of ordinary
  English, far fewer for punctuation-dense text. Beyond that the tail is
  *ignored*, not rejected.
- **Long, unfocused text gives generic answers.** Coherent on-topic text works
  well past 450 characters, but a single interesting sentence buried in neutral
  prose washes out: mean-pooling pulls the embedding toward the average and
  generic emoji win. Short and focused beats long and vague.
- **English only.** CJK input largely tokenizes to `[UNK]`, and emoji in the
  input are effectively ignored.
- **Negation is weak** — "not a good day" embeds close to "a good day". A known
  sentence-transformer limitation, tracked by an unasserted row in
  `testdata/golden.yaml`.

## Contributing

See [CLAUDE.md](CLAUDE.md) for the non-obvious parts — build tags, the ONNX
Runtime version floor and library-path handling, test setup per package, and
why the release pipeline avoids QEMU.

## License

[MIT](LICENSE)
