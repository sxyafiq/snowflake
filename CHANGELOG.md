# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `SequenceResolver` interface and `NewLocalResolver` to make sequence-number
  allocation pluggable, allowing a fleet of generators to share a sequence space
  (e.g. via Redis) instead of requiring a unique worker ID per node.
- `ClockGuard` interface and `FileClockGuard` (stdlib, atomic file writes) to
  protect against duplicate IDs when the wall clock regresses across a process
  restart. Configured via `Config.ClockGuard`, `ClockGuardInterval`, and
  `ClockGuardMaxWait`; persists a high-water timestamp ahead of the clock and
  waits or refuses to start on regression.
- `WorkerLease` interface and `FileWorkerLease` (OS `flock`, unix-family) to
  prevent two live processes from sharing a worker ID. Configured via
  `Config.WorkerLease`; `NewWithConfig` returns `ErrWorkerLeaseHeld` when the
  identity is already held.
- `Generator.Close` to release the worker-ID lease on shutdown (no-op when no
  lease is configured; safe to call multiple times).
- `ErrWorkerLeaseHeld` error.

---

## [1.0.0] - 2025-10-10

### Added
- Initial release of Snowflake ID generator
- Core ID generation with monotonic clock support
- 11 encoding formats (Base2, Base32, Base36, Base58, Base62, Base64, Base64URL, Hex)
- Context support for graceful cancellation
- Built-in metrics and observability
- Database integration (sql.Scanner, driver.Valuer)
- JSON/XML/YAML marshaling support
- Sharding and partitioning methods
- Comprehensive test coverage
- Benchmark suite
- CI/CD workflows
- Complete project documentation and templates
- Development tooling (Makefile, linting config)

---

<!--
## Template for future releases

## [X.Y.Z] - YYYY-MM-DD

### Added
- New features

### Changed
- Changes in existing functionality

### Deprecated
- Soon-to-be removed features

### Removed
- Removed features

### Fixed
- Bug fixes

### Security
- Security fixes

-->

[Unreleased]: https://github.com/sxyafiq/snowflake/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/sxyafiq/snowflake/releases/tag/v1.0.0
