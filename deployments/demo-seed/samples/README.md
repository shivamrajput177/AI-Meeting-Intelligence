# Demo seed samples

`scripts/seed-demo-org.sh` uploads every file in this directory through the
real API (signup → presigned upload → confirm-upload → poll status) as the
Phase 7 public demo org's pre-processed sample meetings — see
`docs/architecture/deployment-demo-strategy.md` §2's "3-4 pre-processed
sample meetings" requirement.

**This directory is intentionally empty of actual audio in this repo.**
This project's own development sandbox cannot source real recordings (no
microphone, no network access to download sample audio, and fabricating
"meeting audio" would make the demo's first impression a lie about what the
pipeline actually processed). Before running the seed script against a real
deployment, drop 3-4 short (under the demo's ~25MB / ~2min clip-length cap
— see `../../configs/demo/README.md`) real audio recordings here, named
however you want the meeting title to read (the script titles each meeting
from the filename, extension stripped) — e.g.:

```
deployments/demo-seed/samples/
├── Sprint Planning.m4a
├── Customer Onboarding Call.wav
└── Weekly Standup.mp3
```

Anything whisper.cpp can transcribe works — short clips of yourself talking
through a few agenda items and action items make for a more convincing demo
than silence or music, since the downstream summary/action-item/RAG steps
all depend on there being real speech content to extract from.
