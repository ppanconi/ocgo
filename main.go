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
}

func main() {
	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		log.Fatal(err)
	}

	cookie, err := extractCookie(context.Background(), cfg)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print(cookie)
}

func parseArgs(args []string) (config, error) {
	fs := flag.NewFlagSet("ocgo", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	cfg := config{}
	fs.StringVar(&cfg.cookieName, "n", "DSID", "name of the cookie")
	fs.StringVar(&cfg.cookieName, "name", "DSID", "name of the cookie")
	fs.StringVar(&cfg.clickLink, "click-link", defaultClickLink, "CSS selector of a link to click after each page load, if present")
	fs.DurationVar(&cfg.timeout, "timeout", 10*time.Minute, "maximum time to wait for the cookie")

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

	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserDataDir(userDataDir),
		chromedp.Flag("headless", false),
		chromedp.Flag("window-size", "460,600"),
		chromedp.Flag("app", cfg.server),
		chromedp.Flag("enable-automation", false),
		// chromedp.Flag("disable-gpu", false),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
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

func findChromePath() string {
	for _, name := range []string{"google-chrome-stable", "google-chrome", "chromium", "chromium-browser", "chrome"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}
