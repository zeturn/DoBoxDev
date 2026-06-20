# Security Policy

DoBoxDev is a Docker management tool and can access Docker Engine. Treat any deployment of this project as security-sensitive.

## Supported Versions

The `main` branch is the only supported development line at this stage.

## Reporting a Vulnerability

Please do not disclose vulnerabilities publicly before maintainers have had time to investigate.

To report a vulnerability, open a private security advisory on GitHub when available, or contact the repository owner through GitHub with:

- a short description of the issue
- affected commit, branch, or version
- reproduction steps
- expected impact
- any suggested mitigation

## Security Expectations

Before running this project outside a local development environment:

- replace the default `JWT_SECRET`
- restrict access to Docker socket or Docker API endpoints
- limit allowed CORS origins
- enforce HTTPS at the edge
- set CPU, memory, network, and image-source constraints for managed containers
- avoid committing secrets, tokens, private keys, databases, or generated runtime artifacts
