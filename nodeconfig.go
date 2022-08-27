//
// Watch the node endpoints and add them to a configmap

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

//
// Connect to the default cluster based on kubeconfig, in
// the future we might support tokens
//
func connectCluster(kubeConfig *string) *kubernetes.Clientset {

	config, err := clientcmd.BuildConfigFromFlags("", *kubeConfig)
	if err != nil {
		fmt.Errorf("Could not load kubeconfig")
		os.Exit(1)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Errorf("Could not load kubeconfig")
		os.Exit(1)
	}

	return client
}

func main() {

	var kubeConfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeConfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) path of kubeconfig")
	} else {
		kubeConfig = flag.String("kubeconfig", "", "Path of kubeconfig")
	}
	namespace := flag.String("ns", "", "Namespace where configmap should be created")
	configmapName := flag.String("cmName", "", "Name of the configmap")
	flag.Parse()

	if *namespace == "" {
		fmt.Printf("Failed to determine namespace where CM should be created\n")
		os.Exit(1)
	}

	if *configmapName == "" {
		fmt.Printf("Failed to determine configmap name\n")
		os.Exit(1)
	}

	client := connectCluster(kubeConfig)
	existingNodes, err := client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Failed to query nodes %v", err)
		os.Exit(1)
	}

	nodeLabelData := make(map[string]string)
	for _, node := range existingNodes.Items {
		fmt.Printf("Node information %v:\n", node.Name)
		for k, v := range node.Labels {
			if strings.HasPrefix(k, "kubernetes.io") {
				keys := strings.Split(k, "/")
				nodeLabelData[keys[1]] = v
				fmt.Printf("\t%v: %v\n", keys[1], v)
			}
		}
	}

	exists := false
	configMaps, err := client.CoreV1().ConfigMaps(*namespace).Get(context.TODO(), *configmapName, metav1.GetOptions{})
	if err == nil {
		exists = true
	}

	if !exists {
		fmt.Printf("Creating new config map %s\n", *configmapName)
		immutable := false
		nodeLabelConfigMap := &v1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind: "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: *configmapName,
			},
			Immutable: &immutable,
			Data:      nodeLabelData,
		}
		configMaps, err = client.CoreV1().ConfigMaps(*namespace).Create(context.TODO(), nodeLabelConfigMap, metav1.CreateOptions{})
		if err != nil {
			fmt.Printf("Failed to create basic configmap %v", err)
			os.Exit(1)
		}
	} else {
		fmt.Printf("Existing Node Label Maps")
		for k, v := range configMaps.Data {
			fmt.Printf("\t%v: %v\n", k, v)
		}
	}

	// for {

	// 	nodes, err := client.CoreV1().Nodes().Watch(context.TODO(), metav1.ListOptions{})
	// }
}
