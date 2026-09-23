# whisper.cpp models

This directory is bind-mounted into the `whisper` service (read-only)
and the `whisper-model-init` service (read-write) at `/models` — see
`../docker-compose.yaml`. It's empty in git: the `whisper-model-init`
service downloads `ggml-base.en.bin` (~140MB) into it automatically on
`docker compose up`/`make up`, before `whisper` itself starts, and skips
the download on every run after the first (it checks whether the file
already exists). Nothing to do by hand for the default setup.

**To use a different model size** (e.g. `tiny.en` for a faster, less
accurate dev loop, or `small`/`medium` for better accuracy), download it
yourself and point `whisper`'s `command:` in `../docker-compose.yaml` at
it instead:

```bash
# from a whisper.cpp checkout:
./models/download-ggml-model.sh tiny.en
cp models/ggml-tiny.en.bin <this-repo>/deployments/whisper-models/
```
then change `-m /models/ggml-base.en.bin` to `-m /models/ggml-tiny.en.bin`
in the `whisper` service's `command:`.

See `docs/architecture/microservices.md` §6 for why `base`/`small` models
are this project's dev-sized default, and the `whisper` service's own
comment in `docker-compose.yaml` for the Apple Silicon (`linux/arm64`)
caveat — the image/command pair here is otherwise unverified end-to-end
(this project was built in a sandbox with no container-registry access).
