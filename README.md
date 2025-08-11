# Easy Web GPG

Lightweight web UI for encrypting/decrypting small PGP blobs.

<div align="center">
  <img src="https://raw.githubusercontent.com/lkshrk/Easy-Web-GPG/ci/screenshot/.github/screenshot.png" alt="Easy Web GPG screenshot" style="max-width:900px;width:100%;height:auto;display:block;margin:0 auto;" />
</div>

This project provides:

- A minimal Tailwind-styled web interface.
- Master-password gated access with HMAC-signed cookies (24h).
- AES-encrypted, recoverable passphrase storage encrypted with a key derived from `MASTER_PASSWORD`.

Quick links

- Server entrypoint: `./cmd/easywebgpg`
- Templates: `templates/`
- Static assets: `static/` (Tailwind CLI build outputs to `static/dist`)

Requirements

- Go 1.20+ to build the server
- Node.js + npm (only required to build the Tailwind CSS asset locally)

Environment

The app is configured with environment variables. Example values and purpose:

| Variable | Required | Description |
|---|:---:|---|
| `MASTER_PASSWORD` | yes | Plaintext master password used to derive the 32-byte master key via Argon2id. A random salt is persisted into the application's database (table `secrets`, name `master_salt`) on first run. Back up your database to preserve the salt and allow recovery of encrypted passphrases. |
| `DATABASE_URL` | optional | If set (e.g. Postgres), runtime runs migrations against it. Otherwise the app uses a local sqlite file for convenience. |
| `PORT` | optional | HTTP port (default: `8080`). |
| `ARGON2_TIME` | optional | Argon2id time parameter (default `1`). |
| `ARGON2_MEMORY_KB` | optional | Argon2id memory in KB (default `32768`). |
| `ARGON2_THREADS` | optional | Argon2id threads (default `2`). |
| `FORCE_SECURE_COOKIES` | optional | When set to `1`, cookies will be marked `Secure` (useful behind HTTPS). |

Tip: the app derives a 32-byte key from `MASTER_PASSWORD` using Argon2id and a persisted salt stored in the application database (table `secrets`, name `master_salt`). You only need to set `MASTER_PASSWORD`; the app will create the salt on first run and store it in the DB. Back up your DB to preserve the salt.

Quick start (development)

1. Build Tailwind CSS (optional locally):

```bash
# builds CSS (runs npm install if needed)
make css
```

2. Run the server locally:

```bash
# set your master password, then run the development server via Makefile
export MASTER_PASSWORD='your-secret-password'
make run-dev
```

Note on run targets:

- `make run-dev`: builds a development binary (with debug info) and runs it; useful when iterating locally. It also expects CSS to be built or will use the `static/dist` fallback.
- `make run`: builds the production binary and runs it; this is closer to what the Docker image will run and produces an optimized binary in `bin/`.


Run tests

```bash
make test
```

Docker

Build and run the container using the Makefile targets. Ensure the CSS is built (CI or local) so `static/dist/styles.css` exists.

```bash
# build CSS locally if needed
make css

# build the docker image and run it
make docker-build
make docker-run
```

CI

The repository may include a GitHub Actions workflow that runs tests and builds the image on every push; publishing to a registry should be gated to runs on `main` or tags. Recommended CI steps using the Makefile targets:

- Build CSS: `make css`
- Run tests: `make test`
- Build artifacts / binary: `make build` (or `make build-dev` for debug)
- Build Docker image: `make docker-build`

Ensure the workflow runs `make css` before `make docker-build` so `static/dist/styles.css` is present in the image.

Security notes

- Keep `MASTER_PASSWORD` secret; the derived key (via Argon2id + salt) is used to encrypt stored passphrases and sign auth cookies.
- The salt is stored in the DB under `secrets.name = 'master_salt'`. Back up your DB so you can recover encrypted data after restores.
- Prefer using a secrets manager to supply `MASTER_PASSWORD` in production; avoid committing plaintext credentials.
- For production, run behind HTTPS and set `FORCE_SECURE_COOKIES=1` so auth cookies are marked `Secure`.

Contributing

Contributions are welcome. Open an issue or pull request with proposed changes. If you change the database schema, add a migration in `migrations/sql`.

License

This project is provided as-is. See the repository for license details.
