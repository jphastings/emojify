#!/usr/bin/env bash
# scripts/fetch-onnx-model.sh — downloads the pinned quantized all-MiniLM-L6-v2
# ONNX model + tokenizer vocab into data/. Not go:embed'd (spec §8): loaded
# from disk at startup. Gitignored; run this once per dev/deploy environment.
set -euo pipefail
mkdir -p data
curl -fL --progress-bar \
  "https://huggingface.co/Xenova/all-MiniLM-L6-v2/resolve/main/onnx/model_quantized.onnx" \
  -o data/model.onnx
curl -fL --progress-bar \
  "https://huggingface.co/Xenova/all-MiniLM-L6-v2/resolve/main/vocab.txt" \
  -o data/vocab.txt
echo "fetched: $(wc -c < data/model.onnx) bytes model, $(wc -l < data/vocab.txt) vocab lines"
