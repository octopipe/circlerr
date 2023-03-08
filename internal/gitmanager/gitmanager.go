package gitmanager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	circlerriov1alpha1 "github.com/octopipe/circlerr/internal/api/v1alpha1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	httpsAuthType       = "HTTPS"
	sshAuthType         = "SSH"
	accessTokenAuthType = "ACCESS_TOKEN"
)

type Manager interface {
	Sync(module circlerriov1alpha1.Module) error
}

type manager struct {
	client.Client
}

func NewManager(client client.Client) Manager {
	return manager{
		Client: client,
	}
}

func (r manager) getSecretByModule(secretRef circlerriov1alpha1.SecretRef) (apiv1.Secret, error) {
	ref := types.NamespacedName{
		Name:      secretRef.Name,
		Namespace: secretRef.Namespace,
	}
	secret := apiv1.Secret{}
	err := r.Get(context.Background(), ref, &secret)
	if err != nil {
		return apiv1.Secret{}, err
	}

	return secret, nil
}

func getHttpsAuthMethodBySecretData(secretData map[string][]byte) (transport.AuthMethod, error) {

	username, ok := secretData["username"]
	if !ok {
		return nil, errors.New("username not found")
	}

	password, ok := secretData["password"]
	if !ok {
		return nil, errors.New("password not found")
	}

	authMethod := &http.BasicAuth{
		Username: string(username),
		Password: string(password),
	}

	return authMethod, nil
}

func getSSHAuthMethodBySecretData(secretData map[string][]byte) (transport.AuthMethod, error) {
	sshPrivateKey, ok := secretData["sshPrivateKey"]
	if !ok {
		return nil, errors.New("auth type not found")
	}

	return ssh.NewPublicKeys("git", sshPrivateKey, "")
}

func getAccessTokenAuthMethodBySecretData(secretData map[string][]byte) (transport.AuthMethod, error) {
	username, ok := secretData["username"]
	if !ok {
		return nil, errors.New("username not found")
	}

	accessToken, ok := secretData["accessToken"]
	if !ok {
		return nil, errors.New("auth type not found")
	}

	authMethod := &http.BasicAuth{
		Username: string(username),
		Password: string(accessToken),
	}

	return authMethod, nil
}

func (r *manager) getAuthMethodBySecret(secret apiv1.Secret) (transport.AuthMethod, error) {
	authType, ok := secret.Data["type"]
	if !ok {
		return nil, errors.New("auth type not found")
	}

	if string(authType) == httpsAuthType {
		method, err := getHttpsAuthMethodBySecretData(secret.Data)
		if err != nil {
			return nil, err
		}

		return method, nil
	}

	if string(authType) == accessTokenAuthType {
		method, err := getAccessTokenAuthMethodBySecretData(secret.Data)
		if err != nil {
			return nil, err
		}

		return method, nil
	}

	if string(authType) == sshAuthType {
		method, err := getSSHAuthMethodBySecretData(secret.Data)
		if err != nil {
			return nil, err
		}

		return method, nil
	}

	return nil, errors.New("invalid auth type")
}

func (r manager) Sync(module circlerriov1alpha1.Module) error {
	gitCloneConfig := &git.CloneOptions{
		URL:  module.Spec.Url,
		Auth: nil,
	}

	if module.Spec.SecretRef != nil {
		secret, err := r.getSecretByModule(*module.Spec.SecretRef)
		if err != nil {
			return err
		}
		authMethod, err := r.getAuthMethodBySecret(secret)
		if err != nil {
			return err
		}
		gitCloneConfig.Auth = authMethod
	}

	path := fmt.Sprintf("%s/%s", os.Getenv("REPOSITORIES_TMP_DIR"), module.Spec.Path)
	repo, err := git.PlainClone(path, false, gitCloneConfig)
	if err != nil && !strings.Contains(err.Error(), "manager already exists") {
		return err
	}

	if strings.Contains(err.Error(), "manager already exists") {
		repo, err = git.PlainOpen(path)
		if err != nil {
			return err
		}
	}

	w, err := repo.Worktree()
	if err != nil {
		return err
	}

	err = w.Pull(&git.PullOptions{RemoteName: "origin"})
	if err != nil && !strings.Contains(err.Error(), "already up-to-date") {
		return err
	}

	return nil
}
