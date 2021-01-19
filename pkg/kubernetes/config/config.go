package config

type Config struct {

	// Kubeconfig is the kubeconfig for the cluster containing the bundle secrets
	Kubeconfig string `json:"kubeconfig"`

	// Namespace is the kubernetes namespace in the cluster that will contain the bundle secrets
	Namespace string `json:"kubeconfig"`

}