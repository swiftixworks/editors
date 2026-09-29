# Swiftix editors

This repository provides terminal text editors for Swiftix. It follows the same
source, deterministic builder, native `.pkg`, and execution-test pattern as the
sibling `coreutils` and `sysutils` repositories.

The `editors_0.1.0.pkg` package contains one Swiftix Go command:

- `nano [FILE]` is a nano-style full-screen editor with a title bar, a status
  line, and a two-row shortcut bar.

## Keys

| Key | Action |
| --- | --- |
| Arrows, Home, End, PageUp, PageDown | Move (also `^P` `^N` `^B` `^F`, `^A` `^E`, `^Y` `^V`) |
| Backspace, Delete (`^D`) | Remove the character before or under the cursor |
| `^O` | Write the file, asking for a name |
| `^X` | Exit, offering to save a modified buffer |
| `^W` | Search forward, wrapping around; an empty answer repeats the last search |
| `^K` / `^U` | Cut the current line (consecutive cuts accumulate) / paste |
| `^C` | Report the cursor position |
| `^G` | Show help |
| `^L` | Redraw the screen |

## Compatibility and honest limits

The name describes a familiar workflow, not a GNU nano implementation:

- one buffer; no undo, replace, file insertion, syntax highlighting, mouse, or
  Meta (`Esc` + key) bindings;
- files up to 24 KiB, so each load, search, and save fits one Swiftix Go
  instruction slice (about one million VM instructions);
- every character occupies one terminal column, matching the Swiftix terminal
  renderer; tabs expand to multiples of eight and control bytes show as `^X`;
- saving writes the file in place and ends it with a newline;
- Swiftix has no `SIGWINCH`, so nano re-reads the window size before each
  frame; and
- messages outside the editor, such as usage errors, go to standard output.

nano needs the `swiftix/userland` terminal ABI (`ReadStdin`, `WriteFile`,
`SetRawMode`, `WindowSize`), string escapes, and resumable file-backed
execution, so it requires Swiftix 0.12.0 or later.

## Building and verification

Building requires Swift 6.3 or later and this sibling layout:

```text
swiftixworks/
├── Swiftix/
└── editors/
```

Run:

```sh
swift run EditorsPackageBuilder
swift run EditorsPackageBuilder --check
swift test -Xswiftc -warnings-as-errors
```

The builder compiles every Go file in `Commands/<command>/` to `swiftix/svm64`,
installs each image at `/usr/bin/<command>`, and creates
`Artifacts/editors_0.1.0.pkg` with the native Swiftix package codec. `--check`
performs a byte-for-byte reproducibility check.

## License

Swiftix editors is available under the [MIT License](LICENSE).
