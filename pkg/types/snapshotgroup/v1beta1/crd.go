package v1

import (
	"context"
	"fmt"
	"reflect"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
)

// CreateCustomResourceDefinition creates the CRD and add it into Kubernetes. If there is error,
// it will do some clean up.
func CreateCustomResourceDefinition(namespace string, clientSet apiextensionsclientset.Interface) (*apiextensionsv1.CustomResourceDefinition, error) {
	crdVersion := apiextensionsv1.CustomResourceDefinitionVersion{
		Name:               GroupVersion,
		Served:             true,
		Storage:            true,
		Deprecated:         false,
		DeprecationWarning: nil,
		Schema: &apiextensionsv1.CustomResourceValidation{
			OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
				Type: "object",
				Properties: map[string]apiextensionsv1.JSONSchemaProps{
					"spec": {
						Type: "object",
						Properties: map[string]apiextensionsv1.JSONSchemaProps{
							"persistentVolumeClaim": {
								Type: "object",
								Properties: map[string]apiextensionsv1.JSONSchemaProps{
									"claimName": {
										Description: "PersistentVolumeClaim name to backup",
										Type:        "string",
									},
									"spec": {
										Description: "PersistentVolumeClaim spec to create and backup",
										Type:        "object",
									},
								},
							},
							"schedule": {
								Type: "array",
								Items: &apiextensionsv1.JSONSchemaPropsOrArray{
									Schema: &apiextensionsv1.JSONSchemaProps{
										Type: "object",
										Properties: map[string]apiextensionsv1.JSONSchemaProps{
											"every": {
												Description: "Interval for creating new backups",
												Type:        "string",
											},
											"keep": {
												Description: "Number of historical backups to keep",
												Type:        "integer",
											},
										},
									},
								},
							},
							"template": {
								Type: "object",
								Properties: map[string]apiextensionsv1.JSONSchemaProps{
									"spec": {
										Description: "VolumeSnapshot spec",
										Type:        "object",
									},
								},
							},
						},
					},
				},
			},
		},
		Subresources:             nil,
		AdditionalPrinterColumns: nil,
	}

	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CRDName,
			Namespace: namespace,
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: GroupName,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural: Plural,
				Kind:   reflect.TypeOf(SnapshotGroup{}).Name(),
			},
			Scope:                 apiextensionsv1.NamespaceScoped,
			Versions:              []apiextensionsv1.CustomResourceDefinitionVersion{crdVersion},
			PreserveUnknownFields: false,
		},
	}

	_, err := clientSet.ApiextensionsV1().CustomResourceDefinitions().Create(context.TODO(), crd, metav1.CreateOptions{})
	if err == nil {
		fmt.Println("CRD SnapshotGroup is created")
	} else if apierrors.IsAlreadyExists(err) {
		fmt.Println("CRD SnapshotGroup already exists")
	} else {
		fmt.Printf("Fail to create CRD SnapshotGroup: %+v\n", err)

		return nil, err
	}

	// Wait for CRD creation.
	err = wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
		crd, err = clientSet.ApiextensionsV1().CustomResourceDefinitions().Get(context.TODO(), CRDName, metav1.GetOptions{})
		if err != nil {
			fmt.Printf("Fail to wait for CRD SnapshotGroup creation: %+v\n", err)

			return false, err
		}
		for _, cond := range crd.Status.Conditions {
			switch cond.Type {
			case apiextensionsv1.Established:
				if cond.Status == apiextensionsv1.ConditionTrue {
					return true, err
				}
			case apiextensionsv1.NamesAccepted:
				if cond.Status == apiextensionsv1.ConditionFalse {
					fmt.Printf("Name conflict while wait for CRD SnapshotGroup creation: %s, %+v\n", cond.Reason, err)
				}
			}
		}

		return false, err
	})

	// If there is an error, delete the object to keep it clean.
	if err != nil {
		fmt.Println("Try to cleanup")
		deleteErr := clientSet.ApiextensionsV1().CustomResourceDefinitions().Delete(context.TODO(), CRDName, metav1.DeleteOptions{})
		if deleteErr != nil {
			fmt.Printf("Fail to delete CRD SnapshotGroup: %+v\n", deleteErr)

			return nil, errors.NewAggregate([]error{err, deleteErr})
		}

		return nil, err
	}

	return crd, nil
}
