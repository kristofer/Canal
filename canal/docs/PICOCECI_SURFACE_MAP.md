# Canal picoceci Surface Map

This document defines the Canal object-level surface intended for picoceci code.

## Global Objects

- `Canal`
- `FS`

## `Canal` Methods

- `openChannel: #fs` → returns the `FS` object
- `openChannel: #fsRead` → returns the `FS` object
- `openChannel: #fsWrite` → returns the `FS` object
- `openChannel: #fsReadWrite` → returns the `FS` object
- `printString` → returns `"Canal"`

Compatibility alias:

- `capability:` remains as an alias for `openChannel:` during migration.

## `FS` Methods

- `list: <String|Symbol path>` → `Array[String]`
- `readFile: <String|Symbol path>` → `String`
- `exists: <String|Symbol path>` → `Bool`
- `writeFile: <String|Symbol path> data: <String|ByteArray>` → `Nil`
- `printString` → `"CanalFS(service:fs)"`

## Notes

- Service transport is typed-channel oriented (`fs`, `wifi`, `tls` service IDs in `channel` package).
- On WiFi-domain interpreter paths, FS requests now go through `stdlib/fs` instead of domain-local capability shim calls.
