# bazelrc-parser-go

This repository contains a parser for [bazelrc files](https://bazel.build/run/bazelrc) written in Go.

It is limited in its accuracy, as it is providing mostly syntactic parsing, and does not have a full awareness of how Bazel itself parses config (which also changes over time).

Some of its known limitations:
* It treats config-gated settings as independent commands, including platform-specific configs (i.e. `build:remote --jobs=100` is independent of `build --jobs=200`).
* It does not understand what settings accumulate multiple uses (i.e. it doesn't know that `foo` is relevant in `build --extra_toolchains=foo --extra_toolchains=bar`).
* It does not know about the types of values that are expected, so e.g. doesn't know that boolean flags may coerce `0` and `1` to `false` and `true`.
* Its handling of spaces inside flags (e.g. `--copt="--foo --bar"`) may not exactly match Bazel's.

There's plausibly a space for expanding the `bazel canonicalize-flags` command to make this library obsolete. Some of the limitations of `bazel canonicalize-flags` are:
* It require invoking bazel (and require setting up a whole server so is slow, requires the server lock, and may invalidate the analysis cache if not done very carefully)
* It doesn't support reading from `.bazelrc` files at all, so pre-processing would still need to be done to load the flags to pass them to bazel.
  * Also, there isn't an easy way to support `--config` flags, and the order of priority of handling flags enabled by `--config` is one of the more fiddly parts of flag parsing.
* `bazel canonicalize-flags` will error if targets are specified (i.e. `bazel canonicalize-flags -- --jobs=10 //:gazelle` will error), so these would need detecting and stripping out.
* `bazel canonicalize-flags` doesn't provide structured output, just "flag per line", which causes issues for e.g. copts containing newlines.
