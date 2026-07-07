// Package neighbours lists pods that share a Kubernetes node.
package neighbours

import (
	"context"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/client-go/kubernetes"
)

// now is a hook for tests to fix the clock used for AGE calculation.
var now = time.Now

// PodRow is one rendered line of output.
type PodRow struct {
	Namespace string
	Name      string
	Ready     string
	Status    string
	Age       string
}

// ResolveNode returns the node a pod is scheduled on.
func ResolveNode(ctx context.Context, client kubernetes.Interface, namespace, podName string) (string, error) {
	pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("getting pod %s/%s: %w", namespace, podName, err)
	}
	if pod.Spec.NodeName == "" {
		return "", fmt.Errorf("pod %s/%s is not scheduled on any node", namespace, podName)
	}
	return pod.Spec.NodeName, nil
}

// ListNeighbours returns a row for every pod scheduled on the given node,
// across all namespaces.
func ListNeighbours(ctx context.Context, client kubernetes.Interface, nodeName string) ([]PodRow, error) {
	pods, err := client.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
	})
	if err != nil {
		return nil, fmt.Errorf("listing pods on node %s: %w", nodeName, err)
	}

	rows := make([]PodRow, 0, len(pods.Items))
	for i := range pods.Items {
		rows = append(rows, newPodRow(&pods.Items[i]))
	}
	return rows, nil
}

func newPodRow(pod *corev1.Pod) PodRow {
	ready := 0
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Ready {
			ready++
		}
	}
	return PodRow{
		Namespace: pod.Namespace,
		Name:      pod.Name,
		Ready:     fmt.Sprintf("%d/%d", ready, len(pod.Spec.Containers)),
		Status:    podStatus(pod),
		Age:       duration.HumanDuration(now().Sub(pod.CreationTimestamp.Time)),
	}
}

// podStatus mirrors the STATUS column kubectl shows for common cases.
func podStatus(pod *corev1.Pod) string {
	if pod.DeletionTimestamp != nil {
		return "Terminating"
	}
	if pod.Status.Reason != "" {
		return pod.Status.Reason
	}
	return string(pod.Status.Phase)
}

// Render writes the rows as an aligned, kubectl-style table.
func Render(w io.Writer, rows []PodRow) error {
	tw := tabwriter.NewWriter(w, 0, 8, 3, ' ', 0)
	fmt.Fprintln(tw, "NAMESPACE\tNAME\tREADY\tSTATUS\tAGE")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.Namespace, r.Name, r.Ready, r.Status, r.Age)
	}
	return tw.Flush()
}
