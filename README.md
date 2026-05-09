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

Run ocgo as your normal desktop user, not with sudo. The browser login needs your
desktop session; ocgo asks for sudo before opening the browser, then reuses that
authorization when OpenConnect is ready to start.

After browser login, ocgo starts OpenConnect with the captured cookie:

```sh
sudo openconnect --protocol=nc --cookie-on-stdin <server-url>
```
