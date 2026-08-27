# Dockerfile
# Builds the default `onnx` binary. This is not a static binary (cgo links
# against libonnxruntime.so) — see README.md for the CGO_ENABLED=0 !onnx
# static-binary alternative for boards where installing ORT isn't practical.
FROM golang:1.26-bookworm AS build
WORKDIR /src
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates
# ONNX Runtime C library + headers, matched to the arch being built for.
# 1.29.0 is the floor: yalue/onnxruntime_go's pinned version (see go.mod)
# requires ORT C-API v29, first available in ONNX Runtime 1.29.
ARG ONNXRUNTIME_VERSION=1.29.0
# TARGETARCH is set automatically by BuildKit (matches --platform, or the
# host arch when unset) — it must be mapped to ORT's own release naming
# (linux-x64 / linux-aarch64), which doesn't match Go/Docker's arch names.
# Getting this wrong doesn't fail the build (the Go side dlopen()s ORT at
# runtime rather than linking it in) — it fails silently until the
# container actually starts, with a same-looking "cannot open shared
# object file" error either way, so don't skip this on an arm64 build host.
ARG TARGETARCH
RUN set -eux; \
    case "$TARGETARCH" in \
        amd64) ort_arch=x64 ;; \
        arm64) ort_arch=aarch64 ;; \
        *) echo "no known ONNX Runtime release for TARGETARCH=$TARGETARCH" >&2; exit 1 ;; \
    esac; \
    curl -fL "https://github.com/microsoft/onnxruntime/releases/download/v${ONNXRUNTIME_VERSION}/onnxruntime-linux-${ort_arch}-${ONNXRUNTIME_VERSION}.tgz" \
        -o /tmp/ort.tgz \
    && tar -xzf /tmp/ort.tgz -C /usr/local --strip-components=1 \
    && ldconfig
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Defaults to "dev" for a plain `docker build`, matching main.go's own
# fallback — release.yml passes the real tag via --build-arg.
ARG VERSION=dev
RUN CGO_ENABLED=1 go build -tags onnx -ldflags "-s -w -X main.version=${VERSION}" -o /out/emojify ./cmd/emojify

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl && rm -rf /var/lib/apt/lists/*
COPY --from=build /usr/local/lib/libonnxruntime.so* /usr/local/lib/
RUN ldconfig
COPY --from=build /out/emojify /usr/local/bin/emojify
# Fetched here rather than COPY'd from the host: data/model.onnx and
# data/vocab.txt are gitignored (scripts/fetch-onnx-model.sh, run locally,
# populates them for a dev build) — a platform building straight from this
# git repo (Railway, a fresh clone, CI) has neither file, so COPY would fail
# with "not found". Same pinned source as that script; keep both in sync.
RUN mkdir -p /app/data \
    && curl -fL "https://huggingface.co/Xenova/all-MiniLM-L6-v2/resolve/main/onnx/model_quantized.onnx" \
        -o /app/data/model.onnx \
    && curl -fL "https://huggingface.co/Xenova/all-MiniLM-L6-v2/resolve/main/vocab.txt" \
        -o /app/data/vocab.txt
ENV EMOJIFY_MODEL_PATH=/app/data/model.onnx
# embedder_onnx.go's default lookup path is the Linux-packaged /usr/lib
# location; ldconfig alone doesn't help since the app dlopen()s an absolute
# path rather than a bare library name, so point it at where ORT actually
# landed above.
ENV EMOJIFY_ORT_LIBRARY_PATH=/usr/local/lib/libonnxruntime.so
EXPOSE 8080
ENTRYPOINT ["emojify", "server", "--addr", ":8080"]
