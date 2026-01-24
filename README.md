
# Dependency-Track Lifecycle CLI

[![Release](https://github.com/MedUnes/dtrack-cli/actions/workflows/release.yml/badge.svg)](https://github.com/MedUnes/dtrack-cli/actions/workflows/release.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/medunes/dtrack-cli)](https://goreportcard.com/report/github.com/medunes/dtrack-cli)
[![License](https://img.shields.io/github/license/medunes/dtrack-cli)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/medunes/dtrack-cli.svg)](https://pkg.go.dev/github.com/medunes/dtrack-cli)

A Go-based CLI tool to automate the upload and lifecycle management of Software Bill of Materials (SBOM) in [OWASP Dependency-Track](https://dependencytrack.org/).

This tool bridges the gap between simple API uploads and full CI/CD lifecycle management by handling **version sprawl**, **active states**, and **latest version tagging** in a single execution.

## 🚀 Why this tool? (The Gap)

If you simply use `curl` to upload an SBOM to Dependency-Track, you encounter two major problems over time. These are well-documented pain points in the community:

### 1. The "Version Sprawl" Problem

Every CI build creates a new version. If you have 100 builds, you have 100 "Active" versions. Dependency-Track monitors *all* active versions for vulnerabilities, meaning you will receive alerts for vulnerabilities in old, undeployed versions.

* **Community Validation:** Users have explicitly requested an `isActiveExclusively` flag to solve this, noting that "Over time there will be hundreds of 'active' versions, even though they are actually not 'active'".
* **Current Workaround:** Teams currently resort to manual housekeeping or complex scripts to set "dirty tags to inactive" to avoid polluting their risk score.
* **Our Solution:** The `-clean` flag automatically iterates through project versions and sets old ones to `active: false`.

### 2. The "Latest Version" Ambiguity

Dependency-Track attempts to guess the "latest" version, but it is not always accurate (e.g., when patching older release branches or dealing with pre-releases).

* **Community Validation:** Users have reported issues where Dependency-Track incorrectly identifies pre-release versions as "latest," skewing "Outdated Component" analysis metrics.
* **Our Solution:** The `-latest` flag explicitly forces the version you are currently uploading to be marked `isLatest=true`, ensuring your "Outdated Component" metrics are calculated against the correct baseline.

### 3. Missing Auto-Purge/Cleanup

There is no built-in native feature to "keep only the last X versions" or "purge old versions" during upload.

* **Community Validation:** Feature requests for "automatic purging of projects" have been raised by users who find it "hard to do this manually" for projects with many releases.
* **Our Solution:** While we don't delete data (auditors hate that!), our tool effectively "archives" old versions by deactivating them, solving the noise issue without destroying history.

## 🧠 Concepts: Active vs. Latest

Understanding these flags is critical for a clean dashboard:

| Flag | Dependency-Track Meaning | Impact on Pipeline |
| --- | --- | --- |
| **Active** | Indicates this version is currently deployed/supported. | **Critical.** Only "Active" versions contribute to Portfolio Metrics and trigger Vulnerability Alerts. |
| **Latest** | Indicates this is the most current version of the project. | **Visual & Analytical.** Used as the baseline for "Outdated Component" analysis and marked with a badge in the UI. |

## 📦 Installation

Since this is a single-file Go program, you can run it directly or build a binary.

### Option 1: Download the lastest release binary (Recommended for CI)
**Run the following command to get the latest release binary**:

```bash
     wget https://raw.githubusercontent.com/MedUnes/dtrack-cli/master/latest.sh && \
     chmod +x latest.sh && \
     ./latest.sh && \
      rm ./latest.sh
```

### Option 2: Run directly

```bash
go run main.go -help
```

### Option 3: Build Binary

```bash
go build -o dtrack-cli deploy_sbom.go
./dtrack-cli -help

```

## 🛠 Usage Scenarios

### Scenario 1: The "Golden Path" (CI Pipeline)

**Goal:** Upload a new SBOM, mark it as the **only** active version, and tag it as latest.

Use the `-ci` shortcut flag, which enables `-upload`, `-latest`, and `-clean` simultaneously.

```bash
# Syntax: go run deploy_sbom.go [flags] <API_KEY> <PROJECT_NAME> <VERSION>
dtrack -ci -file ./build/bom.json $DT_API_KEY "My-Web-App" "v1.2.0"

```

**What happens:**

1. **Uploads** `build/bom.json` to project "My-Web-App" version "v1.2.0".
2. **Sets** `v1.2.0` to `Active=True` and `IsLatest=True`.
3. **Iterates** through all other versions (e.g., v1.1.0, v1.0.0) and sets them to `Active=False` / `IsLatest=False`.

### Scenario 2: Read-Only Audit

**Goal:** Check what versions exist and when they were uploaded without making changes.

```bash
dtrack -list $DT_API_KEY "My-Web-App"

```

**Output:**

```text
--- Current Versions ---
VERSION    ACTIVE  LATEST  LAST UPLOAD       UUID
v1.2.0     true    true    2023-10-25 14:30  abc-123...
v1.1.0     false   false   2023-10-20 10:00  def-456...
------------------------

```

### Scenario 3: Maintenance / Hotfix

**Goal:** You uploaded a hotfix for an older version (`v1.0.1`) and want to mark it as Active, but you **do not** want it to become the "Latest" version (because `v2.0` already exists).

Use specific flags instead of `-ci`.

```bash
# Upload and Clean (deactivate others), but do NOT touch the 'Latest' flag
dtrack -upload -clean $DT_API_KEY "My-Web-App" "v1.0.1"

```

### Scenario 4: Manual Cleanup (No Upload)

**Goal:** Your dashboard is cluttered with 50 old versions. You want to keep `v2.0` as the only active one, but you don't have the SBOM file handy to re-upload.

```bash
# Just clean up the metadata
dtrack -clean -latest $DT_API_KEY "My-Web-App" "v2.0"

```

## 🚩 Flag Reference

| Flag | Description | Requirement                           |
| --- | --- |---------------------------------------|
| `-upload` | Uploads the file specified by `-file`. | Requires `VERSION` argument.          |
| `-list` | Displays a table of all versions for this project. |                                       |
| `-latest` | Marks the target version as `isLatest=true` and unsets it for others. | Requires `VERSION` argument.          |
| `-clean` | Marks the target version as `active=true` and **all others** as `active=false`. | Requires `VERSION` argument.          |
| `-ci` | **Recommended.** Shortcut for `-upload -latest -clean`. | Requires `VERSION` argument.          |
| `-url` | Base URL of your Dependency-Track instance. | Default: `https://dtrack.example.com` |
| `-file` | Path to the SBOM file. | Default: `sbom.json`                  |

## ⚠️ Requirements

* **Go 1.21+** (uses `slices` and `cmp` packages).
* **Network:** Access to the Dependency-Track API.
* **Permissions:** The API Key used must have `BOM_UPLOAD` and `PROJECT_CREATION_UPLOAD` permissions.

## 📚 References

* Dependency Track REST API documentation https://docs.dependencytrack.org/integrations/rest-api/
