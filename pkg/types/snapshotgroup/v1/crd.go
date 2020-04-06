package v1

import (
	"context"
	"fmt"
	"reflect"
	"time"

	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
)

// CreateCustomResourceDefinition creates the CRD and add it into Kubernetes. If there is error,
// it will do some clean up.
func CreateCustomResourceDefinition(namespace string, clientSet apiextensionsclientset.Interface) (*apiextensionsv1beta1.CustomResourceDefinition, error) {
	crd := &apiextensionsv1beta1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CRDName,
			Namespace: namespace,
		},
		Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
			Group:   GroupName,
			Version: SchemeGroupVersion.Version,
			Scope:   apiextensionsv1beta1.NamespaceScoped,
			Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
				Plural: Plural,
				Kind:   reflect.TypeOf(SnapshotGroup{}).Name(),
			},
		},
	}
	_, err := clientSet.ApiextensionsV1beta1().CustomResourceDefinitions().Create(context.TODO(), crd, metav1.CreateOptions{})
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
		crd, err = clientSet.ApiextensionsV1beta1().CustomResourceDefinitions().Get(context.TODO(), CRDName, metav1.GetOptions{})
		if err != nil {
			fmt.Printf("Fail to wait for CRD SnapshotGroup creation: %+v\n", err)

			return false, err
		}
		for _, cond := range crd.Status.Conditions {
			switch cond.Type {
			case apiextensionsv1beta1.Established:
				if cond.Status == apiextensionsv1beta1.ConditionTrue {
					return true, err
				}
			case apiextensionsv1beta1.NamesAccepted:
				if cond.Status == apiextensionsv1beta1.ConditionFalse {
					fmt.Printf("Name conflict while wait for CRD SnapshotGroup creation: %s, %+v\n", cond.Reason, err)
				}
			}
		}

		return false, err
	})

	// If there is an error, delete the object to keep it clean.
	if err != nil {
		fmt.Println("Try to cleanup")
		deleteErr := clientSet.ApiextensionsV1beta1().CustomResourceDefinitions().Delete(context.TODO(), CRDName, nil)
		if deleteErr != nil {
			fmt.Printf("Fail to delete CRD SnapshotGroup: %+v\n", deleteErr)
			return nil, errors.NewAggregate([]error{err, deleteErr})
		}

		return nil, err
	}

	return crd, nil
}
