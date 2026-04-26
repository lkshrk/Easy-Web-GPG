# Easy Web GPG

<div align="center">
  <img src=".github/assets/demo.gif" alt="Easy Web GPG demo" style="max-width:900px;width:100%;height:auto;display:block;margin:0 auto;" />
</div>

Lightweight web UI for encrypting and decrypting PGP messages, with master-password-protected access and AES-encrypted key storage.

## Docker

```bash
docker run --rm \
  -p 8080:8080 \
  -e MASTER_PASSWORD=your-secret \
  -v ./data:/data \
  ghcr.io/lkshrk/easy-web-gpg:latest
```

Open http://localhost:8080

## Configuration

| Variable | Required | Description |
|---|:---:|---|
| `MASTER_PASSWORD` | ✓ | Master password for deriving encryption keys (Argon2id) |
| `DATABASE_URL` | | Database connection string (default SQLite, Postgres supported) |
| `PORT` | | HTTP port (default: `8080`) |
| `FORCE_SECURE_COOKIES` | | Set to `1` for HTTPS environments |

## Development

```bash
export MASTER_PASSWORD=test123
make dev      # live reload (Go + CSS)
make test     # run tests
make build    # build binary
```

Requires Docker. The dev environment hot-reloads Go, templates, and Tailwind CSS.

## Contributing

Contributions welcome — open an issue or PR. See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

[MIT](LICENSE)
