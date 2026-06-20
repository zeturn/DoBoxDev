# Contributing

Thanks for helping improve DoBoxDev. This project is currently a learning and demonstration Docker management tool, so changes should keep the code easy to understand, verify, and run locally.

## Development Setup

Backend:

```bash
cd backend
cp .env.example .env
go mod download
go run cmd/server/main.go
```

Frontend:

```bash
cd frontend
npm install
npm run dev
```

## Before Opening a Pull Request

Run the checks that match your change:

```bash
cd backend
go test ./...
go vet ./...
```

```bash
cd frontend
npm ci
npm run lint
npm run build
```

## Pull Request Checklist

- Explain what changed and why.
- Include screenshots or API examples for user-facing changes when helpful.
- Update `README.md` or other docs when setup, config, API, or behavior changes.
- Mention any security impact, especially Docker socket access, auth, CORS, secrets, or container limits.
- Keep unrelated refactors out of feature and bug-fix PRs.

## License

By contributing, you agree that your contribution is provided under the ISC License used by this repository.
