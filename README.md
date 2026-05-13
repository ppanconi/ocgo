# ocgo

`ocgo` helps start VPN sessions for gateways compatible with the Pulse Secure /
Ivanti Connect Secure web authentication flow supported by OpenConnect.

It opens the VPN web login in Chrome or Chromium, waits until the VPN cookie is
issued, then either starts OpenConnect automatically or prints the exact
OpenConnect command to run manually.

## Why

Some VPN deployments authenticate users through a browser flow instead of a
terminal username/password prompt. `ocgo` bridges that flow:

1. Open the web authentication page in a temporary Chrome/Chromium profile.
2. Let the user complete SSO, MFA, or SAML login normally.
3. Extract the VPN session cookie, `DSID` by default.
4. Start OpenConnect with the Pulse protocol, or print the command for manual
   execution.

## Requirements

`ocgo` is compiled and tested on macOS and most common Linux distributions.
Runtime requirements:

- Chrome or Chromium available in `PATH`
- OpenConnect available in `PATH` for automatic connection mode

`ocgo` searches for one of these browser commands:

- `google-chrome-stable`
- `google-chrome`
- `chromium`
- `chromium-browser`
- `chrome`

Install OpenConnect with your platform package manager:

```sh
# macOS (Homebrew)
brew install openconnect

# Debian / Ubuntu
sudo apt install openconnect

# Fedora / RHEL / CentOS
sudo dnf install openconnect

# Arch / Manjaro
sudo pacman -S openconnect
```

## Install

Download the latest prebuilt release archive for your platform from the GitHub
[Releases](https://github.com/ppanconi/ocgo/releases) page.

Released archives are provided for:

- Linux amd64
- Linux arm64
- macOS amd64
- macOS arm64

Each release includes:

- `ocgo` binary
- `README.md`
- `LICENSE`
- `.sha256` checksum file alongside the archive

Typical install steps:

```sh
# 1. Download both the archive and its matching .sha256 file from GitHub Releases
# 2. Verify the checksum if desired
shasum -a 256 -c ocgo_v1.0.1_linux_amd64.tar.gz.sha256

# 3. Extract the archive
tar -xzf ocgo_v1.0.1_linux_amd64.tar.gz

# 4. Install the binary somewhere in PATH
sudo install -m 0755 ocgo_v1.0.1_linux_amd64/ocgo /usr/local/bin/ocgo
```

The checksum command expects the archive and `.sha256` file to be in the same
directory after download.

Confirm the installation:

```sh
ocgo -h
```

Note: On MacOS probably you need to confirm the application in System Settings/Privacy & Security

### Nix / NixOS

This repository is also a Nix flake. Run it directly:

```sh
nix run github:ppanconi/ocgo -- -h
```

Run a specific tagged release:

```sh
nix run github:ppanconi/ocgo/v1.0.1 -- -h
```

On Linux, the flake package wraps `ocgo` with Nix-provided `openconnect` and
`chromium` in `PATH`. On macOS, it wraps `ocgo` with Nix-provided `openconnect`
and expects Chrome to be installed normally on the system.

Install it into a user profile:

```sh
nix profile install github:ppanconi/ocgo
```

Or install a specific tagged release:

```sh
nix profile install github:ppanconi/ocgo/v1.0.1
```

Use it from a NixOS configuration:

```nix
{
  inputs.ocgo.url = "github:ppanconi/ocgo/v1.0.1";

  outputs = { nixpkgs, ocgo, ... }: {
    nixosConfigurations.my-host = nixpkgs.lib.nixosSystem {
      system = "x86_64-linux";
      modules = [
        {
          environment.systemPackages = [
            ocgo.packages.x86_64-linux.default
          ];
        }
      ];
    };
  };
}
```

## Usage

Run `ocgo` as your normal desktop user, not with `sudo`.

In the default automatic mode, `ocgo` still needs administrator privileges to
start OpenConnect. It asks for sudo itself before opening the browser, then
reuses that authorization when it starts the VPN connection.

```sh
ocgo [options] <server-url>
[sudo] password for your-user:
```

Example:

```sh
ocgo https://vpn.example.org/saml
```

Important: do not run this:

```sh
sudo ocgo https://vpn.example.org/saml
```

Chrome/Chromium needs access to your desktop session. Running the whole program
as root can break browser startup or browser profile access on both Linux and
macOS.

If you prefer not to give `ocgo` sudo privileges, use the `--no-c` option and
then run the generated OpenConnect command yourself.

```sh
ocgo --no-c https://vpn.example.org/saml

Authorization succeeded.

To start the VPN session, run:

  sudo openconnect --protocol=nc --cookie DSID=... https://vpn.example.org/saml
```

Flow:

1. Do not ask for sudo.
2. Do not check for `openconnect`.
3. Open Chrome/Chromium for web authentication.
4. Capture the VPN cookie.
5. Print a ready-to-run command, then exit.

## Options

```text
-n string
    name of the cookie (default "DSID")

-name string
    name of the cookie (default "DSID")

-click-link string
    CSS selector of a link to click after each page load, if present
    (default "#continue > a:nth-child(1)")

-timeout duration
    maximum time to wait for the cookie (default 10m0s)

-no-c
    print the OpenConnect command after login instead of asking sudo and
    starting the VPN
```

## Examples

Connect using the default `DSID` cookie:

```sh
ocgo https://vpn.example.org/saml
```

Connect using a custom cookie name:

```sh
ocgo -n MYCOOKIE https://vpn.example.org/saml
```

Print the command without starting OpenConnect:

```sh
ocgo --no-c https://vpn.example.org/saml
```

Disable the automatic link click:

```sh
ocgo -click-link "" https://vpn.example.org/saml
```

Wait up to 15 minutes for the browser login:

```sh
ocgo -timeout 15m https://vpn.example.org/saml
```

## Build

With Go installed:

```sh
go build -o ocgo .
```

For a cgo-disabled portable binary:

```sh
CGO_ENABLED=0 go build -o ocgo-portable .
```

Using the included Nix development shell:

```sh
nix develop
go build -o ocgo .
```

## Release

Create a release from a clean working tree:

```sh
scripts/release.sh v0.2.0
```

The release script updates README examples, commits that change if needed,
creates the tag, and pushes the current branch plus the tag. The release
workflow also checks that the README examples match the pushed tag.

Run tests:

```sh
go test ./...
```

## Development Environment

The repository includes a Nix flake with:

- Go
- gopls
- golangci-lint
- Chromium
- OpenConnect

Enter the shell manually:

```sh
nix develop
```

Or enable automatic loading with direnv:

```sh
direnv allow
```

## Security Notes

- `ocgo` creates a temporary Chrome/Chromium profile and removes it when the run
  finishes.
- Chrome password saving is disabled for the temporary profile.
- In automatic mode, the VPN cookie is passed to OpenConnect through standard
  input.
- In `--no-c` mode, the authentication cookie is printed to standard output as
  part of the generated command. Treat that output as sensitive.
- Do not paste logs or generated commands into public issue reports without
  removing cookies and private VPN hostnames.

## Scope

`ocgo` is focused on gateways compatible with the Pulse Secure / Ivanti Connect
Secure flow supported by OpenConnect's `nc` protocol.

It does not implement a VPN client itself. OpenConnect performs the VPN
connection; `ocgo` only handles the browser authentication step and passes the
resulting cookie to OpenConnect.

## License

This project is released under the MIT License. See [LICENSE](LICENSE).
