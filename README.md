# tfcopyvars

A CLI tool to manage and copy variables between [HCP Terraform](https://app.terraform.io) workspaces.

## Prerequisites

Set the following environment variables:

```sh
export TFE_TOKEN=<your-api-token>
export TFE_ORG=<your-organization-name>
```

## Build

```sh
go build -o tfcopyvars .
```

## Usage

```
tfcopyvars workspaces [--filter <name>]
tfcopyvars vars --workspace <name>
tfcopyvars copy-vars --from <name> --to <name> [--overwrite] [--copy-sensitive]
```

## Commands

### `workspaces`

List all workspaces in the organization.

```sh
tfcopyvars workspaces
tfcopyvars workspaces --filter staging
```

### `vars`

List all variables for a workspace.

```sh
tfcopyvars vars --workspace my-workspace
```

### `copy-vars`

Copy variables from one workspace to another.

```sh
tfcopyvars copy-vars --from source-workspace --to destination-workspace
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--overwrite` | Overwrite existing variables in the destination |
| `--copy-sensitive` | Copy sensitive variables with an empty value (must be filled in manually) |


## License

MIT — see [LICENSE](LICENSE).

## Authors

- [Puru Tuladhar](https://purutuladhar.com)