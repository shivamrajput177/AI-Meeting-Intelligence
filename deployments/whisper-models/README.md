# whisper.cpp models

This directory is bind-mounted read-only into the `whisper` service at
`/models` (see `../docker-compose.yaml`). It's empty in git — download a
real GGML model file into it before `docker compose up` will produce a
usable transcription:

```bash
# from a whisper.cpp checkout:
./models/download-ggml-model.sh base.en
cp models/ggml-base.en.bin <this-repo>/deployments/whisper-models/
```

See `docs/architecture/microservices.md` §6 for why `base`/`small` models
are this project's dev-sized default, and the `whisper` service's own
comment in `docker-compose.yaml` for why this whole setup is unverified
in this sandbox (no container-registry access to actually pull and run
whisper.cpp's server image).
