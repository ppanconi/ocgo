# ocgo

Go project using a project-local Nix development shell.

## Development

Enter the shell manually:

```sh
nix develop
```

Or enable automatic loading with direnv:

```sh
direnv allow
```

Run the project:

```sh
go run .
```

Example:

```sh
go run . -n DSID "https://vpn.example.org/Linux"
```
