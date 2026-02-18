package kube

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func WatchEvents(ctx context.Context, namespace string, eventHandler func(event *corev1.Event)) error {
	_, _, clientset, _, err := Kinit(namespace)
	if err != nil {
		return fmt.Errorf("initialize kubernetes client: %w", err)
	}

	evList, err := clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("list events: %w", err)
	}
	resourceVersion := evList.ResourceVersion

	watcher, err := clientset.CoreV1().Events(namespace).Watch(ctx, metav1.ListOptions{
		ResourceVersion: resourceVersion,
	})
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("watch events: %w", err)
	}
	defer watcher.Stop()

	ch := watcher.ResultChan()

	for {
		select {
		case <-ctx.Done():
			return nil
		case evt, ok := <-ch:
			if !ok {
				return nil
			}
			if event, ok := evt.Object.(*corev1.Event); ok {
				eventHandler(event)
			}
		}
	}
}
