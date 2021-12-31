---
title: "Contribution"
date: 2017-08-30T19:07:12-03:00
weight: 6
menu: main
---

This document explains how to contribute changes to the prestd's projects.

Did you found an bug or you would like to suggest a new feature? I'm open for feedback. Please open a new [issue](https://github.com/prest/prest/issues) and let me know what you think.

You're also welcome to contribute with [pull requests](https://github.com/prest/prest/pulls).

## Bug reports

Please search the issues on the issue tracker with a variety of keywords to ensure your bug is not already reported.

If unique, [open an issue](https://github.com/prest/prest/issues/new) and answer the questions so we can understand and reproduce the problematic behavior.

To show us that the issue you are having is in pREST itself, please write clear, concise instructions so we can reproduce the behavior (even if it seems obvious). The more detailed and specific you are, the faster we can fix the issue. Check out [How to Report Bugs Effectively](http://www.chiark.greenend.org.uk/~sgtatham/bugs.html).

Please be kind, remember that pREST comes at no cost to you, and you're getting free help.

## Discuss your design

The project welcomes submissions but please let everyone know what you're working on if you want to change or add something to the pREST repository.

Before starting to write something new for the pREST project, please [open discussion here](https://github.com/prest/prest/discussions/new).

This process gives everyone a chance to validate the design, helps prevent duplication of effort, and ensures that the idea fits inside the goals for the project and tools. It also checks that the design is sound before code is written; the code review tool is not the place for high-level discussions.

## Testing redux

Before sending code out for review, run all the tests for the whole tree to make sure the changes don't break other usage and keep the compatibility on upgrade. To make sure you are running the test suite exactly like we do - the tests are run in [GitHub Actions](https://github.com/features/actions), I recommend reading [Development Guides](/contribute/development-guide) that explains how to run the tests locally.

## Code review

Changes to pREST must be reviewed before they are accepted, no matter who makes the change even if it is an owner or a maintainer.

Please try to make your pull request easy to review for us. Please read the _"[How to get faster PR reviews](https://github.com/kubernetes/community/blob/main/contributors/devel/faster_reviews.md)"_ guide, it has lots of useful tips for any project you may want to contribute. Some of the key points:

- Make small pull requests. The smaller, the faster to review and the more likely it will be merged soon.
- Don't make changes unrelated to your PR. Maybe there are typos on some comments, maybe refactoring would be welcome on a function... but if that is not related to your PR, please make *another* PR for that.
- Split big pull requests into multiple small ones. An incremental change will be faster to review than a huge PR.

## Code of Conduct

This project and everyone participating in it are governed by the [prestd code of conduct](/contribute/code-of-conduct). By participating, you are expected to uphold this code. Please read the [full text](/contribute/code-of-conduct) so that you can read which actions may or may not be tolerated.

## Contributor License Agreement (CLA)

[![CLA assistant](https://cla-assistant.io/readme/badge/prest/prest)](https://cla-assistant.io/prest/prest)

In order to accept your pull request, we need you to submit a CLA. You only need to do this once. If you are submitting a pull request for the first time, you can complete your CLA [here](https://cla-assistant.io/prest/prest) or just submit a Pull Request and our CLA Bot will ask you to sign the CLA before merging your Pull Request.

### Company

If you are making contributions to our repositories on behalf of your company, then we will need a Corporate Contributor License Agreement (CLA) signed. In order to do that, please contact us at [opensource@prestd.com](mailto:opensource@prestd.com).

## Maintainers

To make sure every PR is checked, we have [team maintainers](https://github.com/orgs/prest/people).

Every PR **MUST** be reviewed by at least two maintainers (or owners) before it can get merged. A maintainer should be a contributor of pREST and contributed at least 4 accepted PRs. A contributor should apply as a maintainer in the [Github Discussions](https://github.com/prest/prest/discussions).

The team maintainers may invite the contributor. A maintainer should spend some time on code reviews. If a maintainer has no time to do that, they should apply to leave the maintainers team and we will give them the honor of being a member of the **advisors team**. Of course, if an advisor has time to code review, we will gladly welcome them back to the maintainers team. If a maintainer is inactive for more than 3 months and forgets to leave the maintainers team, the owners may move him or her from the maintainers team to the advisors team.

## Owners

All projects are supported by the community and [prestd, LLC](https://prestd.com/) (we do not own but **support the community**), to keep the development healthy we will elect three owners every year. All contributors may vote to elect up to three candidates, one of which will be the main owner, and the other two the assistant owners. When the new owners have been elected, the old owners will give up ownership to the newly elected owners. If an owner is unable to do so, the other owners will assist in ceding ownership to the newly elected owners.

After the election, the new owners should proactively agree with our _CONTRIBUTING (this page)_ requirements on the [Github Discussions](https://github.com/prest/prest/discussions). Below are the words to speak:

## Copyright

Code that you contribute should use the standard copyright header:

```
// Copyright 2016 The prestd Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
```

Files in the repository contain copyright from the year they are added to the year they are last changed. If the copyright author is changed, just paste the header below the old one.
