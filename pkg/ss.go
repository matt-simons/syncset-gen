package pkg

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	hivev1 "github.com/openshift/hive/pkg/apis/hive/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

func loadSecrets(name, prefix, paths string) ([]hivev1.SecretMapping, error) {
	var secrets = []hivev1.SecretMapping{}
	if paths == "" {
		return secrets, nil
	}
	for _, path := range strings.Split(paths, ",") {
		err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if strings.HasSuffix(p, ".yaml") {
				data, err := ioutil.ReadFile(p)
				if err != nil {
					return err
				}
				jsonBytes, err := yaml.YAMLToJSON(data)
				if err != nil {
					return err
				}
				var j map[string]interface{}
				json.Unmarshal(jsonBytes, &j)
				kind, ok := j["kind"].(string)
				if ok && kind == "Secret" {
					metadata, ok := j["metadata"].(map[string]interface{})
					if !ok {
						return errors.New("Could not read metadata of " + p)
					}
					n, ok := metadata["name"].(string)
					if !ok {
						return errors.New("Could not read metadata.name of " + p)
					}
					ns, ok := metadata["namespace"].(string)
					if !ok {
						return errors.New("Could not read metadata.namespace of " + p)
					}
					secret := hivev1.SecretMapping{
						SourceRef: hivev1.SecretReference{
							Namespace: "remote-secrets",
							Name:      fmt.Sprintf("%s-%s-%s-%s", prefix, name, ns, n),
						},
						TargetRef: hivev1.SecretReference{
							Namespace: ns,
							Name:      n,
						},
					}
					secrets = append(secrets, secret)
				}
			}
			return nil
		})
		if err != nil {
			if _, ok := err.(*os.PathError); !ok {
				return secrets, err
			}
		}
	}
	return secrets, nil
}

func loadResources(paths string) ([]runtime.RawExtension, error) {
	var resources = []runtime.RawExtension{}
	if paths == "" {
		return resources, nil
	}
	for _, path := range strings.Split(paths, ",") {
		err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if strings.HasSuffix(p, ".yaml") {
				data, err := ioutil.ReadFile(p)
				if err != nil {
					return err
				}
				jsonBytes, err := yaml.YAMLToJSON(data)
				if err != nil {
					return err
				}
				r, err := rawExtensionWithUnstructured(jsonBytes)
				if err != nil {
					return err
				}
				if !isSecret(r.Object) {
					resources = append(resources, *r)
				}
			}
			return nil
		})
		if err != nil {
			if _, ok := err.(*os.PathError); !ok {
				return resources, err
			}
		}
	}

	// Attempt to sort resources
	sort.Slice(resources, func(i, j int) bool {
		return isLessThan(resources[i], resources[j])
	})

	return resources, nil
}

func loadPatches(paths string) ([]hivev1.SyncObjectPatch, error) {
	var patches = []hivev1.SyncObjectPatch{}
	if paths == "" {
		return patches, nil
	}
	for _, path := range strings.Split(paths, ",") {
		err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if strings.HasSuffix(p, ".yaml") {
				data, err := ioutil.ReadFile(p)
				if err != nil {
					return err
				}
				jsonBytes, err := yaml.YAMLToJSON(data)
				if err != nil {
					return err
				}
				var p = hivev1.SyncObjectPatch{}
				json.Unmarshal(jsonBytes, &p)
				patches = append(patches, p)
			}
			return nil
		})
		if err != nil {
			if _, ok := err.(*os.PathError); !ok {
				return patches, err
			}
		}
	}
	return patches, nil
}

func TransformSecrets(name, prefix, paths string) []corev1.Secret {
	var secrets = []corev1.Secret{}
	if paths == "" {
		return nil
	}
	for _, path := range strings.Split(paths, ",") {
		err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if strings.HasSuffix(p, ".yaml") {
				data, err := ioutil.ReadFile(p)
				if err != nil {
					return err
				}
				jsonBytes, err := yaml.YAMLToJSON(data)
				if err != nil {
					return err
				}
				var j map[string]interface{}
				json.Unmarshal(jsonBytes, &j)
				kind, ok := j["kind"].(string)
				if ok && kind == "Secret" {
					var s = corev1.Secret{}
					json.Unmarshal(jsonBytes, &s)
					ns := s.ObjectMeta.GetNamespace()
					n := s.ObjectMeta.GetName()
					l := s.ObjectMeta.GetLabels()
					if l == nil {
						l = make(map[string]string)
					}
					key := "atlas.worldpay.com/" + prefix
					l[key] = name
					s.ObjectMeta.SetNamespace("remote-secrets")
					s.ObjectMeta.SetName(fmt.Sprintf("%s-%s-%s-%s", prefix, name, ns, n))
					s.ObjectMeta.SetLabels(l)
					secrets = append(secrets, s)
				}
			}
			return nil
		})
		if err != nil {
			if _, ok := err.(*os.PathError); !ok {
				log.Println(err)
			}
		}
	}
	return secrets
}

// Taken directly from kustomise
// https://github.com/kubernetes-sigs/kustomize/blob/master/kyaml/resid/gvk.go
//
// An attempt to order things to help k8s, e.g.
// a Service should come before things that refer to it.
// Namespace should be first.
// In some cases order just specified to provide determinism.
var orderFirst = []string{
	"Namespace",
	"ResourceQuota",
	"StorageClass",
	"CustomResourceDefinition",
	"ServiceAccount",
	"PodSecurityPolicy",
	"SecurityContextConstraints",
	"Role",
	"ClusterRole",
	"RoleBinding",
	"ClusterRoleBinding",
	"ConfigMap",
	"Secret",
	"Endpoints",
	"Service",
	"LimitRange",
	"PriorityClass",
	"PersistentVolume",
	"PersistentVolumeClaim",
	"Deployment",
	"StatefulSet",
	"CronJob",
	"PodDisruptionBudget",
	"CatalogSource",
	"OperatorGroup",
	"Subscription",
}
var orderLast = []string{
	"MutatingWebhookConfiguration",
	"ValidatingWebhookConfiguration",
}
var typeOrders = func() map[string]int {
	m := map[string]int{}
	for i, n := range orderFirst {
		m[n] = -len(orderFirst) + i
	}
	for i, n := range orderLast {
		m[n] = 1 + i
	}
	return m
}()

// isLessThan returns true if i is less important than j
func isLessThan(i, j runtime.RawExtension) bool {
	indexI := typeOrders[i.Object.GetObjectKind().GroupVersionKind().Kind]
	indexJ := typeOrders[j.Object.GetObjectKind().GroupVersionKind().Kind]
	if indexI != indexJ {
		return indexI < indexJ
	}
	return i.Object.GetObjectKind().GroupVersionKind().String() < j.Object.GetObjectKind().GroupVersionKind().String()
}

func convertRawExtension(in *runtime.RawExtension) error {
	u := &unstructured.Unstructured{}
	err := yaml.UnmarshalStrict(in.Raw, u)
	if err != nil {
		return err
	}
	in.Object = u
	return nil
}

func rawExtensionWithUnstructured(data []byte) (*runtime.RawExtension, error) {
	re := &runtime.RawExtension{}
	err := yaml.Unmarshal(data, re)
	if err != nil {
		return nil, err
	}
	err = convertRawExtension(re)
	if err != nil {
		return nil, err
	}
	return re, nil
}

func isSecret(o runtime.Object) bool {
	if o == nil {
		return false
	}
	if o.GetObjectKind().GroupVersionKind().Kind != "Secret" {
		return false
	}
	if o.GetObjectKind().GroupVersionKind().Version != "v1" {
		return false
	}
	return true
}

func CreateSelectorSyncSet(name, selector, resourcesPath, patchesPath, applyMode string) hivev1.SelectorSyncSet {
	resources, err := loadResources(resourcesPath)
	if err != nil {
		log.Println(err)
	}

	secrets, err := loadSecrets(name, "sss", resourcesPath)
	if err != nil {
		log.Println(err)
	}

	patches, err := loadPatches(patchesPath)
	if err != nil {
		log.Println(err)
	}

	labelSelector, err := metav1.ParseToLabelSelector(selector)
	if err != nil {
		log.Println(err)
	}

	var syncSet = &hivev1.SelectorSyncSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SelectorSyncSet",
			APIVersion: "hive.openshift.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"generated": "true",
			},
		},
		Spec: hivev1.SelectorSyncSetSpec{
			SyncSetCommonSpec: hivev1.SyncSetCommonSpec{
				Resources:         resources,
				Patches:           patches,
				ResourceApplyMode: hivev1.SyncSetResourceApplyMode(applyMode),
				Secrets:           secrets,
			},
			ClusterDeploymentSelector: *labelSelector,
		},
	}
	return *syncSet
}

func CreateSyncSet(name, clusterName, resourcesPath, patchesPath, applyMode string) hivev1.SyncSet {
	resources, err := loadResources(resourcesPath)
	if err != nil {
		log.Println(err)
	}

	secrets, err := loadSecrets(name, "ss", resourcesPath)
	if err != nil {
		log.Println(err)
	}

	patches, err := loadPatches(patchesPath)
	if err != nil {
		log.Println(err)
	}

	var syncSet = &hivev1.SyncSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SyncSet",
			APIVersion: "hive.openshift.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"generated": "true",
			},
		},
		Spec: hivev1.SyncSetSpec{
			SyncSetCommonSpec: hivev1.SyncSetCommonSpec{
				Resources:         resources,
				Patches:           patches,
				ResourceApplyMode: hivev1.SyncSetResourceApplyMode(applyMode),
				Secrets:           secrets,
			},
			ClusterDeploymentRefs: []corev1.LocalObjectReference{
				corev1.LocalObjectReference{
					Name: clusterName,
				},
			},
		},
	}
	return *syncSet
}
