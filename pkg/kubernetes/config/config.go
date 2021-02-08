package config

type Config struct {

	// Namespace is the kubernetes namespace in the cluster that will contain the bundle secrets, if running in Kubernetes this value can be excluded and the service account namespace of the pod runing the process will be used.
	Namespace string `json:"namespace"`
}
