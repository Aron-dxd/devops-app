# Taskflow

A small Go REST API for managing tasks

## Installation

### Prerequisites
- Go 1.22+
- Docker

### Clone the repo
```bash
git clone https://github.com/Aron-dxd/devops-app.git 
cd devops-app
```

### Download and run it
```
go mod download
go run .
```

### With Nix (optional)
A flake for nix is available.
```bash
nix develop
go mod download
go run .
```

Server starts on `:8080`.

### Run the tests
```bash
go test ./... -v
```

### Build the container
```bash
docker build -t taskflow .
docker run -p 8080:8080 taskflow
```
