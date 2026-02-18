package kube

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
)

type ResourceDrillDown struct {
	Describe string
	Related  string
	Logs     string
}

func GetResourceDrillDown(
	ctx context.Context,
	clientset *kubernetes.Clientset,
	namespace string,
	kind string,
	name string,
) ResourceDrillDown {
	res := ResourceDrillDown{
		Describe: "No describe information available.",
		Related:  "No related resources found.",
		Logs:     "No logs available for this resource.",
	}

	if clientset == nil {
		res.Describe = "Kubernetes client is not available."
		return res
	}

	normalizedKind := strings.ToLower(strings.TrimSpace(kind))
	resourceName := strings.TrimSpace(name)
	if normalizedKind == "" || resourceName == "" {
		res.Describe = "Resource kind/name is not available."
		return res
	}

	resourceNamespace := namespace
	if resourceNamespace == "" && isNamespacedKind(normalizedKind) {
		resourceNamespace = metav1.NamespaceDefault
	}

	var logPod string
	switch normalizedKind {
	case "pod":
		res.Describe = describePod(ctx, clientset, resourceNamespace, resourceName)
		res.Related, logPod = relatedForPod(ctx, clientset, resourceNamespace, resourceName)
	case "deployment":
		res.Describe = describeDeployment(ctx, clientset, resourceNamespace, resourceName)
		res.Related, logPod = relatedForDeployment(ctx, clientset, resourceNamespace, resourceName)
	case "replicaset":
		res.Describe = describeReplicaSet(ctx, clientset, resourceNamespace, resourceName)
		res.Related, logPod = relatedForReplicaSet(ctx, clientset, resourceNamespace, resourceName)
	case "statefulset":
		res.Describe = describeStatefulSet(ctx, clientset, resourceNamespace, resourceName)
		res.Related, logPod = relatedForStatefulSet(ctx, clientset, resourceNamespace, resourceName)
	case "daemonset":
		res.Describe = describeDaemonSet(ctx, clientset, resourceNamespace, resourceName)
		res.Related, logPod = relatedForDaemonSet(ctx, clientset, resourceNamespace, resourceName)
	case "job":
		res.Describe = describeJob(ctx, clientset, resourceNamespace, resourceName)
		res.Related, logPod = relatedForJob(ctx, clientset, resourceNamespace, resourceName)
	case "cronjob":
		res.Describe = describeCronJob(ctx, clientset, resourceNamespace, resourceName)
		res.Related, logPod = relatedForCronJob(ctx, clientset, resourceNamespace, resourceName)
	case "service":
		res.Describe = describeService(ctx, clientset, resourceNamespace, resourceName)
		res.Related, logPod = relatedForService(ctx, clientset, resourceNamespace, resourceName)
	case "node":
		res.Describe = describeNode(ctx, clientset, resourceName)
		res.Related = relatedForNode(ctx, clientset, resourceName)
	default:
		res.Describe = fmt.Sprintf("No describe adapter for kind %q.", kind)
		res.Related = "No related adapter for this resource kind yet."
	}

	if logPod != "" {
		res.Logs = podLogs(ctx, clientset, resourceNamespace, logPod)
	}

	eventsSummary := recentObjectEvents(ctx, clientset, namespace, kind, resourceName)
	if eventsSummary != "" {
		res.Describe = strings.TrimSpace(res.Describe) + "\n\nRecent object events:\n" + eventsSummary
	}

	return res
}

func isNamespacedKind(kind string) bool {
	switch kind {
	case "node", "namespace", "persistentvolume":
		return false
	default:
		return true
	}
}

func describePod(ctx context.Context, clientset *kubernetes.Clientset, namespace, name string) string {
	pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Sprintf("Failed to load pod: %v", err)
	}

	lines := []string{
		fmt.Sprintf("Kind: Pod"),
		fmt.Sprintf("Name: %s", pod.Name),
		fmt.Sprintf("Namespace: %s", pod.Namespace),
		fmt.Sprintf("Phase: %s", pod.Status.Phase),
		fmt.Sprintf("Node: %s", pod.Spec.NodeName),
		fmt.Sprintf("Pod IP: %s", pod.Status.PodIP),
		fmt.Sprintf("Host IP: %s", pod.Status.HostIP),
	}
	if pod.Status.StartTime != nil {
		lines = append(lines, fmt.Sprintf("Started: %s", pod.Status.StartTime.Time.Format(time.RFC3339)))
	}
	if len(pod.OwnerReferences) > 0 {
		owners := make([]string, 0, len(pod.OwnerReferences))
		for _, ref := range pod.OwnerReferences {
			owners = append(owners, fmt.Sprintf("%s/%s", ref.Kind, ref.Name))
		}
		lines = append(lines, "Owners: "+strings.Join(owners, ", "))
	}
	if len(pod.Status.ContainerStatuses) > 0 {
		lines = append(lines, "Containers:")
		for _, cs := range pod.Status.ContainerStatuses {
			lines = append(lines, fmt.Sprintf(
				"- %s ready=%t restarts=%d image=%s",
				cs.Name, cs.Ready, cs.RestartCount, trimString(cs.Image, 70),
			))
		}
	}
	return strings.Join(lines, "\n")
}

func describeDeployment(ctx context.Context, clientset *kubernetes.Clientset, namespace, name string) string {
	dep, err := clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Sprintf("Failed to load deployment: %v", err)
	}
	desired := int32(1)
	if dep.Spec.Replicas != nil {
		desired = *dep.Spec.Replicas
	}
	lines := []string{
		"Kind: Deployment",
		fmt.Sprintf("Name: %s", dep.Name),
		fmt.Sprintf("Namespace: %s", dep.Namespace),
		fmt.Sprintf("Selector: %s", metav1.FormatLabelSelector(dep.Spec.Selector)),
		fmt.Sprintf(
			"Replicas: desired=%d updated=%d ready=%d available=%d unavailable=%d",
			desired, dep.Status.UpdatedReplicas, dep.Status.ReadyReplicas, dep.Status.AvailableReplicas, dep.Status.UnavailableReplicas,
		),
		fmt.Sprintf("Strategy: %s", dep.Spec.Strategy.Type),
	}
	return strings.Join(lines, "\n")
}

func describeReplicaSet(ctx context.Context, clientset *kubernetes.Clientset, namespace, name string) string {
	rs, err := clientset.AppsV1().ReplicaSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Sprintf("Failed to load replicaset: %v", err)
	}
	desired := int32(1)
	if rs.Spec.Replicas != nil {
		desired = *rs.Spec.Replicas
	}
	lines := []string{
		"Kind: ReplicaSet",
		fmt.Sprintf("Name: %s", rs.Name),
		fmt.Sprintf("Namespace: %s", rs.Namespace),
		fmt.Sprintf("Selector: %s", metav1.FormatLabelSelector(rs.Spec.Selector)),
		fmt.Sprintf("Replicas: desired=%d ready=%d available=%d", desired, rs.Status.ReadyReplicas, rs.Status.AvailableReplicas),
	}
	return strings.Join(lines, "\n")
}

func describeStatefulSet(ctx context.Context, clientset *kubernetes.Clientset, namespace, name string) string {
	sts, err := clientset.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Sprintf("Failed to load statefulset: %v", err)
	}
	desired := int32(1)
	if sts.Spec.Replicas != nil {
		desired = *sts.Spec.Replicas
	}
	lines := []string{
		"Kind: StatefulSet",
		fmt.Sprintf("Name: %s", sts.Name),
		fmt.Sprintf("Namespace: %s", sts.Namespace),
		fmt.Sprintf("Service: %s", sts.Spec.ServiceName),
		fmt.Sprintf("Selector: %s", metav1.FormatLabelSelector(sts.Spec.Selector)),
		fmt.Sprintf("Replicas: desired=%d ready=%d current=%d updated=%d", desired, sts.Status.ReadyReplicas, sts.Status.CurrentReplicas, sts.Status.UpdatedReplicas),
	}
	return strings.Join(lines, "\n")
}

func describeDaemonSet(ctx context.Context, clientset *kubernetes.Clientset, namespace, name string) string {
	ds, err := clientset.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Sprintf("Failed to load daemonset: %v", err)
	}
	lines := []string{
		"Kind: DaemonSet",
		fmt.Sprintf("Name: %s", ds.Name),
		fmt.Sprintf("Namespace: %s", ds.Namespace),
		fmt.Sprintf("Selector: %s", metav1.FormatLabelSelector(ds.Spec.Selector)),
		fmt.Sprintf(
			"Pods: desired=%d current=%d ready=%d updated=%d available=%d",
			ds.Status.DesiredNumberScheduled,
			ds.Status.CurrentNumberScheduled,
			ds.Status.NumberReady,
			ds.Status.UpdatedNumberScheduled,
			ds.Status.NumberAvailable,
		),
	}
	return strings.Join(lines, "\n")
}

func describeJob(ctx context.Context, clientset *kubernetes.Clientset, namespace, name string) string {
	job, err := clientset.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Sprintf("Failed to load job: %v", err)
	}
	lines := []string{
		"Kind: Job",
		fmt.Sprintf("Name: %s", job.Name),
		fmt.Sprintf("Namespace: %s", job.Namespace),
		fmt.Sprintf("Completions: %d", valueOrDefault(job.Spec.Completions)),
		fmt.Sprintf("Parallelism: %d", valueOrDefault(job.Spec.Parallelism)),
		fmt.Sprintf("Status: active=%d succeeded=%d failed=%d", job.Status.Active, job.Status.Succeeded, job.Status.Failed),
	}
	if job.Status.StartTime != nil {
		lines = append(lines, fmt.Sprintf("Started: %s", job.Status.StartTime.Time.Format(time.RFC3339)))
	}
	if job.Status.CompletionTime != nil {
		lines = append(lines, fmt.Sprintf("Completed: %s", job.Status.CompletionTime.Time.Format(time.RFC3339)))
	}
	return strings.Join(lines, "\n")
}

func describeCronJob(ctx context.Context, clientset *kubernetes.Clientset, namespace, name string) string {
	cron, err := clientset.BatchV1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Sprintf("Failed to load cronjob: %v", err)
	}
	lines := []string{
		"Kind: CronJob",
		fmt.Sprintf("Name: %s", cron.Name),
		fmt.Sprintf("Namespace: %s", cron.Namespace),
		fmt.Sprintf("Schedule: %s", cron.Spec.Schedule),
		fmt.Sprintf("Suspend: %t", boolOrDefault(cron.Spec.Suspend)),
		fmt.Sprintf("ConcurrencyPolicy: %s", cron.Spec.ConcurrencyPolicy),
	}
	if cron.Status.LastScheduleTime != nil {
		lines = append(lines, fmt.Sprintf("Last scheduled: %s", cron.Status.LastScheduleTime.Time.Format(time.RFC3339)))
	}
	return strings.Join(lines, "\n")
}

func describeService(ctx context.Context, clientset *kubernetes.Clientset, namespace, name string) string {
	svc, err := clientset.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Sprintf("Failed to load service: %v", err)
	}
	lines := []string{
		"Kind: Service",
		fmt.Sprintf("Name: %s", svc.Name),
		fmt.Sprintf("Namespace: %s", svc.Namespace),
		fmt.Sprintf("Type: %s", svc.Spec.Type),
		fmt.Sprintf("ClusterIP: %s", svc.Spec.ClusterIP),
	}
	if len(svc.Spec.Selector) > 0 {
		pairs := make([]string, 0, len(svc.Spec.Selector))
		for k, v := range svc.Spec.Selector {
			pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
		}
		sort.Strings(pairs)
		lines = append(lines, "Selector: "+strings.Join(pairs, ", "))
	}
	if len(svc.Spec.Ports) > 0 {
		lines = append(lines, "Ports:")
		for _, p := range svc.Spec.Ports {
			lines = append(lines, fmt.Sprintf("- %s %d->%s/%s", p.Name, p.Port, p.TargetPort.String(), p.Protocol))
		}
	}
	return strings.Join(lines, "\n")
}

func describeNode(ctx context.Context, clientset *kubernetes.Clientset, name string) string {
	node, err := clientset.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Sprintf("Failed to load node: %v", err)
	}
	lines := []string{
		"Kind: Node",
		fmt.Sprintf("Name: %s", node.Name),
		fmt.Sprintf("Kubelet: %s", node.Status.NodeInfo.KubeletVersion),
		fmt.Sprintf("Container Runtime: %s", node.Status.NodeInfo.ContainerRuntimeVersion),
		fmt.Sprintf("OS Image: %s", node.Status.NodeInfo.OSImage),
		fmt.Sprintf("Kernel: %s", node.Status.NodeInfo.KernelVersion),
	}

	cond := make([]string, 0, len(node.Status.Conditions))
	for _, c := range node.Status.Conditions {
		if c.Status == corev1.ConditionTrue {
			cond = append(cond, string(c.Type))
		}
	}
	if len(cond) > 0 {
		sort.Strings(cond)
		lines = append(lines, "Healthy conditions: "+strings.Join(cond, ", "))
	}
	return strings.Join(lines, "\n")
}

func relatedForPod(ctx context.Context, clientset *kubernetes.Clientset, namespace, name string) (string, string) {
	pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Sprintf("Failed to load pod relationship: %v", err), ""
	}

	lines := []string{fmt.Sprintf("Pod: %s", pod.Name)}
	if len(pod.OwnerReferences) > 0 {
		for _, ref := range pod.OwnerReferences {
			lines = append(lines, fmt.Sprintf("Owner: %s/%s", ref.Kind, ref.Name))
			if ref.Kind == "ReplicaSet" {
				rs, rsErr := clientset.AppsV1().ReplicaSets(namespace).Get(ctx, ref.Name, metav1.GetOptions{})
				if rsErr == nil {
					if dep := ownerName(rs.OwnerReferences, "Deployment"); dep != "" {
						lines = append(lines, "Deployment: "+dep)
					}
				}
			}
		}
	}
	return strings.Join(lines, "\n"), pod.Name
}

func relatedForDeployment(ctx context.Context, clientset *kubernetes.Clientset, namespace, name string) (string, string) {
	dep, err := clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Sprintf("Failed to load deployment relationship: %v", err), ""
	}

	lines := []string{
		fmt.Sprintf("Deployment: %s", dep.Name),
		fmt.Sprintf("Selector: %s", metav1.FormatLabelSelector(dep.Spec.Selector)),
	}
	rsList, err := clientset.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		replicaSets := make([]string, 0)
		for _, rs := range rsList.Items {
			if ownerName(rs.OwnerReferences, "Deployment") == dep.Name {
				replicaSets = append(replicaSets, rs.Name)
			}
		}
		sort.Strings(replicaSets)
		if len(replicaSets) > 0 {
			lines = append(lines, "ReplicaSets: "+strings.Join(replicaSets, ", "))
		}
	}

	pods, podErr := listPodsBySelector(ctx, clientset, namespace, metav1.FormatLabelSelector(dep.Spec.Selector))
	if podErr != nil {
		lines = append(lines, fmt.Sprintf("Pods: failed to list (%v)", podErr))
		return strings.Join(lines, "\n"), ""
	}
	lines = append(lines, summarizePods(pods)...)
	return strings.Join(lines, "\n"), pickPodForLogs(pods)
}

func relatedForReplicaSet(ctx context.Context, clientset *kubernetes.Clientset, namespace, name string) (string, string) {
	rs, err := clientset.AppsV1().ReplicaSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Sprintf("Failed to load replicaset relationship: %v", err), ""
	}
	lines := []string{
		fmt.Sprintf("ReplicaSet: %s", rs.Name),
		fmt.Sprintf("Selector: %s", metav1.FormatLabelSelector(rs.Spec.Selector)),
	}
	if dep := ownerName(rs.OwnerReferences, "Deployment"); dep != "" {
		lines = append(lines, "Deployment: "+dep)
	}
	pods, podErr := listPodsBySelector(ctx, clientset, namespace, metav1.FormatLabelSelector(rs.Spec.Selector))
	if podErr != nil {
		lines = append(lines, fmt.Sprintf("Pods: failed to list (%v)", podErr))
		return strings.Join(lines, "\n"), ""
	}
	lines = append(lines, summarizePods(pods)...)
	return strings.Join(lines, "\n"), pickPodForLogs(pods)
}

func relatedForStatefulSet(ctx context.Context, clientset *kubernetes.Clientset, namespace, name string) (string, string) {
	sts, err := clientset.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Sprintf("Failed to load statefulset relationship: %v", err), ""
	}
	lines := []string{
		fmt.Sprintf("StatefulSet: %s", sts.Name),
		fmt.Sprintf("Selector: %s", metav1.FormatLabelSelector(sts.Spec.Selector)),
	}
	pods, podErr := listPodsBySelector(ctx, clientset, namespace, metav1.FormatLabelSelector(sts.Spec.Selector))
	if podErr != nil {
		lines = append(lines, fmt.Sprintf("Pods: failed to list (%v)", podErr))
		return strings.Join(lines, "\n"), ""
	}
	lines = append(lines, summarizePods(pods)...)
	return strings.Join(lines, "\n"), pickPodForLogs(pods)
}

func relatedForDaemonSet(ctx context.Context, clientset *kubernetes.Clientset, namespace, name string) (string, string) {
	ds, err := clientset.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Sprintf("Failed to load daemonset relationship: %v", err), ""
	}
	lines := []string{
		fmt.Sprintf("DaemonSet: %s", ds.Name),
		fmt.Sprintf("Selector: %s", metav1.FormatLabelSelector(ds.Spec.Selector)),
	}
	pods, podErr := listPodsBySelector(ctx, clientset, namespace, metav1.FormatLabelSelector(ds.Spec.Selector))
	if podErr != nil {
		lines = append(lines, fmt.Sprintf("Pods: failed to list (%v)", podErr))
		return strings.Join(lines, "\n"), ""
	}
	lines = append(lines, summarizePods(pods)...)
	return strings.Join(lines, "\n"), pickPodForLogs(pods)
}

func relatedForJob(ctx context.Context, clientset *kubernetes.Clientset, namespace, name string) (string, string) {
	job, err := clientset.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Sprintf("Failed to load job relationship: %v", err), ""
	}
	lines := []string{fmt.Sprintf("Job: %s", job.Name)}
	pods, podErr := podsForJob(ctx, clientset, namespace, job)
	if podErr != nil {
		lines = append(lines, fmt.Sprintf("Pods: failed to list (%v)", podErr))
		return strings.Join(lines, "\n"), ""
	}
	lines = append(lines, summarizePods(pods)...)
	return strings.Join(lines, "\n"), pickPodForLogs(pods)
}

func relatedForCronJob(ctx context.Context, clientset *kubernetes.Clientset, namespace, name string) (string, string) {
	cron, err := clientset.BatchV1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Sprintf("Failed to load cronjob relationship: %v", err), ""
	}
	lines := []string{fmt.Sprintf("CronJob: %s", cron.Name)}
	jobs, err := clientset.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		matched := make([]batchv1.Job, 0)
		for _, job := range jobs.Items {
			if ownerName(job.OwnerReferences, "CronJob") == cron.Name {
				matched = append(matched, job)
			}
		}
		sort.Slice(matched, func(i, j int) bool {
			return matched[i].CreationTimestamp.Time.After(matched[j].CreationTimestamp.Time)
		})
		if len(matched) > 0 {
			lines = append(lines, "Recent Jobs:")
			limit := 5
			if len(matched) < limit {
				limit = len(matched)
			}
			for _, job := range matched[:limit] {
				lines = append(lines, fmt.Sprintf("- %s active=%d succeeded=%d failed=%d", job.Name, job.Status.Active, job.Status.Succeeded, job.Status.Failed))
			}
			pods, podErr := podsForJob(ctx, clientset, namespace, &matched[0])
			if podErr == nil {
				lines = append(lines, summarizePods(pods)...)
				return strings.Join(lines, "\n"), pickPodForLogs(pods)
			}
		}
	}
	return strings.Join(lines, "\n"), ""
}

func relatedForService(ctx context.Context, clientset *kubernetes.Clientset, namespace, name string) (string, string) {
	svc, err := clientset.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Sprintf("Failed to load service relationship: %v", err), ""
	}
	lines := []string{fmt.Sprintf("Service: %s", svc.Name)}
	if len(svc.Spec.Selector) == 0 {
		lines = append(lines, "No selector configured.")
		return strings.Join(lines, "\n"), ""
	}
	selectorParts := make([]string, 0, len(svc.Spec.Selector))
	for k, v := range svc.Spec.Selector {
		selectorParts = append(selectorParts, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(selectorParts)
	selector := strings.Join(selectorParts, ",")
	lines = append(lines, "Selector: "+selector)
	pods, podErr := listPodsBySelector(ctx, clientset, namespace, selector)
	if podErr != nil {
		lines = append(lines, fmt.Sprintf("Pods: failed to list (%v)", podErr))
		return strings.Join(lines, "\n"), ""
	}
	lines = append(lines, summarizePods(pods)...)
	return strings.Join(lines, "\n"), pickPodForLogs(pods)
}

func relatedForNode(ctx context.Context, clientset *kubernetes.Clientset, nodeName string) string {
	pods, err := clientset.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("spec.nodeName", nodeName).String(),
	})
	if err != nil {
		return fmt.Sprintf("Failed to load pods on node: %v", err)
	}
	lines := []string{fmt.Sprintf("Node: %s", nodeName)}
	if len(pods.Items) == 0 {
		lines = append(lines, "No pods scheduled on this node.")
		return strings.Join(lines, "\n")
	}
	lines = append(lines, "Pods on node:")
	sorted := append([]corev1.Pod(nil), pods.Items...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	limit := 10
	if len(sorted) < limit {
		limit = len(sorted)
	}
	for _, pod := range sorted[:limit] {
		lines = append(lines, fmt.Sprintf("- %s/%s (%s)", pod.Namespace, pod.Name, pod.Status.Phase))
	}
	if len(sorted) > limit {
		lines = append(lines, fmt.Sprintf("... +%d more", len(sorted)-limit))
	}
	return strings.Join(lines, "\n")
}

func recentObjectEvents(ctx context.Context, clientset *kubernetes.Clientset, namespace, kind, name string) string {
	if strings.TrimSpace(name) == "" || strings.TrimSpace(kind) == "" {
		return ""
	}
	eventNamespace := namespace
	if eventNamespace == "" {
		eventNamespace = metav1.NamespaceAll
	}
	selector := fields.AndSelectors(
		fields.OneTermEqualSelector("involvedObject.name", strings.TrimSpace(name)),
		fields.OneTermEqualSelector("involvedObject.kind", strings.TrimSpace(kind)),
	).String()
	events, err := clientset.CoreV1().Events(eventNamespace).List(ctx, metav1.ListOptions{
		FieldSelector: selector,
	})
	if err != nil || len(events.Items) == 0 {
		return ""
	}

	sorted := append([]corev1.Event(nil), events.Items...)
	sort.Slice(sorted, func(i, j int) bool {
		return eventTimestamp(sorted[i]).After(eventTimestamp(sorted[j]))
	})
	limit := 6
	if len(sorted) < limit {
		limit = len(sorted)
	}
	lines := make([]string, 0, limit)
	for _, event := range sorted[:limit] {
		lines = append(lines, fmt.Sprintf(
			"- %s %s/%s: %s",
			eventTimestamp(event).Format("15:04:05"),
			event.Type,
			event.Reason,
			trimString(event.Message, 140),
		))
	}
	return strings.Join(lines, "\n")
}

func podsForJob(ctx context.Context, clientset *kubernetes.Clientset, namespace string, job *batchv1.Job) ([]corev1.Pod, error) {
	if job.Spec.Selector == nil {
		return []corev1.Pod{}, nil
	}
	return listPodsBySelector(ctx, clientset, namespace, metav1.FormatLabelSelector(job.Spec.Selector))
}

func listPodsBySelector(
	ctx context.Context,
	clientset *kubernetes.Clientset,
	namespace string,
	selector string,
) ([]corev1.Pod, error) {
	if strings.TrimSpace(selector) == "" {
		return []corev1.Pod{}, nil
	}
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return nil, err
	}
	sorted := append([]corev1.Pod(nil), pods.Items...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Status.Phase != sorted[j].Status.Phase {
			return sorted[i].Status.Phase == corev1.PodRunning
		}
		return sorted[i].Name < sorted[j].Name
	})
	return sorted, nil
}

func summarizePods(pods []corev1.Pod) []string {
	if len(pods) == 0 {
		return []string{"Pods: none"}
	}
	lines := []string{"Pods:"}
	limit := 8
	if len(pods) < limit {
		limit = len(pods)
	}
	for _, pod := range pods[:limit] {
		lines = append(lines, fmt.Sprintf("- %s (%s)", pod.Name, pod.Status.Phase))
	}
	if len(pods) > limit {
		lines = append(lines, fmt.Sprintf("... +%d more", len(pods)-limit))
	}
	return lines
}

func pickPodForLogs(pods []corev1.Pod) string {
	if len(pods) == 0 {
		return ""
	}
	for _, pod := range pods {
		if pod.Status.Phase == corev1.PodRunning {
			return pod.Name
		}
	}
	return pods[0].Name
}

func podLogs(ctx context.Context, clientset *kubernetes.Clientset, namespace, podName string) string {
	pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Sprintf("Failed to load pod for logs: %v", err)
	}
	container := pickContainerName(pod)
	if container == "" {
		return "Pod has no containers."
	}

	tail := int64(80)
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container:  container,
		TailLines:  &tail,
		Timestamps: true,
	})
	stream, err := req.Stream(ctx)
	if err != nil {
		return fmt.Sprintf("Failed to fetch logs for pod %s (container %s): %v", podName, container, err)
	}
	defer stream.Close()

	data, err := io.ReadAll(io.LimitReader(stream, 64*1024))
	if err != nil {
		return fmt.Sprintf("Failed reading logs stream: %v", err)
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return fmt.Sprintf("No recent logs in pod %s (container %s).", podName, container)
	}
	return fmt.Sprintf("Pod: %s\nContainer: %s\n\n%s", podName, container, text)
}

func pickContainerName(pod *corev1.Pod) string {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Running != nil {
			return cs.Name
		}
	}
	if len(pod.Spec.Containers) > 0 {
		return pod.Spec.Containers[0].Name
	}
	return ""
}

func ownerName(refs []metav1.OwnerReference, kind string) string {
	for _, ref := range refs {
		if ref.Kind == kind {
			return ref.Name
		}
	}
	return ""
}

func valueOrDefault(v *int32) int32 {
	if v == nil {
		return 0
	}
	return *v
}

func boolOrDefault(v *bool) bool {
	if v == nil {
		return false
	}
	return *v
}

func eventTimestamp(event corev1.Event) time.Time {
	if !event.LastTimestamp.IsZero() {
		return event.LastTimestamp.Time
	}
	if !event.EventTime.IsZero() {
		return event.EventTime.Time
	}
	if !event.FirstTimestamp.IsZero() {
		return event.FirstTimestamp.Time
	}
	return event.CreationTimestamp.Time
}

func trimString(s string, limit int) string {
	if limit <= 0 || len(s) <= limit {
		return s
	}
	return s[:limit-3] + "..."
}
