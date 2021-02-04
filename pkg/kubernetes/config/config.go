package config

type Config struct {

	// Kubeconfig is the kubeconfig for the cluster containing the bundle secrets, if set this will be used instead of KUBECONFIG or  ~/.kube/config
	Kubeconfig string `json:"kubeconfig"`

	// Namespace is the kubernetes namespace in the cluster that will contain the bundle secrets
	Namespace string `json:"namespace"`
}
