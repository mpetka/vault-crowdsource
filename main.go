//go:generate go-bindata data/...
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/hashicorp/vault/api"
)

const (
	EnvVaultEndpointURL = "VAULT_ENDPOINT"

	// vaultPolicyCrowdsource and vaultPolicyDefault are the names of the
	// policies to apply to the token.
	vaultPolicyCrowdsource = "crowdsource"
	vaultPolicyDefault     = "default"

	// vaultNumUses is the total number of uses to allow for the token.
	vaultNumUses = 5

	// vaultTTL is the explicit and implicit maximum lifetime for the generated
	// token.
	vaultTTL = "5m"

	// header types as constants to prevent re-allocations of strings.
	headerContentType         = "Content-Type"
	headerTypeTextHTML        = "text/html; charset=utf8"
	headerTypeApplicationJSON = "application/json"

	// respStatusOK is the response for a successful status.
	respStatusOK = `{"status": "ok"}`
)

var (
	// vaultPolicies is the list of policies to apply to the generated token.
	vaultPolicies = []string{vaultPolicyCrowdsource, vaultPolicyDefault}

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

	// Setup the API client
	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		fmt.Fprintln(stderrW, "Failed to setup API client: "+err.Error())
		os.Exit(127)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", httpLog(stdoutW, withAppHeaders(index())))
	mux.HandleFunc("/favicon.ico", httpLog(stdoutW, withAppHeaders(favicon())))
	mux.HandleFunc("/token.json", httpLog(stdoutW, withAppHeaders(acquireToken(client))))
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

func httpError(w http.ResponseWriter, code int, msg string) {
	w.WriteHeader(code)
	io.WriteString(w, msg)
}

func index() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := Asset("data/index.html")
		if err != nil {
			httpError(w, http.StatusNotFound, err.Error())
			return
		}

		w.Header().Set(headerContentType, headerTypeTextHTML)
		w.Write(data)
	}
}

func favicon() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := Asset("data/favicon.ico")
		if err != nil {
			httpError(w, http.StatusNotFound, err.Error())
			return
		}

		w.Header().Set(headerContentType, headerTypeTextHTML)
		w.Write(data)
	}
}

func acquireToken(client *api.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		secret, err := client.Auth().Token().Create(&api.TokenCreateRequest{
			Policies:       vaultPolicies,
			NumUses:        vaultNumUses,
			TTL:            vaultTTL,
			ExplicitMaxTTL: vaultTTL,
		})
		if err != nil {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintln(w, err.Error())
			return
		}

		w.Header().Set(headerContentType, headerTypeApplicationJSON)
		fmt.Fprintf(w, `{"endpoint":"%s","token":"%s"}`,
			vaultEndpoint, secret.Auth.ClientToken)
	}
}

func httpHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, respStatusOK)
	}
}
