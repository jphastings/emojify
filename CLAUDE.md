# emojify — notes for working on this repo

Maps a short passage of text to the emoji that best fits its *meaning*, by
embedding the text and doing nearest-neighbour search against a pre-built
index of emoji descriptions. See README.md for build/run/release usage; this
file is the sharp edges that aren't obvious from reading the code.

## The single most surprising thing

**A bare `go build` produces the *lower-quality* binary.**

There are two embedders behind build tags, and they are not close in quality:

| build | embedder | index | quality |
|---|---|---|---|
| `go build` (no tags) | pure-Go averaged GloVe vectors | `data/index_static.bin` (50-dim) | much worse |
| `go build -tags onnx` | real ONNX Runtime, all-MiniLM-L6-v2 | `data/index.bin` (384-dim) | production |

The static build exists for portability and hermetic tests (no cgo, runs on an
ARMv6 Pi Zero, needs no model files), *not* because it's good. The golden-table
floors in `rank_test_static.go` and `rank_test_onnx.go` record the real gap —
they're deliberately far apart.

Consequences that bite:
- `go install …@latest` and the Homebrew cask both ship the **static** build
  (goreleaser builds `CGO_ENABLED=0`, no tags). Anything user-facing that shows
  example output must show what *that* build really returns, not the ONNX one.
- Always run tests under **both** tags. `go test ./...` alone proves little.
- Both embedders must export the same symbols (`NewDefaultEmbedder`,
  `defaultIndexData`, `DefaultIndexPath`) so the rest of the tree compiles
  unchanged either way.

## Getting set up

```bash
./scripts/fetch-onnx-model.sh   # data/model.onnx + data/vocab.txt — gitignored
brew install onnxruntime        # macOS; needs >= 1.29, see below
go test ./... && go test -tags onnx ./...
```

`data/*.bin` (the built indexes and trimmed vectors) **are** committed — they're
build artefacts, regenerated deliberately, not on every build. `model.onnx` and
`vocab.txt` are **not** committed; fetch them.

## Sharp edges

1. **ONNX Runtime needs ≥ 1.29.** The Go binding (`yalue/onnxruntime_go`)
   requires ORT C-API v29. Older versions fail at *runtime*, not build time.
   The floor is pinned in two places that must stay in sync: the Dockerfile's
   `ONNXRUNTIME_VERSION` arg and CI's `ONNXRUNTIME_VERSION` env.

2. **The ORT library path is an absolute `dlopen`, not a linker lookup.**
   `ldconfig` does not help. Defaults are the Homebrew Apple-Silicon prefix on
   macOS and `/usr/lib/libonnxruntime.so` on Linux; override with
   `EMOJIFY_ORT_LIBRARY_PATH`. Both the Dockerfile and CI set it explicitly
   because they install ORT somewhere else. An Intel Mac needs it set too.

3. **`go test` runs with the package dir as its working directory.** The onnx
   embedder resolves `data/model.onnx` relative to the CWD, so any package
   whose tests construct the default embedder needs an `onnx`-tagged setup file
   pointing `EMOJIFY_MODEL_PATH` at the repo root — see
   `cmd/emojify/onnx_test_setup_test.go` and `server/onnx_test_setup_test.go`.
   **New package with onnx-tagged tests? Copy that file and fix the depth.**

4. **Don't cross-build the Docker image under QEMU.** It was tried; emulated
   `go mod download` either crashed or silently returned a module zip that
   failed `go.sum` verification (checked against sum.golang.org — an emulation
   artefact, not a real supply-chain problem). CI and release therefore build
   each arch on a **native** runner and stitch a manifest. Don't "simplify"
   that back into one buildx multi-platform job.

5. **goreleaser**: `brews:` is removed — use `homebrew_casks:`. And use
   `index .Env "X"` in templates, never `.Env.X`: the latter is a hard error
   when the variable is *absent* (as opposed to set-but-empty), which broke CI
   once. Test with the variable genuinely unset (`env -u`), not set to `""`.

6. **Score is cosine × penalty, and the wire format is per-mille.** The
   `Suggestion.Score` float is a similarity adjusted by a generic-emoji
   penalty, not raw cosine. The XRPC integer is 0–1000 (per-mille) — *not*
   basis points, despite how it reads. Both are documented in the lexicon;
   keep them honest if you touch the ranking.

7. **The landing page must stay self-contained.** `server/index.html` is
   embedded in the binary, which people self-host. No webfonts, no CDN scripts.
   A test in `server/server_test.go` enforces this.

8. **Root route is `GET /{$}`, not `GET /`.** The latter is a catch-all and
   would swallow 404s for mistyped API paths.

## Input length: measured, not guessed

Current cap is 600 runes (`emojify.go`). If you want to change it, the
constraints, in order:

- **Latency isn't one.** The ONNX tensors are fixed at 256 tokens and every
  request runs the full width, so embedding cost is flat regardless of input
  length. Longer input is free.
- **Truncation at 254 content tokens.** Beyond that, text is silently ignored.
  That's roughly 1300 runes of ordinary English, ~660 accented/European, and
  ~280 for punctuation-dense worst cases.
- **Quality is the real limit, and it's dilution, not length.** Coherent
  on-topic text still resolves correctly well past 450 runes. But a clear
  signal padded with neutral prose washes out between roughly 200 and 400
  runes — mean-pooling drags the embedding toward the corpus average and
  generic emoji (calendar, handshake) win.

Re-measure rather than trusting these numbers if the model or tokenizer
changes. Raising the cap also means raising the lexicon's `maxLength` (bytes,
~4× the rune count for emoji-heavy input).

## Known limitations

- **English only.** The model is English-only; CJK largely tokenizes to
  `[UNK]`. Emoji in the *input* are also effectively ignored (deliberately not
  split as punctuation, so they become a single `[UNK]`).
- **Negation is weak** — a known sentence-transformer failure. There's an
  intentionally unasserted `informational` row in `testdata/golden.yaml`
  tracking it.
- **`maxGraphemes` in the lexicon counts runes**, not grapheme clusters. The
  field name is the AT-Proto convention; the description says what's really
  enforced. Real grapheme counting would need a new dependency.

## Open / not done

- **Homebrew publishing is configured but inert** until the tap's GitHub App is
  created — see `docs/homebrew-tap-setup.md`. Until then the release passes an
  empty token and `skip_upload` makes the cask a no-op, so releases still
  succeed. Once it's live, add `brew install` to README and the landing page
  (deliberately omitted while it would 404).
- **No benchmarks on real target hardware.** `BenchmarkSuggest` has only ever
  run on a dev machine; the Pi Zero 2 W figures in the design docs are
  extrapolation. README has the procedure.

## Conventions

- Tests are behavioural — assert on observable output, not internals.
- Comments explain *why*, not *what*; the codebase leans on clear naming.
- Don't put changing facts (counts, dates, "N passing") in docs — state things
  qualitatively or point at the command that produces the truth.
