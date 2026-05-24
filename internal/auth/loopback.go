package auth

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"time"
)

func StartLoopback(ctx context.Context, redirectURI string, buildURL func() string, expectedState string) (string, error) {
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	server := &http.Server{
		Addr: "localhost:8080",
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		if query.Get("error") != "" {
			msg := query.Get("error_description")
			if msg == "" {
				msg = query.Get("error")
			}

			errChan <- fmt.Errorf(msg)
			_, _ = w.Write([]byte("<html><body><h2>Login failed</h2><p>You can close this window.</p></body></html>"))
			return
		}

		code := query.Get("code")
		state := query.Get("state")

		if code == "" {
			errChan <- fmt.Errorf("missing code")
			_, _ = w.Write([]byte("<html><body><h2>Missing code</h2><p>You can close this window.</p></body></html>"))
			return
		}

		if state != expectedState {
			errChan <- fmt.Errorf("invalid state")
			_, _ = w.Write([]byte("<html><body><h2>Invalid state</h2><p>You can close this window.</p></body></html>"))
			return
		}

		_, _ = w.Write([]byte("<html><body style='font-family:Arial;background:#111;color:#eee'><h2>GoLauncher login complete</h2><p>You can close this window.</p></body></html>"))

		codeChan <- code

		go func() {
			time.Sleep(500 * time.Millisecond)
			_ = server.Shutdown(context.Background())
		}()
	})

	server.Handler = mux

	go func() {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	authURL := buildURL()

	if err := openBrowser(authURL); err != nil {
		_ = server.Shutdown(context.Background())
		return "", err
	}

	select {
	case code := <-codeChan:
		return code, nil
	case err := <-errChan:
		_ = server.Shutdown(context.Background())
		return "", err
	case <-ctx.Done():
		_ = server.Shutdown(context.Background())
		return "", ctx.Err()
	case <-time.After(3 * time.Minute):
		_ = server.Shutdown(context.Background())
		return "", fmt.Errorf("login timeout")
	}
}

func openBrowser(target string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", target).Start()
	case "darwin":
		return exec.Command("open", target).Start()
	default:
		return exec.Command("xdg-open", target).Start()
	}
}
