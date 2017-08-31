---
title: "Contribute"
date: 2017-08-30T19:07:12-03:00
weight: 16
menu: main
---

Did you found an bug or you would like to suggest a new feature? I'm open for feedback. Please open a new [issue](https://github.com/prest/prest/issues) and let me know what you think.

You're also welcome to contribute with [pull requests](https://github.com/prest/prest/pulls).

## Running tests

Clone the repository and create a test database and insert dummy data for specs.

```
PREST_PG_HOST=127.0.0.1 PREST_PG_DATABASE=prest sh ./testdata/schema.sh
```

Run migrations on test database.

```
PREST_PG_HOST=127.0.0.1 PREST_PG_DATABASE=prest sh ./testdata/migrations_test.sh
```

Run tests.

```
PREST_PG_HOST=127.0.0.1 PREST_PG_DATABASE=prest sh ./testdata/test.sh
```

# Contribution Guidelines

## Introduction

This document explains how to contribute changes to the pREST project.

## Bug reports

Please search the issues on the issue tracker with a variety of keywords to ensure your bug is not already reported.

If unique, [open an issue](https://github.com/prest/prest/issues/new) and answer the questions so we can understand and reproduce the problematic behavior.

To show us that the issue you are having is in pREST itself, please write clear, concise instructions so we can reproduce the behavior (even if it seems obvious). The more detailed and specific you are, the faster we can fix the issue. Check out [How to Report Bugs Effectively](http://www.chiark.greenend.org.uk/~sgtatham/bugs.html).

Please be kind, remember that pREST comes at no cost to you, and you're getting free help.

## Discuss your design

The project welcomes submissions but please let everyone know what you're working on if you want to change or add something to the pREST repository.

Before starting to write something new for the pREST project, please [file an issue](https://github.com/prest/prest/issues/new).

This process gives everyone a chance to validate the design, helps prevent duplication of effort, and ensures that the idea fits inside the goals for the project and tools. It also checks that the design is sound before code is written; the code review tool is not the place for high-level discussions.

## Testing redux

Before sending code out for review, run all the tests for the whole tree to make sure the changes don't break other usage and keep the compatibility on upgrade. To make sure you are running the test suite exactly like we do, you should install the CLI for [Travis CI](https://travis-ci.org/), as we are using the server for continous testing.

## Code review

Changes to pREST must be reviewed before they are accepted, no matter who makes the change even if it is an owner or a maintainer.

Please try to make your pull request easy to review for us. Please read the "[How to get faster PR reviews](https://github.com/kubernetes/community/blob/master/contributors/devel/faster_reviews.md)" guide, it has lots of useful tips for any project you may want to contribute. Some of the key points:

* Make small pull requests. The smaller, the faster to review and the more likely it will be merged soon.
* Don't make changes unrelated to your PR. Maybe there are typos on some comments, maybe refactoring would be welcome on a function... but if that is not related to your PR, please make *another* PR for that.
* Split big pull requests into multiple small ones. An incremental change will be faster to review than a huge PR.

## Sign your work

The sign-off is a simple line at the end of the explanation for the patch. Your signature certifies that you wrote the patch or otherwise have the right to pass it on as an open-source patch. The rules are pretty simple: If you can certify [DCO](DCO), then you just add a line to every git commit message:

```
Signed-off-by: Thiago Avelino <avelino@email.com>
```

Please use your real name, we really dislike pseudonyms or anonymous contributions. We are in the open-source world without secrets. If you set your `user.name` and `user.email` git configs, you can sign your commit automatically with `git commit -s`.

## Maintainers

To make sure every PR is checked, we have [team maintainers](MAINTAINERS). Every PR **MUST** be reviewed by at least two maintainers (or owners) before it can get merged. A maintainer should be a contributor of pREST and contributed at least 4 accepted PRs. A contributor should apply as a maintainer in the [Gitter develop channel](https://gitter.im/prest/prest). The owners or the team maintainers may invite the contributor. A maintainer should spend some time on code reviews. If a maintainer has no time to do that, they should apply to leave the maintainers team and we will give them the honor of being a member of the **advisors team**. Of course, if an advisor has time to code review, we will gladly welcome them back to the maintainers team. If a maintainer is inactive for more than 3 months and forgets to leave the maintainers team, the owners may move him or her from the maintainers team to the advisors team.

## Owners

Since pREST is maintained by Community, [Nuveo](https://nuveo.com.br/en) (company that has idealized) does not provide professional support for pREST, to keep the development healthy we will elect three owners every year. All contributors may vote to elect up to three candidates, one of which will be the main owner, and the other two the assistant owners. When the new owners have been elected, the old owners will give up ownership to the newly elected owners. If an owner is unable to do so, the other owners will assist in ceding ownership to the newly elected owners.

After the election, the new owners should proactively agree with our [CONTRIBUTING](CONTRIBUTING.md) requirements on the [Gitter main channel](https://gitter.im/prest/prest). Below are the words to speak:

```
I'm honored to having been elected an owner of pREST, I agree with [CONTRIBUTING](CONTRIBUTING.md). I will spend part of my time on pREST and lead the development of pREST.
```

## Versions

pREST has the `master` branch as a tip branch and has version branches such as `v0.1`. `v0.1` is a release branch and we will tag `v0.1.0` for binary download. If `v0.1.0` has bugs, we will accept pull requests on the `v0.1` branch and publish a `v0.1.1` tag, after bringing the bug fix also to the master branch.

Since the `master` branch is a tip version, if you wish to use pREST in production, please download the latest release tag version. All the branches will be protected via GitHub, all the PRs to every branch must be reviewed by two maintainers and must pass the automatic tests.

## Copyright

Code that you contribute should use the standard copyright header:

```
// Copyright 2017 The pREST Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
```

Files in the repository contain copyright from the year they are added to the year they are last changed. If the copyright author is changed, just paste the header below the old one.
