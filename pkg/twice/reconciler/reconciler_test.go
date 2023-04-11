package reconciler

import (
	"context"
	"flag"
	"path/filepath"
	"testing"

	"github.com/go-logr/zapr"
	"github.com/octopipe/circlerr/pkg/twice/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type ReconcilerTestSuite struct {
	suite.Suite
	reconciler Reconciler
}

func (suite *ReconcilerTestSuite) SetupTest() {
	logger, _ := zap.NewProduction()
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err)
	}
	inMemoryCache := cache.NewLocalCache()
	suite.reconciler = NewReconciler(zapr.NewLogger(logger), config, inMemoryCache)
}

func (suite *ReconcilerTestSuite) TestPlan() {
	deployment := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    app: nginx
spec:
  replicas: 1
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
	service := `
apiVersion: v1
kind: Service
metadata:
  name: my-service
spec:
  selector:
    app.kubernetes.io/name: MyApp
  ports:
    - protocol: TCP
      port: 80
      targetPort: 9376
    `

	isManagedByTwice := func(un *unstructured.Unstructured) bool {
		annotations := un.GetAnnotations()
		controlledBy, ok := annotations["controlled-by"]

		return ok && controlledBy == "twice.io"
	}

	suite.reconciler.Preload(context.TODO(), isManagedByTwice, false)

	isManagedBySpecificSection := func(un *unstructured.Unstructured) bool {
		annotations := un.GetAnnotations()
		controlledBy, ok := annotations["controlled-by"]
		if !ok {
			return false
		}

		specificSection, ok := annotations["twice.io/specific-section"]
		if !ok {
			return false
		}

		return controlledBy == "twice.io" && specificSection == "section-1"
	}

	planResults, err := suite.reconciler.Plan(context.TODO(), []string{deployment, service}, "default", isManagedBySpecificSection)
	assert.NoError(suite.T(), err)

	_, err = suite.reconciler.Apply(context.TODO(), planResults, "default")
	assert.NoError(suite.T(), err)
}

func TestReconcilerTestSuite(t *testing.T) {
	suite.Run(t, new(ReconcilerTestSuite))
}
