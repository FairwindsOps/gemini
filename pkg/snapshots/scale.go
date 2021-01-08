package snapshots

import (
	"encoding/json"

	autoscale "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/fairwindsops/gemini/pkg/kube"
)

type scaleItem struct {
	Resource schema.GroupResource `yaml:"resource"`
	Name     string               `yaml:"name"`
	scale    int32
}

func parseScaleAnnotation(annot string) ([]scaleItem, error) {
	items := []scaleItem{}
	if annot == "" {
		return items, nil
	}
	err := json.Unmarshal([]byte(annot), &items)
	return items, err
}

func scaleDown(namespace string, items []scaleItem) error {
	client := kube.GetClient()
	scaler := client.ScaleClient.Scales(namespace)
	for _, item := range items {
		current, err := scaler.Get(item.Resource, item.Name)
		if err != nil {
			return err
		}
		item.scale = current.Spec.Replicas
		err = scaleTo(namespace, item, 0)
		if err != nil {
			return err
		}
	}
	return nil
}

func scaleUp(namespace string, items []scaleItem) error {
	for _, item := range items {
		err := scaleTo(namespace, item, item.scale)
		if err != nil {
			return err
		}
	}
	return nil
}

func scaleTo(namespace string, item scaleItem, amt int32) error {
	client := kube.GetClient()
	scaler := client.ScaleClient.Scales(namespace)
	newScale := autoscale.Scale{
		ObjectMeta: metav1.ObjectMeta{
			Name:      item.Name,
			Namespace: namespace,
		},
		Spec: autoscale.ScaleSpec{
			Replicas: amt,
		},
	}
	_, err := scaler.Update(item.Resource, &newScale)
	return err
}
