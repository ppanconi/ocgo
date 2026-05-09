package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/storage"
	"github.com/chromedp/chromedp"
)

const defaultClickLink = "#continue > a:nth-child(1)"

type config struct {
	server     string
	cookieName string
	clickLink  string
	timeout    time.Duration
	noConnect  bool
}

func main() {
	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		log.Fatal(err)
	}

	if !cfg.noConnect {
		if err := ensureOpenConnectAvailable(); err != nil {
			log.Fatal(err)
		}
		if err := authorizeVPN(); err != nil {
			log.Fatal(err)
		}
	}

	cookie, err := extractCookie(context.Background(), cfg)
	if err != nil {
		log.Fatal(err)
	}

	if cfg.noConnect {
		printOpenConnectCommand(cfg.server, cfg.cookieName, cookie)
		return
	}

	if err := startVPN(cfg.server, cfg.cookieName, cookie); err != nil {
		log.Fatal(err)
	}
}

func parseArgs(args []string) (config, error) {
	fs := flag.NewFlagSet("ocgo", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	cfg := config{}
	fs.StringVar(&cfg.cookieName, "n", "DSID", "name of the cookie")
	fs.StringVar(&cfg.cookieName, "name", "DSID", "name of the cookie")
	fs.StringVar(&cfg.clickLink, "click-link", defaultClickLink, "CSS selector of a link to click after each page load, if present")
	fs.DurationVar(&cfg.timeout, "timeout", 10*time.Minute, "maximum time to wait for the cookie")
	fs.BoolVar(&cfg.noConnect, "no-c", false, "print the OpenConnect command after login instead of asking sudo and starting the VPN")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s [options] <server-url>\n\n", fs.Name())
		fmt.Fprintln(fs.Output(), "Open SSO/SAML page to retrieve an authentication cookie for Pulse Connect Secure VPN.")
		fmt.Fprintln(fs.Output(), "\nOptions:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return config{}, err
	}

	if fs.NArg() != 1 {
		fs.Usage()
		return config{}, errors.New("server URL is required")
	}

	cfg.server = fs.Arg(0)
	return cfg, nil
}

func extractCookie(parent context.Context, cfg config) (string, error) {
	if cfg.timeout <= 0 {
		cfg.timeout = 10 * time.Minute
	}

	userDataDir, err := os.MkdirTemp("", "ocgo-chrome-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(userDataDir)
	if err := disableChromePasswordManager(userDataDir); err != nil {
		return "", err
	}

	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserDataDir(userDataDir),
		chromedp.Flag("headless", false),
		chromedp.Flag("window-size", "460,600"),
		chromedp.Flag("app", cfg.server),
		chromedp.Flag("enable-automation", false),
		// chromedp.Flag("disable-gpu", false),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
		chromedp.Flag("disable-save-password-bubble", true),
		chromedp.Flag("disable-features", "PasswordManagerOnboarding,PasswordManagerAccountStorage,PasswordLeakDetection"),
	)
	if chromePath := findChromePath(); chromePath != "" {
		allocOpts = append(allocOpts, chromedp.ExecPath(chromePath))
	}

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(parent, allocOpts...)
	defer cancelAlloc()

	ctx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	ctx, cancelTimeout := context.WithTimeout(ctx, cfg.timeout)
	defer cancelTimeout()

	if err := chromedp.Run(ctx, network.Enable(), chromedp.Navigate(cfg.server)); err != nil {
		return "", err
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("cookie %q was not found before timeout: %w", cfg.cookieName, ctx.Err())
		case <-ticker.C:
			if cfg.clickLink != "" {
				_ = clickIfPresent(ctx, cfg.clickLink)
			}

			value, found, err := readCookie(ctx, cfg.cookieName)
			if err != nil {
				return "", err
			}
			if found {
				return value, nil
			}
		}
	}
}

func clickIfPresent(ctx context.Context, selector string) error {
	var clicked bool
	selectorJSON, err := json.Marshal(selector)
	if err != nil {
		return err
	}

	return chromedp.Run(ctx, chromedp.EvaluateAsDevTools(
		fmt.Sprintf(`(() => {
			const selector = %s;
			const link = document.querySelector(selector);
			if (link && typeof link.click === "function" && link.dataset.ocgoClicked !== "true") {
				link.dataset.ocgoClicked = "true";
				link.click();
				return true;
			}
			return false;
		})()`, selectorJSON),
		&clicked,
	))
}

func readCookie(ctx context.Context, name string) (string, bool, error) {
	var cookies []*network.Cookie
	err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		cookies, err = storage.GetCookies().Do(ctx)
		return err
	}))
	if err != nil {
		return "", false, err
	}

	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie.Value, true, nil
		}
	}

	return "", false, nil
}

func startVPN(server, cookieName, cookieValue string) error {
	cmd := openConnectCommand(server)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(stdin, "%s=%s", cookieName, cookieValue); err != nil {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		return err
	}
	if err := stdin.Close(); err != nil {
		_ = cmd.Process.Kill()
		return err
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("VPN disconnected: %w", err)
	}
	return nil
}

func printOpenConnectCommand(server, cookieName, cookieValue string) {
	fmt.Printf("\nAuthorization succeeded.\n\nTo start the VPN session, run:\n\n  %s\n\n",
		shellCommand([]string{
			"sudo",
			"openconnect",
			"--protocol=nc",
			"--cookie",
			fmt.Sprintf("%s=%s", cookieName, cookieValue),
			server,
		}),
	)
}

func shellCommand(args []string) string {
	command := ""
	for i, arg := range args {
		if i > 0 {
			command += " "
		}
		command += shellQuote(arg)
	}
	return command
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}

	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '_' || r == '-' || r == '.' || r == '/' || r == ':' || r == '=' {
			continue
		}
		return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
	}
	return s
}

func authorizeVPN() error {
	if os.Geteuid() == 0 {
		return nil
	}

	cmd := exec.Command("sudo", "-v")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sudo authorization failed: %w", err)
	}
	return nil
}

func ensureOpenConnectAvailable() error {
	if _, err := exec.LookPath("openconnect"); err == nil {
		return nil
	}

	return fmt.Errorf("openconnect command was not found.\n\nInstall OpenConnect for your operating system, then run ocgo again:\n\n%s",
		openConnectInstallHint(),
	)
}

func openConnectInstallHint() string {
	osRelease, err := readOSRelease("/etc/os-release")
	if err != nil {
		return genericOpenConnectInstallHint()
	}

	ids := strings.Join([]string{osRelease["ID"], osRelease["ID_LIKE"]}, " ")
	switch {
	case containsAnyID(ids, "debian", "ubuntu"):
		return "  sudo apt install openconnect"
	case containsAnyID(ids, "fedora", "rhel", "centos"):
		return "  sudo dnf install openconnect"
	case containsAnyID(ids, "arch", "manjaro"):
		return "  sudo pacman -S openconnect"
	default:
		return genericOpenConnectInstallHint()
	}
}

func genericOpenConnectInstallHint() string {
	return strings.Join([]string{
		"  Debian/Ubuntu: sudo apt install openconnect",
		"  Fedora/RHEL:   sudo dnf install openconnect",
		"  Arch/Manjaro:  sudo pacman -S openconnect",
	}, "\n")
}

func readOSRelease(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	values := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[key] = strings.Trim(strings.TrimSpace(value), `"`)
	}
	return values, nil
}

func containsAnyID(ids string, candidates ...string) bool {
	fields := strings.Fields(strings.ToLower(ids))
	for _, field := range fields {
		for _, candidate := range candidates {
			if field == candidate {
				return true
			}
		}
	}
	return false
}

func openConnectCommand(server string) *exec.Cmd {
	args := []string{
		"--protocol=nc",
		"--cookie-on-stdin",
		server,
	}
	if os.Geteuid() == 0 {
		return exec.Command("openconnect", args...)
	}
	return exec.Command("sudo", append([]string{"-n", "openconnect"}, args...)...)
}

func disableChromePasswordManager(userDataDir string) error {
	defaultProfileDir := filepath.Join(userDataDir, "Default")
	if err := os.MkdirAll(defaultProfileDir, 0o700); err != nil {
		return err
	}

	preferences := map[string]any{
		"credentials_enable_service":    false,
		"credentials_enable_autosignin": false,
		"autofill": map[string]any{
			"credit_card_enabled": false,
			"profile_enabled":     false,
		},
		"profile": map[string]any{
			"password_manager_enabled":        false,
			"password_manager_leak_detection": false,
		},
	}

	data, err := json.Marshal(preferences)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(defaultProfileDir, "Preferences"), data, 0o600); err != nil {
		return err
	}

	policyDir := filepath.Join(userDataDir, "policies", "managed")
	if err := os.MkdirAll(policyDir, 0o700); err != nil {
		return err
	}
	policy := map[string]any{
		"PasswordManagerEnabled": false,
	}
	data, err = json.Marshal(policy)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(policyDir, "ocgo.json"), data, 0o600)
}

func findChromePath() string {
	for _, name := range []string{"google-chrome-stable", "google-chrome", "chromium", "chromium-browser", "chrome"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}
