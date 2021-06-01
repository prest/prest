load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")
load("@bazel_gazelle//:def.bzl", "gazelle")

# gazelle:prefix github.com/prest/prest
gazelle(name = "gazelle")

go_library(
    name = "prest_lib",
    srcs = ["doc.go"],
    importpath = "github.com/prest/prest",
    visibility = ["//visibility:private"],
)

go_binary(
    name = "prest",
    embed = [":prest_lib"],
    visibility = ["//visibility:public"],
)
