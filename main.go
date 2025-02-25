package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/kelseyhightower/envconfig"
	"github.com/openshift/assisted-image-service/internal/handlers"
	"github.com/openshift/assisted-image-service/pkg/imagestore"
	"github.com/openshift/assisted-image-service/pkg/isoeditor"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

var Options struct {
	AssistedServiceScheme string `envconfig:"ASSISTED_SERVICE_SCHEME"`
	AssistedServiceHost   string `envconfig:"ASSISTED_SERVICE_HOST"`
	DataDir               string `envconfig:"DATA_DIR"`
	HTTPSKeyFile          string `envconfig:"HTTPS_KEY_FILE"`
	HTTPSCertFile         string `envconfig:"HTTPS_CERT_FILE"`
	HTTPSCAFile           string `envconfig:"HTTPS_CA_FILE"`
	ListenPort            string `envconfig:"LISTEN_PORT" default:"8080"`
	RequestAuthType       string `envconfig:"REQUEST_AUTH_TYPE"`
	MaxConcurrentRequests int64  `envconfig:"MAX_CONCURRENT_REQUESTS" default:"400"`
	RHCOSVersions         string `envconfig:"RHCOS_VERSIONS"`
}

func main() {
	log.SetReportCaller(true)
	log.SetFormatter(&log.JSONFormatter{})
	err := envconfig.Process("cluster-image", &Options)
	if err != nil {
		log.Fatalf("Failed to process config: %v\n", err)
	}

	var versions []map[string]string
	if Options.RHCOSVersions == "" {
		versions = imagestore.DefaultVersions
	} else {
		err = json.Unmarshal([]byte(Options.RHCOSVersions), &versions)
		if err != nil {
			log.Fatalf("Failed to unmarshal versions: %v\n", err)
		}
	}

	is, err := imagestore.NewImageStore(isoeditor.NewEditor(Options.DataDir), Options.DataDir, versions)
	if err != nil {
		log.Fatalf("Failed to create image store: %v\n", err)
	}
	err = is.Populate(context.Background())
	if err != nil {
		log.Fatalf("Failed to populate image store: %v\n", err)
	}

	reg := prometheus.NewRegistry()
	http.Handle("/images/", handlers.NewImageHandler(is, reg, Options.AssistedServiceScheme, Options.AssistedServiceHost, Options.RequestAuthType, Options.HTTPSCAFile, Options.MaxConcurrentRequests))
	http.Handle("/health", handlers.NewHealthHandler())
	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	log.Info("Starting http handler...")
	address := fmt.Sprintf(":%s", Options.ListenPort)
	if Options.HTTPSKeyFile != "" && Options.HTTPSCertFile != "" {
		log.Fatal(http.ListenAndServeTLS(address, Options.HTTPSCertFile, Options.HTTPSKeyFile, nil))
	} else {
		log.Fatal(http.ListenAndServe(address, nil))
	}
}
