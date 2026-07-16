# Coverage PR comments

How unit-test coverage comments are posted on pull requests.

## Same-repository PRs

Job **Coverage PR comment** in [`.github/workflows/test-unit.yml`](workflows/test-unit.yml) runs on `pull_request` when the head repo is `prest/prest`. It uses `GITHUB_TOKEN` with `pull-requests: write` and the `fgrosse/go-coverage-report` action (comments directly).

## Fork PRs

GitHub **does not pass secrets** (repository or organization) to workflows triggered by a `pull_request` from a fork. Organization secret `PR_TOKEN` is therefore unavailable in that context.

Flow:

1. **Generate** — job **Coverage PR comment (fork)** in `test-unit.yml` builds the markdown with `skip-comment: true` and uploads artifact `coverage-pr-comment`.
2. **Post** — workflow [`.github/workflows/coverage-pr-comment-fork.yml`](workflows/coverage-pr-comment-fork.yml) runs on `workflow_run` after `test-unit` succeeds. In the base-repo context it can read `secrets.PR_TOKEN` and posts (or replaces) the comment.

### Maintainer setup

1. Create an organization secret named **`PR_TOKEN`** (fine-scoped PAT that can comment on PRs in `prest/prest`).
2. Allow that org secret for the **`prest/prest`** repository (organization secret repository access list).
3. The `workflow_run` job **fails** if `PR_TOKEN` is empty or not available — intentional, so misconfiguration is visible.

Do not store the PAT in an Actions **variable**; variables for fork PRs are readable by the pull request workflow and must not hold credentials.
