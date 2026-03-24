# CLI Reference

## Commands

### `kiln init`

Create `kiln.yaml` interactively. Asks 4 questions: database driver, DSN, output directory, and API base path.

```bash
kiln init
```

### `kiln generate`

Generate your API from the database schema. Reads `kiln.yaml`, introspects the database via bob, and writes generated code.

```bash
kiln generate
kiln generate --table users    # only regenerate one table
kiln generate --no-bob         # skip schema reading, use existing models
kiln generate --dry-run        # preview changes without writing files
kiln generate --force          # overwrite files even if manually edited
```

| Flag | Effect |
|------|--------|
| `--table X` | Only regenerate a specific table |
| `--no-bob` | Skip schema introspection, use existing bob models |
| `--dry-run` | Print what would be generated without writing |
| `--force` | Overwrite user-modified files |

### `kiln diff`

Preview what would be generated without writing any files.

```bash
kiln diff
```

### `kiln introspect`

Print the parsed schema IR (useful for debugging).

```bash
kiln introspect
kiln introspect --format json
```

### `kiln version`

Print the kiln version.

```bash
kiln version
```

## Global Flags

| Flag | Effect |
|------|--------|
| `--config path/to/kiln.yaml` | Use a custom config file (default: `kiln.yaml`) |
