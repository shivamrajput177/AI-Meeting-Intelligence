# GitLab MR-review interview: practice kit

| File | What it is |
|---|---|
| `PREP_GUIDE.md` | **Read first.** The interview format, a review workflow, Go concurrency, error handling, testing, likely questions, and tips for the live session. |
| `linkcheck.bundle` | A practice Go repo (`main` plus the MR branch `concurrent-link-checks`). |
| `setup-gitlab-mr.sh` | Creates a **private GitLab project** from the bundle and opens a real MR. |
| `MR_DESCRIPTION.md` | The MR description (the script uses it automatically if you set `GITLAB_TOKEN`). |
| `ANSWER_KEY.md` | ⚠️ Spoilers: every planted issue, example comments, a summary comment and practice drills. |
| `solution.patch` | A verified reference fix (`go vet` clean, `go test -race` green). |

## Set up the practice MR on GitLab (on your laptop)

```shell
git clone -b claude/stoic-fermat-szup8a https://github.com/shivamrajput177/AI-Meeting-Intelligence.git
cd AI-Meeting-Intelligence/interview-prep

# Option A: full MR description through the API.
# Create a token at GitLab > Preferences > Access tokens, with the `api` scope.
GITLAB_TOKEN=glpat-xxxx ./setup-gitlab-mr.sh <your-gitlab-username>

# Option B: no token. The MR is created with git push options; paste
# MR_DESCRIPTION.md into the MR description yourself.
./setup-gitlab-mr.sh <your-gitlab-username>
```

The script:
1. clones the bundle into `./linkcheck`;
2. pushes `main` to `gitlab.com/<you>/linkcheck`, which GitLab creates as a **private** project;
3. pushes the MR branch and opens a merge request.

It needs an SSH key on GitLab. For HTTPS, set
`REMOTE=https://gitlab.com/<you>/linkcheck.git`.
GitLab CI (`.gitlab-ci.yml`) runs on the MR and passes, just like the real thing.

## Simulate the interview
1. Set a timer for 2 hours. Review the MR on GitLab with
   [Conventional Comments](https://conventionalcomments.org/), then **Submit review**.
2. Only then compare with `ANSWER_KEY.md`.
3. Do the timed drills at the end of the answer key **out loud**, as if pairing.
