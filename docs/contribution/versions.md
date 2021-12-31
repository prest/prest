---
title: "Version & Branch - Patterns"
date: 2021-09-02T13:39:24-03:00
weight: 4
---

_**prestd**_ has the `main` branch as a tip branch and has version branches such as `v1.1`. `v1.1` is a release branch and we will tag `v1.1.0` for binary download. If `v1.1.0` has bugs, we will accept pull requests on the `v1.1` branch and publish a `v1.1.1` tag, after bringing the bug fix also to the main branch.

Since the `main` branch is a tip version, if you wish to use pREST in production, please download the latest release tag version. All the branches will be protected via GitHub, all the PRs to every branch must be reviewed by two maintainers and must pass the automatic tests.
