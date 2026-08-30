# Copyright (c) the go-tex/engine authors.
# SPDX-License-Identifier: BSD-3-Clause
#
# A pure-Go, CGO=0 LaTeX compile image — a FROM scratch drop-in for the
# debian+TeXLive-full weft-loom-texlive sandbox. The whole image is a single
# static binary with the default font embedded (no TeX Live, no latexmk, no
# biber, no shell). It honours the weft-loom compile contract:
#
#   docker run -v <project>:/workspace:ro -v <scratch>:/workspace/.build:rw IMG \
#     -pdf -outdir=/workspace/.build /workspace/main.tex
#   → /workspace/.build/main.pdf

FROM golang:1.27 AS build
WORKDIR /src
COPY . .
ENV CGO_ENABLED=0 GOFLAGS=-trimpath
RUN go build -ldflags="-s -w" -o /gotex ./cmd/gotex

FROM scratch
COPY --from=build /gotex /gotex
# weft-agent mounts the project read-only at /workspace and the scratch dir at
# /workspace/.build; the microVM provides kernel-level isolation.
WORKDIR /workspace
ENTRYPOINT ["/gotex"]
