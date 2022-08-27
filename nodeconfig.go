//
// Watch the node endpoints and add them to a configmap

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
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

func updateNodeMap(nodeLabelConfigMap *v1.ConfigMap, object runtime.Object, removed bool) {
	node, ok := object.(*v1.Node)
	if !ok {
		return
	}

	fmt.Printf("Node %v\n", node)
	for k, v := range node.Labels {
		if strings.HasPrefix(k, "kubernetes.io") {
			keys := strings.Split(k, "/")
			fmt.Printf("\t%v: %v\n", keys[1], v)
		} else {
			continue
		}

		if removed {
			delete(nodeLabelConfigMap.Data, k)
		} else {
			nodeLabelConfigMap.Data[k] = v
		}
	}
}

func watchNodes(client *kubernetes.Clientset, nodeLabelConfigMap *v1.ConfigMap, sigs chan os.Signal, done chan bool) {

	node, err := client.CoreV1().Nodes().Watch(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Got an error watching nodes %v\n", err)
		done <- true
	}

	for {
		select {
		case <-sigs:
			node.Stop()
			done <- true
		case event := <-node.ResultChan():
			switch event.Type {
			case watch.Added:
				updateNodeMap(nodeLabelConfigMap, event.Object, false)
			case watch.Deleted:
				updateNodeMap(nodeLabelConfigMap, event.Object, true)
			}
		}
	}
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

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	if *namespace == "" {
		fmt.Printf("Failed to determine namespace where CM should be created\n")
		os.Exit(1)
	}

	if *configmapName == "" {
		fmt.Printf("Failed to determine configmap name\n")
		os.Exit(1)
	}

	client := connectCluster(kubeConfig)
	immutable := false
	nodeLabelConfigMap := &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind: "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: *configmapName,
		},
		Data:      make(map[string]string),
		Immutable: &immutable,
	}

	done := make(chan bool, 1)
	go watchNodes(client, nodeLabelConfigMap, sigs, done)
	<-done
}
