// Command k8s-neighbours lists all pods scheduled on the same Kubernetes
// node as a given pod, or on a given node directly.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := newRootCmd().ExecuteContext(ctx)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var nodeName, namespace string

	cmd := &cobra.Command{
		Use:   "kubectl-neighbours [pod]",
		Short: "List all pods scheduled on the same node as a pod, or on a node directly",
		Example: `  # pods on the same node as a pod
  kubectl neighbours my-pod-abc123 -n my-namespace

  # pods on a specific node
  kubectl neighbours --node ip-10-0-1-23.ec2.internal`,
		Args:              cobra.MaximumNArgs(1),
		Version:           fmt.Sprintf("%s (commit %s, built %s)", version, commit, date),
		SilenceUsage:      true,
		SilenceErrors:     true,
		ValidArgsFunction: completePods(&namespace),
		RunE: func(cmd *cobra.Command, args []string) error {
			podName := ""
			if len(args) == 1 {
				podName = args[0]
			}
			if (podName == "") == (nodeName == "") {
				return fmt.Errorf("exactly one of a pod name argument or --node is required")
			}
			return run(cmd.Context(), podName, nodeName, namespace)
		},
	}

	cmd.Flags().StringVar(&nodeName, "node", "", "name of the node to list pods from")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace of the pod (defaults to the current kubeconfig namespace)")
	_ = cmd.RegisterFlagCompletionFunc("node", completeNodes)
	_ = cmd.RegisterFlagCompletionFunc("namespace", completeNamespaces)
	cmd.SetVersionTemplate("k8s-neighbours {{.Version}}\n")

	return cmd
}

func run(ctx context.Context, podName, nodeName, namespace string) error {
	client, ns, err := buildClient(namespace)
	if err != nil {
		return err
	}

	node := nodeName
	if podName != "" {
		node, err = neighbours.ResolveNode(ctx, client, ns, podName)
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

// completePods completes the positional pod argument from the live cluster,
// using the --namespace flag value when set.
func completePods(namespace *string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		client, ns, err := buildClient(*namespace)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		pods, err := client.CoreV1().Pods(ns).List(cmd.Context(), metav1.ListOptions{})
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		names := make([]string, 0, len(pods.Items))
		for _, pod := range pods.Items {
			names = append(names, pod.Name)
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	}
}

func completeNodes(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	client, _, err := buildClient("")
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	nodes, err := client.CoreV1().Nodes().List(cmd.Context(), metav1.ListOptions{})
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	names := make([]string, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		names = append(names, node.Name)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

func completeNamespaces(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	client, _, err := buildClient("")
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	namespaces, err := client.CoreV1().Namespaces().List(cmd.Context(), metav1.ListOptions{})
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	names := make([]string, 0, len(namespaces.Items))
	for _, ns := range namespaces.Items {
		names = append(names, ns.Name)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

func buildClient(namespace string) (kubernetes.Interface, string, error) {
	config, ns, err := buildConfig(namespace)
	if err != nil {
		return nil, "", err
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, "", fmt.Errorf("creating Kubernetes client: %w", err)
	}
	return client, ns, nil
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
