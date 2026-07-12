//go:build !remote && (linux || freebsd)

package abi

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"go.podman.io/podman/v6/pkg/domain/entities"
	k8sAPI "go.podman.io/podman/v6/pkg/k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

func (ic *ContainerEngine) KubeApply(_ context.Context, body io.Reader, options entities.ApplyOptions) error {
	// Read the yaml file
	content, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	if len(content) == 0 {
		return errors.New("yaml file provided is empty, cannot apply to a cluster")
	}

	// Split the yaml file
	documentList, err := splitMultiDocYAML(content)
	if err != nil {
		return err
	}

	// Sort the kube kinds
	documentList, err = sortKubeKinds(documentList)
	if err != nil {
		return fmt.Errorf("unable to sort kube kinds: %w", err)
	}

	// Get the namespace to deploy the workload to
	namespace := options.Namespace
	if namespace == "" {
		namespace = "default"
	}

	// Parse the given kubeconfig
	kconfig, err := getClusterInfo(options.Kubeconfig)
	if err != nil {
		return err
	}

	// Set up the client to connect to the cluster endpoints
	client, err := setUpClusterClient(kconfig, options)
	if err != nil {
		return err
	}

	for _, document := range documentList {
		kind, err := getKubeKind(document)
		if err != nil {
			return fmt.Errorf("unable to read kube YAML: %w", err)
		}

		switch kind {
		case entities.TypeService:
			url := kconfig.Clusters[0].Cluster.Server + "/api/v1/namespaces/" + namespace + "/services"
			if err := createObject(client, url, document); err != nil {
				return err
			}
		case entities.TypePVC:
			url := kconfig.Clusters[0].Cluster.Server + "/api/v1/namespaces/" + namespace + "/persistentvolumeclaims"
			if err := createObject(client, url, document); err != nil {
				return err
			}
		case entities.TypePod:
			url := kconfig.Clusters[0].Cluster.Server + "/api/v1/namespaces/" + namespace + "/pods"
			if err := createObject(client, url, document); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported Kubernetes kind found: %q", kind)
		}
	}

	return nil
}

// setUpClusterClient sets up the client to use when connecting to the cluster. It sets up the CA Certs and
// client certs and keys based on the information given in the kubeconfig
func setUpClusterClient(kconfig k8sAPI.Config, applyOptions entities.ApplyOptions) (*http.Client, error) {
	var (
		clientCert tls.Certificate
		err        error
	)

	// Load client certificate and key
	// This information will always be in the kubeconfig
	if kconfig.AuthInfos[0].AuthInfo.ClientCertificate != "" && kconfig.AuthInfos[0].AuthInfo.ClientKey != "" {
		clientCert, err = tls.LoadX509KeyPair(kconfig.AuthInfos[0].AuthInfo.ClientCertificate, kconfig.AuthInfos[0].AuthInfo.ClientKey)
		if err != nil {
			return nil, err
		}
	} else if len(kconfig.AuthInfos[0].AuthInfo.ClientCertificateData) > 0 && len(kconfig.AuthInfos[0].AuthInfo.ClientKeyData) > 0 {
		clientCert, err = tls.X509KeyPair(kconfig.AuthInfos[0].AuthInfo.ClientCertificateData, kconfig.AuthInfos[0].AuthInfo.ClientKeyData)
		if err != nil {
			return nil, err
		}
	}

	// Load CA cert
	// The CA cert may not always be in the kubeconfig and could be in a separate file.
	// The CA cert file can be passed on here by setting the --ca-cert-file flag. If that is not set
	// check the kubeconfig to see if it has the CA cert data.
	var caCert []byte
	insecureSkipVerify := false
	caCertFile := applyOptions.CACertFile
	caCertPool := x509.NewCertPool()

	// Be insecure if user sets ca-cert-file flag to insecure
	if strings.ToLower(caCertFile) == "insecure" {
		insecureSkipVerify = true
	} else if caCertFile == "" {
		caCertFile = kconfig.Clusters[0].Cluster.CertificateAuthority
	}

	// Get the caCert data if we are running secure
	if caCertFile != "" && !insecureSkipVerify {
		caCert, err = os.ReadFile(caCertFile)
		if err != nil {
			return nil, err
		}
	} else if len(kconfig.Clusters[0].Cluster.CertificateAuthorityData) > 0 && !insecureSkipVerify {
		caCert = kconfig.Clusters[0].Cluster.CertificateAuthorityData
	}
	if len(caCert) > 0 {
		caCertPool.AppendCertsFromPEM(caCert)
	}

	// Create transport with ca and client certs
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: caCertPool, Certificates: []tls.Certificate{clientCert}, InsecureSkipVerify: insecureSkipVerify},
	}
	return &http.Client{Transport: tr}, nil
}

// createObject connects to the given url and creates the yaml given in objectData
func createObject(client *http.Client, url string, objectData []byte) error {
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(string(objectData)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/yaml")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Log the response body as fatal if we get a non-success status code
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return errors.New(string(body))
	}
	return nil
}

// getClusterInfo returns the kubeconfig in struct form so that the server
// and certificates data can be accessed and used to connect to the k8s cluster
func getClusterInfo(kubeconfig string) (k8sAPI.Config, error) {
	var config k8sAPI.Config

	configData, err := os.ReadFile(kubeconfig)
	if err != nil {
		return config, err
	}

	// Convert yaml kubeconfig to json so we can unmarshal it
	jsonData, err := yaml.YAMLToJSON(configData)
	if err != nil {
		return config, err
	}

	if err := json.Unmarshal(jsonData, &config); err != nil {
		return config, err
	}

	return config, nil
}
