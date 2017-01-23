//go:generate go-bindata data/...
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/hashicorp/vault/api"
)

const (
	EnvVaultEndpointURL = "VAULT_ENDPOINT"
)

var (
	listenFlag  = flag.String("listen", ":6789", "address and port to listen")
	versionFlag = flag.Bool("version", false, "display version information")

	// stdoutW and stderrW are for overriding in test.
	stdoutW = os.Stdout
	stderrW = os.Stderr

	// vaultEndpoint is where vault should retrieve creds from
	vaultEndpoint = ""
)

func init() {
	vaultEndpoint = os.Getenv(EnvVaultEndpointURL)
}

func main() {
	flag.Parse()

	// Asking for the version?
	if *versionFlag {
		fmt.Fprintln(stderrW, humanVersion)
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) > 0 {
		fmt.Fprintln(stderrW, "Too many arguments!")
		os.Exit(127)
	}

	// Validate creds
	if vaultEndpoint == "" {
		fmt.Fprintln(stderrW, "Missing VAULT_ENDPOINT!")
		os.Exit(127)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", httpLog(stdoutW, withAppHeaders(index())))
	mux.HandleFunc("/token.json", httpLog(stdoutW, withAppHeaders(acquireToken())))

	// Health endpoint
	mux.HandleFunc("/health", withAppHeaders(httpHealth()))

	srv := &http.Server{Addr: *listenFlag, Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Printf("[ERR] Error starting server: %s", err)
		}
	}()
	log.Printf("Server is listening on %s\n", *listenFlag)

	signalCh := make(chan os.Signal)
	signal.Notify(signalCh, os.Interrupt)

	<-signalCh
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	srv.Shutdown(ctx)

	log.Println("Server is stopped!")
}

func vaultClient() (*api.Client, error) {
	vault, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		return nil, err
	}
	return vault, nil
}

func httpError(w http.ResponseWriter, code int, f string, i ...interface{}) {
	w.WriteHeader(code)
	fmt.Fprintf(w, f, i...)
}

func index() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := Asset("data/index.html")
		if err != nil {
			httpError(w, 500, "%s", err)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf8")
		fmt.Fprintf(w, "%s", data)
	}
}

func acquireToken() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vault, err := vaultClient()
		if err != nil {
			httpError(w, 500, "vault: %s", err)
			return
		}

		secret, err := vault.Auth().Token().Create(&api.TokenCreateRequest{
			Policies:       []string{"crowdsource", "default"},
			NumUses:        5,
			TTL:            "5m",
			ExplicitMaxTTL: "5m",
		})
		if err != nil {
			w.WriteHeader(403)
			fmt.Fprintln(w, fmt.Sprintf("%s", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"endpoint":"%s","token":"%s"}`,
			vaultEndpoint, secret.Auth.ClientToken)
	}
}

func httpHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"status":"ok"}`)
	}
}
