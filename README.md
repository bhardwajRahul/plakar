<div align="center">

<img src="./docs/assets/Plakar_Logo_Simple_Pirmary.png" alt="Plakar: open source backup engine" width="200"/>

# Plakar: open source backup engine

**Encrypted, deduplicated, verifiable, and scalable.**

Back up anything, store anywhere, restore everywhere, with zero-trust encryption and no vendor lock-in.

[![Join our Discord community](https://img.shields.io/badge/Discord-Join%20Us-purple?logo=discord&logoColor=white&style=for-the-badge)](https://discord.gg/A2yvjS6r2C)
[![Subscribe on YouTube](https://img.shields.io/badge/YouTube-Subscribe-red?logo=youtube&logoColor=white&style=for-the-badge)](https://www.youtube.com/@PlakarKorp)
[![Join our Subreddit](https://img.shields.io/badge/Reddit-Join%20r%2Fplakar-orange?logo=reddit&logoColor=white&style=for-the-badge)](https://www.reddit.com/r/plakar/)

[![Go Report Card](https://goreportcard.com/badge/github.com/PlakarKorp/plakar)](https://goreportcard.com/report/github.com/PlakarKorp/plakar)
[![codecov](https://codecov.io/gh/PlakarKorp/plakar/branch/main/graph/badge.svg)](https://codecov.io/gh/PlakarKorp/plakar)

[Deutsch](https://www.readme-i18n.com/PlakarKorp/plakar?lang=de) |
[Español](https://www.readme-i18n.com/PlakarKorp/plakar?lang=es) |
[français](https://www.readme-i18n.com/PlakarKorp/plakar?lang=fr) |
[日本語](https://www.readme-i18n.com/PlakarKorp/plakar?lang=ja) |
[한국어](https://www.readme-i18n.com/PlakarKorp/plakar?lang=ko) |
[Português](https://www.readme-i18n.com/PlakarKorp/plakar?lang=pt) |
[Русский](https://www.readme-i18n.com/PlakarKorp/plakar?lang=ru) |
[中文](https://www.readme-i18n.com/PlakarKorp/plakar?lang=zh)

</div>

## 🧭 Introduction

Plakar is an open source backup engine that goes beyond file-level backups. It captures your data with its full context and turns it into encrypted, deduplicated, and verifiable snapshots you can store anywhere and restore everywhere.

Data and context are stored in [Kloset](https://www.plakar.io/posts/2025-04-29/kloset-the-immutable-data-store/), an open source, immutable data store that makes advanced data protection scenarios possible. Everything is deduplicated, compressed, and encrypted **at the source, before it leaves your perimeter**, so you get high storage efficiency without ever exposing your data or your keys to the storage infrastructure.

Plakar's strengths and capabilities:
- **Secure**: Audited end-to-end encryption for data *and* metadata, with zero-trust by design: keys never leave your environment, and storage providers stay mathematically blind. See our latest [crypto audit report](https://www.plakar.io/posts/2025-02-28/audit-of-plakar-cryptography/).
- **Efficient**: Deduplication and compression happen *before* encryption: high density on fully encrypted data means more restore points and less storage, thanks to Kloset's unmatched [deduplication](https://www.plakar.io/posts/2025-07-11/introducing-go-cdc-chunkers-chunk-and-deduplicate-everything/).
- **Verifiable**: Immutable, content-addressed snapshots with built-in cryptographic integrity checks. Don't assume your backups work. Prove it.
- **Portable, no lock-in**: Backups live in the open Kloset and [ptar](https://www.plakar.io/posts/2025-06-27/it-doesnt-make-sense-to-wrap-modern-data-in-a-1979-format-introducing-.ptar/) formats, readable without Plakar. Connect any source or backend (file systems, object stores, SaaS apps…), restore across storage types and platforms, and distribute copies with the 3-2-1 rule or push/pull/sync strategies.
- **Scalable**: Vertically scalable (very large datasets with limited RAM) and horizontally scalable (high concurrency, multiple backup types in a single Kloset). Engineered for exabytes with a minimal footprint.
- **Instant recovery**: Mount large backups on any device without full restoration, then browse, search, and compare snapshots in the Plakar UI to restore an entire snapshot or just a subset.
- **Effortless and production-safe**: Easy to use, with clean defaults. Plakar automatically adjusts backup speed to protect production workloads and runs lock-free garbage collection without interrupting backups or restores. Start with our [quick start guide](https://www.plakar.io/docs/).
- **Open source and actively maintained**: Open source forever, maintained by [Plakar Korp](https://www.plakar.io), and a member of the Linux Foundation and the CNCF.

Our mission is to set a new, open standard for effortless, secure, zero-trust data resilience.

## 🖥️ Plakar UI

Plakar includes a built-in web-based user interface to **monitor, browse, and restore** your backups with ease.

### 🚀 Launch the UI

You can start the interface from any machine with access to your backups:

```
$ plakar ui
```

### 📂 Snapshot Overview

Quickly list all available snapshots and explore them:

![Snapshot browser](https://www.plakar.io/readme/snapshot-list.png)

### 🔍 Granular Browsing

Navigate the contents of each snapshot to inspect, compare, or selectively restore files:

![Snapshot browser](https://www.plakar.io/readme/snapshot-browser.png)

## 📦 Installing the CLI

### From binaries

Visit https://www.plakar.io/download/

### From source

`plakar` requires Go 1.23.3 or higher,
it may work on older versions but hasn't been tested.

```
go install github.com/PlakarKorp/plakar@latest
```

## 🚀 Quickstart

plakar quickstart: https://www.plakar.io/docs/

A taste of plakar (please follow the quickstart to begin):
```
$ plakar at /var/backups create                             # Create a repository
$ plakar at /var/backups backup /private/etc                # Backup /private/etc
$ plakar at /var/backups ls                                 # List all repository backups
$ plakar at /var/backups restore -to /tmp/restore 9abc3294  # Restore a backup to /tmp/restore
$ plakar at /var/backups ui                                 # Start the UI
$ plakar at /var/backups sync to @s3                        # Synchronise a backup repository to S3

```

## 🗄️ Plakar archive format: ptar

[ptar](https://www.plakar.io/posts/2025-06-27/it-doesnt-make-sense-to-wrap-modern-data-in-a-1979-format-introducing-.ptar/) is Plakar's lightweight, high-performance archive format for secure and efficient backup snapshots.

[Kapsul](https://www.plakar.io/posts/2025-07-07/kapsul-a-tool-to-create-and-manage-deduplicated-compressed-and-encrypted-ptar-vaults/) is a companion tool that lets you run most plakar sub-commands directly on a .ptar archive without extracting it.
It mounts the archive in memory as a read-only Plakar repository, enabling transparent and efficient inspection, restoration, and diffing of snapshots.

For installation, usage examples, and full documentation, see the [Kapsul repository](https://github.com/PlakarKorp/kapsul).

## 🔌 Integrations

Plakar writes to the storage you control and protects every source across your estate. Click a connector for its setup guide:

<div align="center">

**Writes to storage you control**

[![Amazon S3](https://img.shields.io/badge/Amazon%20S3-569A31?logo=amazons3&logoColor=white)](https://www.plakar.io/docs/community/v1.1.0/integrations/s3/)
[![Azure Blob](https://img.shields.io/badge/Azure%20Blob-0078D4?logo=microsoftazure&logoColor=white)](https://www.plakar.io/docs/community/v1.1.0/integrations/azblob/)
[![Google Cloud](https://img.shields.io/badge/Google%20Cloud-4285F4?logo=googlecloud&logoColor=white)](https://www.plakar.io/docs/community/v1.1.0/integrations/gcs/)
[![MinIO](https://img.shields.io/badge/MinIO-C72E49?logo=minio&logoColor=white)](https://www.plakar.io/docs/community/v1.1.0/integrations/s3/)
[![Backblaze B2](https://img.shields.io/badge/Backblaze%20B2-E21E29?logo=backblaze&logoColor=white)](https://www.plakar.io/docs/community/v1.1.0/integrations/s3/)
[![Cloudflare R2](https://img.shields.io/badge/Cloudflare%20R2-F38020?logo=cloudflare&logoColor=white)](https://www.plakar.io/docs/community/v1.1.0/integrations/s3/)
[![NetApp](https://img.shields.io/badge/NetApp-0067C5?logo=netapp&logoColor=white)](https://www.plakar.io/docs/community/v1.1.0/integrations/s3/)
[![Local disk](https://img.shields.io/badge/Local%20disk-555555)](https://www.plakar.io/docs/)

**Protects every source**

[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-4169E1?logo=postgresql&logoColor=white)](https://www.plakar.io/docs/community/v1.1.0/integrations/postgres/)
[![MySQL](https://img.shields.io/badge/MySQL-4479A1?logo=mysql&logoColor=white)](https://www.plakar.io/docs/community/v1.1.0/integrations/mysql/)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-326CE5?logo=kubernetes&logoColor=white)](https://www.plakar.io/docs/community/v1.1.0/integrations/kubernetes/)
[![Docker](https://img.shields.io/badge/Docker-2496ED?logo=docker&logoColor=white)](https://www.plakar.io/docs/community/v1.1.0/integrations/)
[![Proxmox](https://img.shields.io/badge/Proxmox-E57000?logo=proxmox&logoColor=white)](https://www.plakar.io/docs/community/v1.1.0/integrations/proxmox/)

</div>

See the full list in the [integrations docs](https://www.plakar.io/docs/community/v1.1.0/integrations/).

## 🛰️ Plakar Control Plane

Managing backups across a fleet, not just one machine? **[Plakar Control Plane](https://www.plakar.io/docs/control-plane/intro/)** is a self-hosted management platform built on the open source engine: unified inventory across providers, first-class integrations, centralized policies and scheduling (from the UI, resource tags, or as code), and secrets you can delegate to AWS Secrets Manager, HashiCorp Vault, GCP, or Scaleway. Deployed as an appliance on your own infrastructure. Your data never leaves your environment.

A **free plan** (fully functional, no time limit) is available.

[**Download & deploy →**](https://www.plakar.io/download/) · [Control Plane docs](https://www.plakar.io/docs/control-plane/intro/)

## 📚 Documentation

For the latest information, read the documentation at https://www.plakar.io/docs/

## 🤝 Contributing

Plakar is open source and community-driven. Contributions, integrations, and issues are welcome.

- Read our [Contributing guide](CONTRIBUTING.md) to get started.
- Please follow our [Code of Conduct](CODE_OF_CONDUCT.md).
- Found a security issue? See our [Security policy](SECURITY.md) for responsible disclosure.

## 💬 Community

- 🗨️ Join our very active [Discord](https://discord.gg/A2yvjS6r2C)
- 𝕏 Follow us on [X](https://x.com/PlakarKorp)
- 💼 Connect on [LinkedIn](https://www.linkedin.com/company/plakarkorp)
- 🦋 Find us on [Bluesky](https://bsky.app/profile/plakar.bsky.social)
- 📣 Follow our subreddit [r/plakar](https://www.reddit.com/r/plakar/)
- ▶️ Subscribe to our YouTube channel [@PlakarKorp](https://www.youtube.com/@PlakarKorp)

## ⭐ Support Plakar

If Plakar is useful to you, please [give us a star](https://github.com/PlakarKorp/plakar) or leave a review on [SourceForge](https://sourceforge.net/software/product/Plakar/reviews/new) or [G2](https://www.g2.com/products/plakar/review_modalities/new). It really helps the project grow.

## 📄 License

plakar is distributed under the terms of the [ISC License](LICENSE).

## 📰 Releases

See the [Releases page](https://github.com/PlakarKorp/plakar/releases) for the changelog, and the [Plakar blog](https://www.plakar.io/posts/) for release deep-dives.
