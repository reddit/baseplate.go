workspace(name = "baseplate_go")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

GO_VERSION = "1.15.1"

# For rules_go
RULES_GO_VERSION = "v0.24.1"

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "7f1aa43d986df189f7cf30e81dd2dc9d8ed7c74e356341a17267f6d7e5748382",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/%s/rules_go-%s.tar.gz" % (RULES_GO_VERSION, RULES_GO_VERSION),
        "https://github.com/bazelbuild/rules_go/releases/download/%s/rules_go-%s.tar.gz" % (RULES_GO_VERSION, RULES_GO_VERSION),
    ],
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains(
    # Note that this is already implicit from rules_go version,
    # but being explicit about which version of go compiler we are using is a
    # good thing anyways.
    go_version = GO_VERSION,
)

# For gazelle
GAZELLE_VERSION = "v0.22.0"

http_archive(
    name = "bazel_gazelle",
    sha256 = "d4113967ab451dd4d2d767c3ca5f927fec4b30f3b2c6f8135a2033b9c05a5687",
    urls = [
        "https://storage.googleapis.com/bazel-mirror/github.com/bazelbuild/bazel-gazelle/releases/download/%s/bazel-gazelle-%s.tar.gz" % (GAZELLE_VERSION, GAZELLE_VERSION),
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/%s/bazel-gazelle-%s.tar.gz" % (GAZELLE_VERSION, GAZELLE_VERSION),
    ],
)

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")

gazelle_dependencies()

load("//:external.bzl", "go_dependencies")

# gazelle:repository_macro external.bzl%go_dependencies
go_dependencies()
