load("@bazel_tools//tools/build_defs/pkg:pkg.bzl", "pkg_tar")
load("@io_bazel_rules_docker//container:container.bzl", "container_image")
load("@io_bazel_rules_docker//contrib:passwd.bzl", "passwd_entry", "passwd_file")
load("@bazel_gazelle//:def.bzl", "gazelle")

# gazelle:prefix github.com/prest/prest
gazelle(name = "gazelle")

# create nonroot user and uid
passwd_entry(
	name = "nonroot_user",
	info = "nonroot",
	uid = 1002,
	username = "nonroot"
)

passwd_file(
	name = "passwd",
	entries = [
		":nonroot_user",
	],
)

pkg_tar(
	name = "passwd_tar",
	srcs = [":passwd"],
	mode = "0644",
	package_dir = "etc",
)

# create package with etc files
pkg_tar(
  name = "config_tar",
  srcs = glob(["etc/**"]),
  mode = "0755",
  package_dir = "/app",
  strip_prefix = "./etc",
)

container_image(
    name = "prest_base_image",
    base = "@alpine_linux_amd64//image",
    workdir = "/app",
    tars = [":config_tar", ":passwd_tar"],
		user = "nonroot",
    visibility = ["//visibility:public"]
)