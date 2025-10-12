# Easy Web GPG

Lightweight web UI for encrypting/decrypting PGP messages.

<div align="center">
  <img src="https://raw.githubusercontent.com/lkshrk/Easy-Web-GPG/ci/screenshot/.github/screenshot.png" alt="Easy Web GPG screenshot" style="max-width:900px;width:100%;height:auto;display:block;margin:0 auto;" />
</div>

## Features

- Minimal Tailwind-styled web interface
- Master-password protected access (24h sessions)
- AES-encrypted passphrase storage

## Quick Start

```bash
export MASTER_PASSWORD='your-secret-password'
make run
```

Open http://localhost:8080

## Configuration

| Variable | Required | Description |
|---|:---:|---|
| `MASTER_PASSWORD` | yes | Master password for deriving encryption keys (Argon2id) |
| `DATABASE_URL` | no | Database connection (defaults to SQLite) |
| `PORT` | no | HTTP port (default: `8080`) |
| `FORCE_SECURE_COOKIES` | no | Set to `1` for HTTPS environments |

## Development

```bash
# Run tests
make test

# Build binary
make build

# Use devcontainer for full dev environment
# (includes Go, Tailwind CLI, all tools)
```

See [docs/OPTIMIZATION.md](docs/OPTIMIZATION.md) for architecture details and CI/CD pipeline information.

## Security

⚠️ **Important:**
- Keep `MASTER_PASSWORD` secret and use a secrets manager in production
- Run behind HTTPS with `FORCE_SECURE_COOKIES=1`
- Back up your database to preserve the encryption salt

## Contributing

Contributions welcome. Open an issue or PR with proposed changes.

## License

See repository for license details.
