workspace(name = "baseplate_go")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

GO_VERSION = "1.15.6"

# For rules_go
RULES_GO_VERSION = "v0.25.0"

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "6f111c57fd50baf5b8ee9d63024874dd2a014b069426156c55adbf6d3d22cb7b",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/%s/rules_go-%s.tar.gz" % (RULES_GO_VERSION, RULES_GO_VERSION),
        "https://github.com/bazelbuild/rules_go/releases/download/%s/rules_go-%s.tar.gz" % (RULES_GO_VERSION, RULES_GO_VERSION),
    ],
)

# For gazelle
GAZELLE_VERSION = "v0.22.2"

http_archive(
    name = "bazel_gazelle",
    sha256 = "b85f48fa105c4403326e9525ad2b2cc437babaa6e15a3fc0b1dbab0ab064bc7c",
    urls = [
        "https://storage.googleapis.com/bazel-mirror/github.com/bazelbuild/bazel-gazelle/releases/download/%s/bazel-gazelle-%s.tar.gz" % (GAZELLE_VERSION, GAZELLE_VERSION),
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/%s/bazel-gazelle-%s.tar.gz" % (GAZELLE_VERSION, GAZELLE_VERSION),
    ],
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")
load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")

go_rules_dependencies()

go_register_toolchains(version = GO_VERSION)

gazelle_dependencies()

load("//:external.bzl", "go_dependencies")

# gazelle:repository_macro external.bzl%go_dependencies
go_dependencies()
