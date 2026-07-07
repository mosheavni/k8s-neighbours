// Command k8s-neighbours lists all pods scheduled on the same Kubernetes
// node as a given pod, or on a given node directly.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/mosheavni/k8s-neighbours/internal/neighbours"
)

const inClusterNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

// Populated by GoReleaser via ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run() error {
	podName := flag.String("pod", "", "name of the pod whose node neighbours to list")
	nodeName := flag.String("node", "", "name of the node to list pods from")
	namespace := flag.String("namespace", "", "namespace of the pod (defaults to the current kubeconfig namespace)")
	showVersion := flag.Bool("version", false, "print version information and exit")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s (-pod <pod_name> [-namespace <namespace>] | -node <node_name>)\n\n", os.Args[0])
		fmt.Fprintln(flag.CommandLine.Output(), "List all pods scheduled on the same node as a pod, or on a node directly.")
		fmt.Fprintln(flag.CommandLine.Output(), "\nFlags:")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Printf("k8s-neighbours %s (commit %s, built %s)\n", version, commit, date)
		return nil
	}

	if (*podName == "") == (*nodeName == "") {
		flag.Usage()
		return fmt.Errorf("exactly one of -pod or -node is required")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	config, ns, err := buildConfig(*namespace)
	if err != nil {
		return err
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("creating Kubernetes client: %w", err)
	}

	node := *nodeName
	if *podName != "" {
		node, err = neighbours.ResolveNode(ctx, client, ns, *podName)
		if err != nil {
			return err
		}
	} else if _, err := client.CoreV1().Nodes().Get(ctx, node, metav1.GetOptions{}); err != nil {
		return fmt.Errorf("getting node %s: %w", node, err)
	}

	fmt.Printf("Node: %s\n", node)

	rows, err := neighbours.ListNeighbours(ctx, client, node)
	if err != nil {
		return err
	}
	return neighbours.Render(os.Stdout, rows)
}

// buildConfig returns a rest.Config (in-cluster first, kubeconfig fallback)
// and the namespace to use when one was not passed explicitly.
func buildConfig(namespace string) (*rest.Config, string, error) {
	if config, err := rest.InClusterConfig(); err == nil {
		if namespace == "" {
			namespace = inClusterNamespace()
		}
		return config, namespace, nil
	}

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, "", fmt.Errorf("creating Kubernetes config: %w", err)
	}
	if namespace == "" {
		if namespace, _, err = kubeConfig.Namespace(); err != nil {
			return nil, "", fmt.Errorf("getting namespace from kubeconfig: %w", err)
		}
	}
	return config, namespace, nil
}

func inClusterNamespace() string {
	if data, err := os.ReadFile(inClusterNamespacePath); err == nil {
		if ns := strings.TrimSpace(string(data)); ns != "" {
			return ns
		}
	}
	return metav1.NamespaceDefault
}
