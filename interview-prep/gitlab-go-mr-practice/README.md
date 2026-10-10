# GitLab Backend Engineer (Go) – MR review practice kit

A self-contained mock of the GitLab technical interview: a small Go service
(`endpoint-monitor`) plus a merge request that adds concurrency to it. The MR
looks reasonable and its CI is green, but it has bugs of the kind the real
interview asks about.

**Start with [`PREP_GUIDE.md`](PREP_GUIDE.md).** Do not open `ANSWER_KEY.md`
or `solution/` until you have submitted your own review.

## Contents

| Path | What it is |
|------|------------|
| `PREP_GUIDE.md` | How to prepare, review workflow, Go concurrency notes, likely questions, live-coding tips |
| `setup.sh` | Builds the practice repository and, if you give it a GitLab URL, creates a private project and opens the MR |
| `MR_DESCRIPTION.md` | The MR description to paste into GitLab |
| `base/` | Source of `main` |
| `mr/`, `mr-commit1/` | Source of the MR branch (two commits) |
| `ANSWER_KEY.md` | **Spoilers.** Every planted issue, how to spot it and how to fix it, plus a mock interview script |
| `solution/` | **Spoilers.** A reference fix with tests that pass under `-race` |

## Setup (about 5 minutes)

You need Go 1.22 or newer and an SSH key added to GitLab.

```shell
# 1. Create the project on GitLab and open the MR in one go.
#    Pushing to a path that doesn't exist yet creates it as a PRIVATE project.
./interview-prep/gitlab-go-mr-practice/setup.sh ~/code/endpoint-monitor \
    git@gitlab.com:<your-gitlab-username>/endpoint-monitor.git

# 2. In GitLab, open the MR ("Check endpoints concurrently and add -once mode")
#    and paste MR_DESCRIPTION.md into its description.

# 3. Check that everything runs locally, as the interview requires.
cd ~/code/endpoint-monitor
git checkout feature/concurrent-checks
go test ./...
```

If push-to-create is turned off for your account, create an empty **private**
project called `endpoint-monitor` in the GitLab UI (untick "Initialize
repository with a README") and run the same command again in a new target
directory.

No GitLab account handy? Run `./setup.sh ~/code/endpoint-monitor` without the
URL and review `git diff main...feature/concurrent-checks` locally.

## Exercise

Do it as the real thing, with a timer.

1. **Review (60 to 90 minutes).** Leave inline comments on the MR in GitLab
   using Conventional Comments, then post a summary note and choose
   "Request changes".
2. **Self-check.** Compare against `ANSWER_KEY.md`. Count what you found, what
   you missed, and anything you flagged that isn't actually a bug.
3. **Live coding (45 minutes).** On a new branch, without looking at
   `solution/`, fix the top three issues and write tests that fail before your
   fix and pass after it. Talk out loud the whole time, ideally while
   recording yourself.
4. **Compare** with `solution/` and run `go test -race -count=20 ./...`.
