/*
Copyright 2020 the Velero contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package backup

import (
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

// RemapCRDVersionAction inspects a PersistentVolumeClaim for the PersistentVolume
// that it references and backs it up
type RemapCRDVersionAction struct {
	logger logrus.FieldLogger
}

func NewRemapCRDVersionAction(logger logrus.FieldLogger) *RemapCRDVersionAction {
	return &RemapCRDVersionAction{logger: logger}
}

func (a *RemapCRDVersionAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{"customresourcedefinition.apiextensions.k8s.io"},
	}, nil
}

func (a *RemapCRDVersionAction) Execute(item runtime.Unstructured, backup *v1.Backup) (runtime.Unstructured, []velero.ResourceIdentifier, error) {
	a.logger.Info("Executing RemapCRDVersionAction")

	var crd apiextv1.CustomResourceDefinition
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), &crd); err != nil {
		return nil, nil, errors.Wrap(err, "unable to convert unstructured item to CRD")
	}

	log := a.logger.WithField("plugin", "RemapCRDVersionAction").WithField("CRD", crd.Name)

	// check spec schema
	if len(crd.Spec.Versions) > 0 {
		// check the first version schema is enough to know it'	s original api version was
		// since
		//    for v1beta1, all versions share the same schema and shema might be empty
		//    for v1, all versions must have schema and schame cannot be empty
		if crd.Spec.Versions[0].Schema == nil || crd.Spec.Versions[0].Schema.OpenAPIV3Schema == nil {
			log.Debug("CRD is a candidate for v1beta1 backup")

			tempMap := item.UnstructuredContent()
			if err := unstructured.SetNestedField(tempMap, "apiextensions.k8s.io/v1beta1", "apiVersion"); err != nil {
				return nil, nil, errors.Wrap(err, "unable to set apiversion to v1beta1")
			}
			item.SetUnstructuredContent(tempMap)
		}
	}

	// check status condition
	for _, c := range crd.Status.Conditions {
		// if the crd status is 'non-structural schema", change the api version as v1beta1
		if c.Type == apiextv1.NonStructuralSchema {
			log.Debug("CRD is a non-structural schema")

			tempMap := item.UnstructuredContent()
			if err := unstructured.SetNestedField(tempMap, "apiextensions.k8s.io/v1beta1", "apiVersion"); err != nil {
				return nil, nil, errors.Wrap(err, "unable to set apiversion to v1beta1")
			}
			item.SetUnstructuredContent(tempMap)

			break
		}
	}

	return item, nil, nil
}
