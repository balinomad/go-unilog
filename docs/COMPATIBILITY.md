# Compatibility

## Go Version Support Policy

`unilog` follows the two most recent major versions of Go, which are **Go 1.24** and **Go 1.25** at the time of writing.

## Versioning Policy

`unilog` follows the **semantic versioning rules** as described by [SemVer.org](https://semver.org/).

This means that the version number of the library is composed of three parts: `MAJOR.MINOR.PATCH`.
- The `MAJOR` version is incremented when we make breaking changes to the public API.
- The `MINOR` version is incremented when we add new functionality in a backwards-compatible manner.
- The `PATCH` version is incremented when we make bug fixes or other non-functional changes.

By following these rules, we ensure that users of the library can easily determine whether a new version will break their code or not.

## Deprecation Policy

Features deprecated for 2 minor versions before removal. Breaking changes only on major versions.

## Adapter Version Policy

Adapters share major version with core.
