# pgloader Linux Server Installation

This document is for a future deployment stage only. Do not install or run pgloader on any server until owner approval and the production safety gate are satisfied.

## 1. Debian / Ubuntu

```bash
sudo apt-get update
sudo apt-get install -y pgloader
pgloader --version
```

## 2. RHEL / AlmaLinux / Rocky Linux

Verify package availability during the upload plan:

```bash
cat /etc/os-release
dnf search pgloader || true
sudo dnf install -y pgloader
pgloader --version
```

If pgloader is unavailable in the default repositories, evaluate the PostgreSQL community RPM repository or an approved external build/package source during the deployment plan. Do not enable repositories or install packages on the server without approval.

## 3. Docker option

For servers that already have Docker and approval to use it:

```bash
docker run --rm dimitri/pgloader:latest pgloader --version
```

## 4. pgloader v4 JAR option

pgloader v4 JAR usage requires Java 21+:

```bash
java -version
java -jar pgloader.jar --version
```

Do not download or run the JAR on the server during planning.

## 5. Production safety gate

Do not run pgloader directly against production unless:

- owner approval exists
- database backup exists
- target database is isolated
- dry-run/staging rehearsal passed
- QantaraDB validation passed
- Console PostgreSQL validation passed
- rollback plan exists
