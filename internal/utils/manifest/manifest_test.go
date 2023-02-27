package manifest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type ManifestTestSuite struct {
	suite.Suite
}

func (s *ManifestTestSuite) SetupTest() {

}

func (s *ManifestTestSuite) TestToUnstructuredSingleManifest() {
	singleManifest := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    app: nginx
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
        ports:
        - containerPort: 80
`
	uns, err := SplitManifests([]byte(singleManifest))
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 1, len(uns))
	assert.Contains(s.T(), "nginx-deployment", uns[0])
}

func (s *ManifestTestSuite) TestToUnstructuredMultipleManifests() {
	multipleManifests := `
apiVersion: v1
kind: Service
metadata:
  name: my-nginx-svc
  labels:
    app: nginx
spec:
  type: LoadBalancer
  ports:
  - port: 80
  selector:
    app: nginx
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-nginx
  labels:
    app: nginx
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
        ports:
        - containerPort: 80
`
	un, err := SplitManifests([]byte(multipleManifests))
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 2, len(un))
	assert.Contains(s.T(), "my-nginx-svc", un[0])
	assert.Contains(s.T(), "my-nginx", un[1])
}

func TestManifestTestSuite(t *testing.T) {
	suite.Run(t, new(ManifestTestSuite))
}
