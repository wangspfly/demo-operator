package helper

import (
	"context"
	v1 "demo-operator/api/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func IsExistPod(podName string, deploy *v1.Deploy, client client.Client) bool {
	err := client.Get(context.Background(), types.NamespacedName{
		Namespace: deploy.Namespace,
		Name:      podName,
	},
		&corev1.Pod{},
	)

	if err != nil {
		return false
	}
	return true
}

func CreateDeploy(client client.Client, deployConfig *v1.Deploy, podName string, schema *runtime.Scheme) error {
	if IsExistPod(podName, deployConfig, client) {
		return nil
	}
	// 建立 POD 对象
	newPod := &corev1.Pod{}
	newPod.Name = podName
	newPod.Namespace = deployConfig.Namespace
	newPod.Spec = corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:            podName,
				Image:           "test:latest",
				ImagePullPolicy: corev1.PullAlways,
			},
		},
	}

	// set owner reference，使用ControllerManager为我们管理 POD
	// 这个就和ReplicateSet是一个道理
	err := controllerutil.SetControllerReference(deployConfig, newPod, schema)
	if err != nil {
		return err
	}
	// 创建 POD
	err = client.Create(context.Background(), newPod)
	return err
}
