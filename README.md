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

To print the final OpenConnect command instead of starting the VPN:

```sh
go run . --no-c -n DSID "https://vpn.example.org/Linux"
```

This mode does not ask sudo and does not require OpenConnect to be installed, but
it prints the authentication cookie to standard output as part of the command.
