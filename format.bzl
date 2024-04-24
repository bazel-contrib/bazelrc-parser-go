load("@aspect_rules_lint//format:defs.bzl", _format_test = "format_test")

# Work around https://github.com/bazelbuild/bazel/issues/11875
def glob(patterns):
    kwargs = {}
    if native.package_name() == "":
        kwargs["exclude"] = ["bazel-*/**"]
    return native.glob(patterns, **kwargs)

def format_test():
    # rules_lint by default creates targets for each of these languages (except starlark, which we need to add because it doesn't have a default tool label), which then no-op (but require fetching toolchains and building potentially large runfiles trees).
    # Ideally it would do this filtering itself in its format_test macro, but until it does, we can do it here.
    # File extensions are taken from https://github.com/aspect-build/rules_lint/blob/bba2ba062c809ce2ec2bdc2a2ca387dbb4c6b567/format/private/format.sh#L56-L83
    src_map = {
        "go": glob(["**/*.go"]),
        "jsonnet": glob(["**/*.jsonnet", "**/*.libsonnet"]),
        "shell": glob(["**/*.sh"]),
        "starlark": glob(["**/BUILD", "**/BUILD.bazel", "**/MODULE.bazel", "**/WORKSPACE", "**/WORKSPACE.bazel", "**/*.bzl", "**/*.star"]),
        "terraform": glob(["**/*.tf", "**/*.tfvars"]),
        "yaml": glob(["**/*.yml", "**/*.yaml", "**/.clang-format", "**/.clang-tidy", "**/.gemrc"]),
    }

    kwargs = {}
    srcs = []
    for lang, lang_srcs in src_map.items():
        if not lang_srcs:
            kwargs[lang] = False
        else:
            srcs += lang_srcs
            if lang == "starlark":
                kwargs["starlark"] = "@buildifier_prebuilt//:buildifier"

    _format_test(
        name = "format_test",
        srcs = srcs,
        **kwargs
    )
