package storage

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"get.porter.sh/plugin/kubernetes/pkg/kubernetes/config"
	k8s "get.porter.sh/plugin/kubernetes/pkg/kubernetes/helper"
	"github.com/cnabio/cnab-go/utils/crud"
	"github.com/hashicorp/go-hclog"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

var _ crud.Store = &Store{}

const (
	SecretSourceType = "secret"
	SecretDataKey    = "credential"
)

// Store implements the backing store for claims as kubernetes secrets.
type Store struct {
	logger    hclog.Logger
	config    config.Config
	clientSet *kubernetes.Clientset
}

func NewStore(cfg config.Config, l hclog.Logger) *Store {
	return &Store{
		config: cfg,
		logger: l,
	}
}

func (s *Store) init() error {

	if s.clientSet != nil {
		return nil
	}

	clientSet, namespace, err := k8s.GetClientSet(s.config.Namespace, s.logger)

	if err != nil {
		s.logger.Debug(fmt.Sprintf("Failed to get Kubernetes Client Set: %v", err))
		return err
	}

	s.clientSet = clientSet
	s.config.Namespace = *namespace

	return nil
}

func (s *Store) Count(itemType string, group string) (int, error) {
	err := s.init()
	if err != nil {
		return 0, err
	}

	count := 0
	secretList, err := s.listSecrets(group, itemType)
	if err != nil {
		return 0, err
	}

	count = len(secretList.Items)

	return count, nil
}

func (s *Store) List(itemType string, group string) ([]string, error) {
	err := s.init()
	if err != nil {
		return nil, err
	}

	secretList, err := s.listSecrets(group, itemType)
	if err != nil {
		return nil, err
	}

	results := make([]string, 0, len(secretList.Items))
	// The item name is stored as a label on the secret
	for _, secret := range secretList.Items {
		results = append(results, secret.Labels["name"])
	}
	return results, nil
}

func (s *Store) listSecrets(group string, itemType string) (*v1.SecretList, error) {
	// Secrets does not support fieldSelector on type so type label is used instead
	labelSelector := labels.Set{"group": group, "type": itemType}
	options := metav1.ListOptions{LabelSelector: labelSelector.String()}

	//TODO: Implement Chunking
	secretList, err := s.clientSet.CoreV1().Secrets(s.config.Namespace).List(context.Background(), options)
	if err != nil {
		s.logger.Debug(fmt.Sprintf("Failed to list secrets: %v", err))
		return nil, err
	}

	return secretList, nil
}

func (s *Store) Save(itemType string, group string, name string, data []byte) error {
	err := s.init()
	if err != nil {
		return err
	}

	if itemType == "" && strings.EqualFold(name, "schema") {
		itemType = "schema"
	}

	secret, err := newSecretsObject(itemType, group, name, data)
	if err != nil {
		return err
	}

	if _, err := s.clientSet.CoreV1().Secrets(s.config.Namespace).Create(context.Background(), secret, metav1.CreateOptions{}); err != nil {
		if k8serrors.IsAlreadyExists(err) {
			return s.updateSecret(itemType, group, name, data)
		} else {
			s.logger.Debug(fmt.Sprintf("Failed to Create secrets for item type:%s group:%s item name:%s %v", itemType, group, name, err))
			return err
		}
	}

	return nil

}

func (s *Store) updateSecret(itemType string, group string, name string, data []byte) error {

	fmt.Printf("Attempting to Save data: %s Resource Name %s \n", string(data), getResourceName(itemType, name))
	secret, err := s.clientSet.CoreV1().Secrets(s.config.Namespace).Get(context.Background(), getResourceName(itemType, name), metav1.GetOptions{})
	if err != nil {
		s.logger.Debug(fmt.Sprintf("Failed to get existing secret for item type:%s group:%s item name:%s %v", itemType, group, name, err))
		return err
	}
	encodedData, err := encodeData(data)
	if err != nil {
		return err
	}
	secret.Data = map[string][]byte{name: []byte(encodedData)}
	if _, err := s.clientSet.CoreV1().Secrets(s.config.Namespace).Update(context.Background(), secret, metav1.UpdateOptions{}); err != nil {
		s.logger.Debug(fmt.Sprintf("Failed to update existing secret for item type:%s group:%s item name:%s %v", itemType, group, name, err))
	}
	fmt.Printf("Saved data: %s \n", string(data))
	return err
}

func (s *Store) Read(itemType string, name string) ([]byte, error) {

	err := s.init()
	if err != nil {
		return nil, err
	}

	if itemType == "" && strings.EqualFold(name, "schema") {
		itemType = "schema"
	}

	// Kubernetes secret names must be valid DNS subdomain, the original name is stored as a label.
	resourceName := getResourceName(itemType, name)
	secret, err := s.clientSet.CoreV1().Secrets(s.config.Namespace).Get(context.Background(), resourceName, metav1.GetOptions{})
	if err != nil {
		s.logger.Debug(fmt.Sprintf("Failed to Read secrets for item type:%s item name:%s %v", itemType, name, err))
		// schema migrate relies on error being ErrRecordDoesNotExist in the case of non existant schema.
		if name == "schema" && strings.Contains(err.Error(), fmt.Sprintf("secrets \"%s\" not found", resourceName)) {
			err = crud.ErrRecordDoesNotExist
		}
		return nil, err
	}

	decodedData, err := decodeData(string(secret.Data[name]))
	if err != nil {
		return nil, err
	}

	fmt.Printf("Read data: %s Resource Name: %s \n", decodedData, resourceName)
	return decodedData, nil
}

func (s *Store) Delete(itemType string, name string) error {
	err := s.init()
	if err != nil {
		return err
	}

	if itemType == "" && strings.EqualFold(name, "schema") {
		itemType = "schema"
	}

	resourceName := getResourceName(itemType, name)
	return s.clientSet.CoreV1().Secrets(s.config.Namespace).Delete(context.Background(), resourceName, metav1.DeleteOptions{})
}

// newSecret creates a kubernetes Secret object
// The secret data contains a base 64 encoded gzipped representation of the data
// The type of secret is porter.sh/<itemType>.v1

func newSecretsObject(itemType string, group string, name string, data []byte) (*v1.Secret, error) {
	encodedData, err := encodeData(data)
	if err != nil {
		return nil, err
	}

	var labels = make(map[string]string)

	// Kubernetes secret names must be valid DNS subdomain, the original name is stored as a label.
	labels["name"] = name
	labels["type"] = itemType
	labels["owner"] = "porter"
	labels["group"] = group
	labels["created"] = fmt.Sprintf("%d", time.Now().Unix())
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:   getResourceName(itemType, name),
			Labels: labels,
		},
		Type: v1.SecretType(getSecretType(itemType)),
		Data: map[string][]byte{name: []byte(encodedData)},
	}, nil
}

func encodeData(data []byte) (string, error) {
	if len(data) == 0 {
		return "", nil
	}

	var buf bytes.Buffer
	writer, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return "", err
	}

	if _, err = writer.Write(data); err != nil {
		return "", err
	}

	writer.Close()

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func decodeData(encodedData string) ([]byte, error) {
	if len(encodedData) == 0 {
		return nil, nil
	}

	data, err := base64.StdEncoding.DecodeString(encodedData)
	if err != nil {
		return nil, err
	}

	byteReader := bytes.NewReader(data)
	reader, err := gzip.NewReader(byteReader)
	if err != nil {
		return nil, err
	}

	defer reader.Close()
	result, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func getSecretType(itemType string) string {
	return fmt.Sprintf("porter.sh/%s.v1", itemType)
}

func getResourceName(itemType string, name string) string {
	// Kubernetes secret names must be valid DNS subdomain and type field/label cannot be used for Get/List
	return hex.EncodeToString([]byte(fmt.Sprintf("%s-%s", itemType, name)))
}
