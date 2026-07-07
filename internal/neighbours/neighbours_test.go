package neighbours

import (
	"context"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

var testNow = time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)

// newFakeClient returns a fake clientset whose pod List honors the
// spec.nodeName field selector, which the stock fake client ignores.
func newFakeClient(objs ...runtime.Object) *fake.Clientset {
	client := fake.NewSimpleClientset(objs...)
	client.PrependReactor("list", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		listAction := action.(k8stesting.ListAction)
		selector := listAction.GetListRestrictions().Fields

		obj, err := client.Tracker().List(
			corev1.SchemeGroupVersion.WithResource("pods"),
			corev1.SchemeGroupVersion.WithKind("Pod"),
			listAction.GetNamespace(),
		)
		if err != nil {
			return true, nil, err
		}

		podList := obj.(*corev1.PodList)
		filtered := &corev1.PodList{}
		for _, pod := range podList.Items {
			if selector.Matches(fields.Set{"spec.nodeName": pod.Spec.NodeName}) {
				filtered.Items = append(filtered.Items, pod)
			}
		}
		return true, filtered, nil
	})
	return client
}

func makePod(namespace, name, node string, opts ...func(*corev1.Pod)) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:         namespace,
			Name:              name,
			CreationTimestamp: metav1.NewTime(testNow.Add(-5 * time.Minute)),
		},
		Spec: corev1.PodSpec{
			NodeName:   node,
			Containers: []corev1.Container{{Name: "app"}},
		},
		Status: corev1.PodStatus{
			Phase:             corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{Name: "app", Ready: true}},
		},
	}
	for _, opt := range opts {
		opt(pod)
	}
	return pod
}

func TestResolveNode(t *testing.T) {
	tests := []struct {
		name     string
		pods     []runtime.Object
		podName  string
		wantNode string
		wantErr  string
	}{
		{
			name:     "scheduled pod",
			pods:     []runtime.Object{makePod("default", "my-pod", "node-1")},
			podName:  "my-pod",
			wantNode: "node-1",
		},
		{
			name:    "pod not found",
			podName: "missing",
			wantErr: "getting pod default/missing",
		},
		{
			name:    "unscheduled pod",
			pods:    []runtime.Object{makePod("default", "pending-pod", "")},
			podName: "pending-pod",
			wantErr: "not scheduled on any node",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newFakeClient(tt.pods...)
			node, err := ResolveNode(context.Background(), client, "default", tt.podName)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ResolveNode() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveNode() unexpected error: %v", err)
			}
			if node != tt.wantNode {
				t.Fatalf("ResolveNode() = %q, want %q", node, tt.wantNode)
			}
		})
	}
}

func TestListNeighboursFiltersByNode(t *testing.T) {
	origNow := now
	now = func() time.Time { return testNow }
	t.Cleanup(func() { now = origNow })

	client := newFakeClient(
		makePod("default", "on-node-a", "node-a"),
		makePod("kube-system", "also-on-node-a", "node-a"),
		makePod("default", "on-node-b", "node-b"),
	)

	rows, err := ListNeighbours(context.Background(), client, "node-a")
	if err != nil {
		t.Fatalf("ListNeighbours() unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("ListNeighbours() returned %d rows, want 2: %+v", len(rows), rows)
	}
	for _, row := range rows {
		if row.Name == "on-node-b" {
			t.Fatalf("pod from node-b leaked into results: %+v", row)
		}
	}
	if rows[0].Namespace != "default" && rows[1].Namespace != "default" {
		t.Fatalf("expected pods from multiple namespaces, got %+v", rows)
	}
}

func TestNewPodRow(t *testing.T) {
	origNow := now
	now = func() time.Time { return testNow }
	t.Cleanup(func() { now = origNow })

	deletionTime := metav1.NewTime(testNow)

	tests := []struct {
		name       string
		pod        *corev1.Pod
		wantReady  string
		wantStatus string
		wantAge    string
	}{
		{
			name:       "running all ready",
			pod:        makePod("default", "p", "n"),
			wantReady:  "1/1",
			wantStatus: "Running",
			wantAge:    "5m",
		},
		{
			name: "mixed ready containers",
			pod: makePod("default", "p", "n", func(p *corev1.Pod) {
				p.Spec.Containers = []corev1.Container{{Name: "a"}, {Name: "b"}, {Name: "c"}}
				p.Status.ContainerStatuses = []corev1.ContainerStatus{
					{Name: "a", Ready: true},
					{Name: "b", Ready: false},
					{Name: "c", Ready: true},
				}
			}),
			wantReady:  "2/3",
			wantStatus: "Running",
			wantAge:    "5m",
		},
		{
			name: "pending pod",
			pod: makePod("default", "p", "n", func(p *corev1.Pod) {
				p.Status.Phase = corev1.PodPending
				p.Status.ContainerStatuses = nil
			}),
			wantReady:  "0/1",
			wantStatus: "Pending",
			wantAge:    "5m",
		},
		{
			name: "succeeded pod",
			pod: makePod("default", "p", "n", func(p *corev1.Pod) {
				p.Status.Phase = corev1.PodSucceeded
				p.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "app", Ready: false}}
			}),
			wantReady:  "0/1",
			wantStatus: "Succeeded",
			wantAge:    "5m",
		},
		{
			name: "terminating pod",
			pod: makePod("default", "p", "n", func(p *corev1.Pod) {
				p.DeletionTimestamp = &deletionTime
			}),
			wantReady:  "1/1",
			wantStatus: "Terminating",
			wantAge:    "5m",
		},
		{
			name: "status reason wins over phase",
			pod: makePod("default", "p", "n", func(p *corev1.Pod) {
				p.Status.Phase = corev1.PodFailed
				p.Status.Reason = "Evicted"
			}),
			wantReady:  "1/1",
			wantStatus: "Evicted",
			wantAge:    "5m",
		},
		{
			name: "old pod age",
			pod: makePod("default", "p", "n", func(p *corev1.Pod) {
				p.CreationTimestamp = metav1.NewTime(testNow.Add(-50 * time.Hour))
			}),
			wantReady:  "1/1",
			wantStatus: "Running",
			wantAge:    "2d2h",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := newPodRow(tt.pod)
			if row.Ready != tt.wantReady {
				t.Errorf("Ready = %q, want %q", row.Ready, tt.wantReady)
			}
			if row.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", row.Status, tt.wantStatus)
			}
			if row.Age != tt.wantAge {
				t.Errorf("Age = %q, want %q", row.Age, tt.wantAge)
			}
		})
	}
}

func TestRender(t *testing.T) {
	rows := []PodRow{
		{Namespace: "default", Name: "web-6d4b75cb6d-abcde", Ready: "1/1", Status: "Running", Age: "5m"},
		{Namespace: "kube-system", Name: "kube-proxy-xyz", Ready: "1/1", Status: "Running", Age: "2d2h"},
	}

	var sb strings.Builder
	if err := Render(&sb, rows); err != nil {
		t.Fatalf("Render() unexpected error: %v", err)
	}

	want := "" +
		"NAMESPACE     NAME                   READY   STATUS    AGE\n" +
		"default       web-6d4b75cb6d-abcde   1/1     Running   5m\n" +
		"kube-system   kube-proxy-xyz         1/1     Running   2d2h\n"
	if got := sb.String(); got != want {
		t.Errorf("Render() output mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}
