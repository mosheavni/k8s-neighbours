package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/nleeper/goment"
	"github.com/olekukonko/tablewriter"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	var podName string
	flag.StringVar(&podName, "pod", "", "Name of the pod")

	var nodeName string
	flag.StringVar(&nodeName, "node", "", "Name of the node")

	var namespace string
	flag.StringVar(&namespace, "namespace", "", "Namespace of the pod")

	// parse flags
	flag.Parse()

	// allow either pod or node name
	if podName == "" && nodeName == "" {
		programName := os.Args[0]
		fmt.Printf("Usage: %s -pod <pod_name> -node <node_name> [-namespace <namespace>]\n", programName)
		os.Exit(1)
	}

	config, err := rest.InClusterConfig()
	if err != nil {

		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		// if you want to change the loading rules (which files in which order), you can do so here

		configOverrides := &clientcmd.ConfigOverrides{}
		// if you want to change override values or bind them to flags, there are methods to help you

		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
		config, err = kubeConfig.ClientConfig()

		if err != nil {
			fmt.Println("Error creating Kubernetes config:", err)
			os.Exit(1)
		}
		if namespace == "" {
			namespace, _, err = kubeConfig.Namespace()

			if err != nil {
				fmt.Println("Error getting namespace:", err)
				os.Exit(1)
			}
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Println("Error creating Kubernetes client:", err)
		os.Exit(1)
	}

	if podName != "" {
		pod, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
		if err != nil {
			fmt.Println("Error getting pod:", err)
			os.Exit(1)
		}
		nodeName = pod.Spec.NodeName
	}

	fmt.Printf("Node Name: %s\n", nodeName)
	if nodeName == "" {
		fmt.Println("Pod is not scheduled on any node")
		os.Exit(1)
	}
	_, err = clientset.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		fmt.Println("Error getting node:", err)
		os.Exit(1)
	}

	listOptions := metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
	}

	pods, err := clientset.CoreV1().Pods("").List(context.TODO(), listOptions)
	if err != nil {
		fmt.Println("Error getting pods:", err)
		os.Exit(1)
	}

	rows := [][]string{}

	for _, pod := range pods.Items {
		containerStatuses := pod.Status.ContainerStatuses
		containersReady := 0
		for _, containerStatus := range containerStatuses {
			if containerStatus.Ready {
				containersReady++
			}
		}

		timeFromStart, err := goment.New(pod.CreationTimestamp.Time.Format("2006-01-02T15:04:05-0700"))
		if err != nil {
			fmt.Println("Error getting pod creation time:", err)
			os.Exit(1)
		}

		fromNow := timeFromStart.FromNow()

		containersLen := len(pod.Spec.Containers)
		rows = append(rows, []string{pod.Namespace, pod.Name, fmt.Sprintf("%d/%d", containersReady, containersLen), fromNow})
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Namespace", "Name", "Containers", "Age"})

	for _, v := range rows {
		table.Append(v)
	}
	table.Render() // Send output
}
