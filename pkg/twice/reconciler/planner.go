package reconciler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/octopipe/circlerr/pkg/twice/cache"
	"github.com/octopipe/circlerr/pkg/twice/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
)

type plannerContext struct {
	cache           cache.Cache
	discoveryClient *discovery.DiscoveryClient
	preHook         func(un *unstructured.Unstructured) *unstructured.Unstructured
}

type plannerOpt func(ctx *plannerContext)

type Planner interface {
	Plan(ctx context.Context, manifests []string, namespace string, isManaged isManagedFunc, opts ...plannerOpt) ([]PlanResult, error)
}

func WithPreHook(preHook func(un *unstructured.Unstructured) *unstructured.Unstructured) plannerOpt {
	return func(ctx *plannerContext) {
		ctx.preHook = preHook
	}
}

func NewPlanner(cache cache.Cache, discoveryClient *discovery.DiscoveryClient) Planner {
	return plannerContext{
		cache:           cache,
		discoveryClient: discoveryClient,
		preHook: func(un *unstructured.Unstructured) *unstructured.Unstructured {
			return un
		},
	}
}

func (c plannerContext) Plan(ctx context.Context, manifests []string, namespace string, isManaged isManagedFunc, opts ...plannerOpt) ([]PlanResult, error) {
	for _, opt := range opts {
		opt(&c)
	}

	allManifests := [][]byte{}
	result := []PlanResult{}

	for _, m := range manifests {
		splitedManifest, err := c.splitManifest([]byte(m))
		if err != nil {
			return nil, err
		}

		allManifests = append(allManifests, splitedManifest...)
	}

	for _, m := range allManifests {
		un, err := c.deserializer(m)
		if err != nil {
			return nil, err
		}

		un = c.preHook(un)
		resourceName, err := c.getResourceName(un)
		if err != nil {
			return nil, err
		}

		res := resource.NewResourceByUnstructured(*un, namespace, resourceName, true)
		if !c.cache.Has(res.GetResourceIdentifier()) {
			result = append(result, PlanResult{
				Resource:       res,
				Action:         PlanCreateAction,
				SrcManifest:    string(m),
				TargetManifest: string(m),
				DiffString:     []string{},
			})
			continue
		}

		if err != nil {
			return nil, err
		}

		currentResource := c.cache.Get(res.GetResourceIdentifier())
		lastAppliedConfiguration := c.getLastAppliedConfiguration(currentResource.Object)
		patch, err := c.getMergePatch([]byte(lastAppliedConfiguration), m)
		if err != nil {
			return nil, err
		}

		currentAction := PlanImmutableAction
		target := []byte(lastAppliedConfiguration)
		if string(patch) != "{}" {
			currentAction = PlanUpdateAction

			target, err = jsonpatch.MergePatch(target, patch)
			if err != nil {
				return nil, err
			}
		}

		targetObject := &unstructured.Unstructured{}
		err = json.Unmarshal(target, targetObject)
		if err != nil {
			return nil, err
		}

		c.preHook(targetObject)
		res.Object = targetObject
		result = append(result, PlanResult{
			Resource:       res,
			Action:         currentAction,
			SrcManifest:    string(m),
			TargetManifest: string(target),
			DiffString:     []string{},
		})
	}

	resultsForDeletion := c.getPlanResultsForDeletion(isManaged, result)
	result = append(result, resultsForDeletion...)

	return result, nil
}

func (c plannerContext) splitManifest(manifest []byte) ([][]byte, error) {
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(manifest), 4096)
	manifests := [][]byte{}

	for {
		newManifest := runtime.RawExtension{}
		err := decoder.Decode(&newManifest)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		newManifest.Raw = bytes.TrimSpace(newManifest.Raw)
		if len(newManifest.Raw) == 0 || bytes.Equal(newManifest.Raw, []byte("null")) {
			continue
		}

		manifests = append(manifests, newManifest.Raw)
	}

	return manifests, nil
}

func (c plannerContext) deserializer(manifest []byte) (*unstructured.Unstructured, error) {
	un := &unstructured.Unstructured{}
	if err := json.Unmarshal([]byte(manifest), un); err != nil {
		return nil, err
	}

	return un, nil
}

func (c plannerContext) getLastAppliedConfiguration(un *unstructured.Unstructured) string {
	annotations := un.GetAnnotations()

	kubectlLastAppliedConfigurationAnnotation := "kubectl.kubernetes.io/last-applied-configuration"
	kubectlLastAppliedConfiguration, ok := annotations[kubectlLastAppliedConfigurationAnnotation]
	if ok {
		return kubectlLastAppliedConfiguration
	}

	twiceLastAppliedConfig, ok := annotations[LastAppliedConfigurationAnnotation]
	if ok {
		return twiceLastAppliedConfig
	}

	return ""
}

func (c plannerContext) removeNulls(m map[string]interface{}) {
	val := reflect.ValueOf(m)
	for _, e := range val.MapKeys() {
		v := val.MapIndex(e)
		if v.IsNil() {
			delete(m, e.String())
			continue
		}
		switch t := v.Interface().(type) {
		// If key is a JSON object (Go Map), use recursion to go deeper
		case map[string]interface{}:
			c.removeNulls(t)
		}
	}
}

func (c plannerContext) getMergePatch(original []byte, modified []byte) ([]byte, error) {
	patch, err := jsonpatch.CreateMergePatch(original, modified)
	if err != nil {
		return nil, err
	}

	p := map[string]interface{}{}
	err = json.Unmarshal(patch, &p)
	if err != nil {
		return nil, err
	}

	c.removeNulls(p)
	l, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}

	return l, nil
}

func (c plannerContext) getResourceName(un *unstructured.Unstructured) (string, error) {
	apiResourceList, err := c.discoveryClient.ServerResourcesForGroupVersion(un.GroupVersionKind().GroupVersion().String())
	if err != nil {
		return "", err
	}

	for _, apiResource := range apiResourceList.APIResources {
		if !isSupportedVerb(apiResource.Verbs) {
			continue
		}

		if apiResource.Kind == un.GetKind() {
			return apiResource.Name, nil
		}
	}

	return "", errors.New("server resource not supported")
}

func (c plannerContext) getPlanResultsForDeletion(isManaged isManagedFunc, currentResults []PlanResult) []PlanResult {
	result := []PlanResult{}
	cachedResources := c.cache.List(func(res resource.Resource) bool {
		return res.Object != nil && isManaged(res.Object)
	})
	for _, cachedKey := range cachedResources {
		forDeletion := true
		for _, planResult := range currentResults {
			if cachedKey == planResult.GetResourceIdentifier() {
				forDeletion = false
			}
		}

		if forDeletion {
			cachedItem := c.cache.Get(cachedKey)

			isControlled := false
			// Verifying if cached resource has a controller to prevent accidentally deleting
			for _, owner := range cachedItem.Object.GetOwnerReferences() {
				isController := owner.Controller
				if isController != nil && *isController {
					isControlled = true
					break
				}
			}

			if !isControlled {
				result = append(result, PlanResult{
					Resource:       cachedItem,
					Action:         PlanDeleteAction,
					SrcManifest:    c.getLastAppliedConfiguration(cachedItem.Object),
					TargetManifest: "",
				})
			}
		}
	}

	return result
}
