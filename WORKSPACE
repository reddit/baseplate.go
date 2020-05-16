workspace(name = "baseplate_go")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

RULES_GO_VERSION = "v0.23.1"

http_archive(
    name = "io_bazel_rules_go",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/%s/rules_go-%s.tar.gz" % (RULES_GO_VERSION, RULES_GO_VERSION),
        "https://github.com/bazelbuild/rules_go/releases/download/%s/rules_go-%s.tar.gz" % (RULES_GO_VERSION, RULES_GO_VERSION),
    ],
    sha256 = "87f0fb9747854cb76a0a82430adccb6269f7d394237104a4523b51061c469171",
)

load("@io_bazel_rules_go//go:deps.bzl", "go_rules_dependencies", "go_register_toolchains")

go_rules_dependencies()

go_register_toolchains(
    # Note that this is already implicit from rules_go version,
    # but being explicit about which version of go compiler we are using is a
    # good thing anyways.
    go_version = "1.14.3",
)

GAZELLE_VERSION = "v0.21.0"

http_archive(
    name = "bazel_gazelle",
    urls = [
        "https://storage.googleapis.com/bazel-mirror/github.com/bazelbuild/bazel-gazelle/releases/download/%s/bazel-gazelle-%s.tar.gz" % (GAZELLE_VERSION, GAZELLE_VERSION),
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/%s/bazel-gazelle-%s.tar.gz" % (GAZELLE_VERSION, GAZELLE_VERSION),
    ],
    sha256 = "bfd86b3cbe855d6c16c6fce60d76bd51f5c8dbc9cfcaef7a2bb5c1aafd0710e8",
)

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")

gazelle_dependencies()

load("//:external.bzl", "go_dependencies")

# gazelle:repository_macro external.bzl%go_dependencies
go_dependencies()
