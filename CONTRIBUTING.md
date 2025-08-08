Contributing
============

Thanks for your interest in contributing to Web-GPG. Contributions are welcome and appreciated.

How to contribute

- Fork the repository and create a branch for your change: `feature/your-change`.
- Open a pull request describing your change, why it's needed, and any relevant notes.
- Keep PRs small and focused; add tests for new behavior where appropriate.

Code style and quality

- Go code: follow `gofmt` and `go vet` recommendations.
- JavaScript/CSS: keep frontend changes small and document any additions to the asset pipeline.

Development workflow

Use the devcontainer for a consistent environment: run **Dev Containers: Rebuild Container** in VS Code.
Run the server locally with `go run .` and open `http://localhost:8080`.

Security

- Do not commit secrets, private keys, or database files. If you need to share sensitive data for reproduction, redact them and provide steps to recreate the state.

Reporting issues

- Open an issue with a clear title, description, and reproduction steps.
