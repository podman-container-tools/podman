# Podman Kube Play Support

This document outlines the kube yaml fields that are currently supported by the **podman kube play** command.

Note: **N/A** means that the option cannot be supported in a single-node Podman environment.

## Pod Fields

| Field                                               | Support |
|-----------------------------------------------------|---------|
| containers                                          | ✅      |
| initContainers                                      | ✅      |
| imagePullSecrets                                    | no      |
| enableServiceLinks                                  | no      |
| os\.name                                            | no      |
| volumes                                             | ✅      |
| nodeSelector                                        | N/A     |
| nodeName                                            | N/A     |
| affinity\.nodeAffinity                              | N/A     |
| affinity\.podAffinity                               | N/A     |
| affinity\.podAntiAffinity                           | N/A     |
| tolerations\.key                                    | N/A     |
| tolerations\.operator                               | N/A     |
| tolerations\.effect                                 | N/A     |
| tolerations\.tolerationSeconds                      | N/A     |
| schedulerName                                       | N/A     |
| runtimeClassName                                    | no      |
| priorityClassName                                   | no      |
| priority                                            | no      |
| topologySpreadConstraints\.maxSkew                  | N/A     |
| topologySpreadConstraints\.topologyKey              | N/A     |
| topologySpreadConstraints\.whenUnsatisfiable        | N/A     |
| topologySpreadConstraints\.labelSelector            | N/A     |
| topologySpreadConstraints\.minDomains               | N/A     |
| restartPolicy                                       | ✅      |
| terminationGracePeriodSeconds                       | ✅      |
| activeDeadlineSeconds                               | no      |
| readinessGates\.conditionType                       | no      |
| hostname                                            | ✅      |
| setHostnameAsFQDN                                   | no      |
| subdomain                                           | no      |
| hostAliases\.hostnames                              | ✅      |
| hostAliases\.ip                                     | ✅      |
| dnsConfig\.nameservers                              | ✅      |
| dnsConfig\.options\.name                            | ✅      |
| dnsConfig\.options\.value                           | ✅      |
| dnsConfig\.searches                                 | ✅      |
| dnsPolicy                                           | no      |
| hostNetwork                                         | ✅      |
| hostPID                                             | ✅      |
| hostIPC                                             | ✅      |
| shareProcessNamespace                               | ✅      |
| serviceAccountName                                  | no      |
| automountServiceAccountToken                        | no      |
| securityContext\.runAsUser                          | ✅      |
| securityContext\.runAsNonRoot                       | no      |
| securityContext\.runAsGroup                         | ✅      |
| securityContext\.supplementalGroups                 | ✅      |
| securityContext\.fsGroup                            | no      |
| securityContext\.fsGroupChangePolicy                | no      |
| securityContext\.seccompProfile\.type               | no      |
| securityContext\.seccompProfile\.localhostProfile   | no      |
| securityContext\.seLinuxOptions\.level              | ✅      |
| securityContext\.seLinuxOptions\.role               | ✅      |
| securityContext\.seLinuxOptions\.type               | ✅      |
| securityContext\.seLinuxOptions\.user               | ✅      |
| securityContext\.sysctls\.name                      | ✅      |
| securityContext\.sysctls\.value                     | ✅      |
| securityContext\.windowsOptions\.gmsaCredentialSpec | no      |
| securityContext\.windowsOptions\.hostProcess        | no      |
| securityContext\.windowsOptions\.runAsUserName      | no      |

## Container Fields

| Field                                               | Support |
|-----------------------------------------------------|---------|
| name                                                | ✅      |
| image                                               | ✅      |
| imagePullPolicy                                     | ✅      |
| command                                             | ✅      |
| args                                                | ✅      |
| workingDir                                          | ✅      |
| ports\.containerPort                                | ✅      |
| ports\.hostIP                                       | ✅      |
| ports\.hostPort                                     | ✅      |
| ports\.name                                         | ✅      |
| ports\.protocol                                     | ✅      |
| env\.name                                           | ✅      |
| env\.value                                          | ✅      |
| env\.valueFrom\.configMapKeyRef\.key                | ✅      |
| env\.valueFrom\.configMapKeyRef\.name               | ✅      |
| env\.valueFrom\.configMapKeyRef\.optional           | ✅      |
| env\.valueFrom\.fieldRef                            | ✅      |
| env\.valueFrom\.resourceFieldRef                    | ✅      |
| env\.valueFrom\.secretKeyRef\.key                   | ✅      |
| env\.valueFrom\.secretKeyRef\.name                  | ✅      |
| env\.valueFrom\.secretKeyRef\.optional              | ✅      |
| envFrom\.configMapRef\.name                         | ✅      |
| envFrom\.configMapRef\.optional                     | ✅      |
| envFrom\.prefix                                     | no      |
| envFrom\.secretRef\.name                            | ✅      |
| envFrom\.secretRef\.optional                        | ✅      |
| volumeMounts\.mountPath                             | ✅      |
| volumeMounts\.name                                  | ✅      |
| volumeMounts\.mountPropagation                      | no      |
| volumeMounts\.readOnly                              | ✅      |
| volumeMounts\.subPath                               | ✅      |
| volumeMounts\.subPathExpr                           | no      |
| volumeDevices\.devicePath                           | no      |
| volumeDevices\.name                                 | no      |
| resources\.limits                                   | ✅      |
| resources\.requests                                 | ✅      |
| lifecycle\.postStart                                | no      |
| lifecycle\.preStop                                  | no      |
| lifecycle\.stopSignal                               | ✅      |
| terminationMessagePath                              | no      |
| terminationMessagePolicy                            | no      |
| livenessProbe                                       | ✅      |
| readinessProbe                                      | no      |
| startupProbe                                        | no      |
| securityContext\.runAsUser                          | ✅      |
| securityContext\.runAsNonRoot                       | no      |
| securityContext\.runAsGroup                         | ✅      |
| securityContext\.readOnlyRootFilesystem             | ✅      |
| securityContext\.procMount                          | ✅      |
| securityContext\.privileged                         | ✅      |
| securityContext\.allowPrivilegeEscalation           | ✅      |
| securityContext\.capabilities\.add                  | ✅      |
| securityContext\.capabilities\.drop                 | ✅      |
| securityContext\.seccompProfile\.type               | no      |
| securityContext\.seccompProfile\.localhostProfile   | no      |
| securityContext\.seLinuxOptions\.level              | ✅      |
| securityContext\.seLinuxOptions\.role               | ✅      |
| securityContext\.seLinuxOptions\.type               | ✅      |
| securityContext\.seLinuxOptions\.user               | ✅      |
| securityContext\.windowsOptions\.gmsaCredentialSpec | no      |
| securityContext\.windowsOptions\.hostProcess        | no      |
| securityContext\.windowsOptions\.runAsUserName      | no      |
| stdin                                               | no      |
| stdinOnce                                           | no      |
| tty                                                 | no      |

## PersistentVolumeClaim Fields

| Field               | Support |
|---------------------|---------|
| volumeName          | no      |
| storageClassName    | ✅      |
| volumeMode          | no      |
| accessModes         | ✅      |
| selector            | no      |
| resources\.limits   | no      |
| resources\.requests | ✅      |

## ConfigMap Fields

| Field      | Support |
|------------|---------|
| binaryData | ✅      |
| data       | ✅      |
| immutable  | no      |

## Deployment Fields

| Field                                   | Support                                               |
|-----------------------------------------|-------------------------------------------------------|
| replicas                                | ✅ (the actual replica count is ignored and set to 1) |
| selector                                | ✅                                                    |
| template                                | ✅                                                    |
| minReadySeconds                         | no                                                    |
| strategy\.type                          | no                                                    |
| strategy\.rollingUpdate\.maxSurge       | no                                                    |
| strategy\.rollingUpdate\.maxUnavailable | no                                                    |
| revisionHistoryLimit                    | no                                                    |
| progressDeadlineSeconds                 | no                                                    |
| paused                                  | no                                                    |

## DaemonSet Fields

| Field                                   | Support |
|-----------------------------------------|---------|
| selector                                | ✅      |
| template                                | ✅      |
| minReadySeconds                         | no      |
| strategy\.type                          | no      |
| strategy\.rollingUpdate\.maxSurge       | no      |
| strategy\.rollingUpdate\.maxUnavailable | no      |
| revisionHistoryLimit                    | no      |

## Job Fields

| Field                   | Support                          |
|-------------------------|----------------------------------|
| activeDeadlineSeconds   | no                               |
| selector                | no (automatically set by k8s)    |
| template                | ✅                               |
| backoffLimit            | no                               |
| completionMode          | no                               |
| completions             | no (set to 1 with kube generate) |
| manualSelector          | no                               |
| parallelism             | no (set to 1 with kube generate) |
| podFailurePolicy        | no                               |
| suspend                 | no                               |
| ttlSecondsAfterFinished | no                               |
