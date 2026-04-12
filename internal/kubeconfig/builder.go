package kubeconfig

import (
	"fmt"

	clientcmd "k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type BuildParams struct {
	Username      string
	ClusterName   string
	ClusterServer string
	ClusterCA     []byte
	ClientCert    []byte
	ClientKey     []byte
}

// Build assembles a kubeconfig YAML from the provided parameters.
func Build(p BuildParams) ([]byte, error) {
	if p.ClusterServer == "" {
		return nil, fmt.Errorf("cluster server URL is required")
	}

	cfg := clientcmdapi.NewConfig()

	cfg.Clusters[p.ClusterName] = &clientcmdapi.Cluster{
		Server:                   p.ClusterServer,
		CertificateAuthorityData: p.ClusterCA,
	}

	cfg.AuthInfos[p.Username] = &clientcmdapi.AuthInfo{
		ClientCertificateData: p.ClientCert,
		ClientKeyData:         p.ClientKey,
	}

	contextName := p.Username + "@" + p.ClusterName
	cfg.Contexts[contextName] = &clientcmdapi.Context{
		Cluster:  p.ClusterName,
		AuthInfo: p.Username,
	}

	cfg.CurrentContext = contextName

	data, err := clientcmd.Write(*cfg)
	if err != nil {
		return nil, fmt.Errorf("serialize kubeconfig: %w", err)
	}
	return data, nil
}
