package kube

import (
    "context"
    "fmt"

    batchv1 "k8s.io/api/batch/v1"
    corev1 "k8s.io/api/core/v1"
    meta "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/tools/clientcmd"
    "k8s.io/client-go/util/retry"
)

func int32Ptr(i int32) *int32 { return &i }

// CreateJobForTile creates a Kubernetes Job that:
// 1) downloads the tile from MinIO
// 2) runs filter.wasm on it via Runwasi
// 3) uploads the processed tile back to MinIO
func CreateJobForTile(jobName, tileName, namespace, bucketURL, wasmBucketURL string) error {
    // Load kubeconfig
    cfg, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
    if err != nil {
        return fmt.Errorf("loading kubeconfig: %w", err)
    }
    clientset, err := kubernetes.NewForConfig(cfg)
    if err != nil {
        return fmt.Errorf("building clientset: %w", err)
    }

    // Job spec
    job := &batchv1.Job{
        ObjectMeta: meta.ObjectMeta{
            Name:      jobName,
            Namespace: namespace,
            Labels:    map[string]string{"app": "wasm-tile-processor"},
        },
        Spec: batchv1.JobSpec{
            BackoffLimit: int32Ptr(1),
            Template: corev1.PodTemplateSpec{
                ObjectMeta: meta.ObjectMeta{
                    Labels: map[string]string{"job-name": jobName},
                },
                Spec: corev1.PodSpec{
                    RestartPolicy: corev1.RestartPolicyOnFailure,

                    // 1) InitContainer to download filter.wasm
                    InitContainers: []corev1.Container{{
                        Name:  "init-wasm",
                        Image: "curlimages/curl:7.85.0",
                        Command: []string{
                            "sh", "-c",
                            fmt.Sprintf(
                                "mkdir -p /opt/filter && "+
                                "curl -s %s/filter.wasm -o /opt/filter/filter.wasm && "+
                                "ls -l /opt/filter && echo \"WASM fetched!\"",
                                wasmBucketURL,
                            ),
                        },
                        VolumeMounts: []corev1.VolumeMount{{
                            Name:      "wasm-volume",
                            MountPath: "/opt/filter",
                        }},
                    }},

                    // 2) Main processing container
                    Containers: []corev1.Container{{
                        Name:  "processor",
                        Image: "ghcr.io/phantominthewire/image-pipeline:latest",
                        Command: []string{
                            "sh", "-c",
                            fmt.Sprintf(
                                "curl -s %s/%s | runwasi /opt/filter/filter.wasm > /tmp/out.png && "+
                                "curl -X PUT -T /tmp/out.png %s/processed/%s",
                                bucketURL, tileName,
                                bucketURL, tileName,
                            ),
                        },
                        Env: []corev1.EnvVar{
                            {Name: "INPUT_URL", Value: fmt.Sprintf("%s/%s", bucketURL, tileName)},
                            {Name: "OUTPUT_URL",Value: fmt.Sprintf("%s/processed/%s", bucketURL, tileName)},
                        },
                        VolumeMounts: []corev1.VolumeMount{{
                            Name:      "wasm-volume",
                            MountPath: "/opt/filter",
                        }},
                    }},

                    // 3) Shared emptyDir for the wasm artifact
                    Volumes: []corev1.Volume{{
                        Name: "wasm-volume",
                        VolumeSource: corev1.VolumeSource{
                            EmptyDir: &corev1.EmptyDirVolumeSource{},
                        },
                    }},
                },
            },
        },
    }

    // Create with retry
    return retry.RetryOnConflict(retry.DefaultRetry, func() error {
        _, err := clientset.BatchV1().Jobs(namespace).Create(context.Background(), job, meta.CreateOptions{})
        return err
    })
}
