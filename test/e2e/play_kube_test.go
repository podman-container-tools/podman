//go:build linux || freebsd

package integration

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	. "github.com/onsi/gomega/gexec"
	"github.com/opencontainers/selinux/go-selinux"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/bindings"
	"go.podman.io/podman/v6/pkg/bindings/play"
	v1 "go.podman.io/podman/v6/pkg/k8s.io/api/core/v1"
	"go.podman.io/podman/v6/pkg/util"
	. "go.podman.io/podman/v6/test/utils"
	"go.podman.io/podman/v6/utils"
	"go.podman.io/storage/pkg/stringid"
)

var secretYaml = `
apiVersion: v1
kind: Secret
metadata:
  name: newsecret
type: Opaque
data:
  username: dXNlcg==
  password: NTRmNDFkMTJlOGZh
`

var secretTxt = `
This secret is not a properly formatted yaml
It will therefore produce an error
`

var complexSecretYaml = `
apiVersion: v1
kind: Secret
metadata:
  name: newsecrettwo
type: Opaque
data:
  username: Y2RvZXJu
  password: dGVzdGluZ3Rlc3RpbmcK
  note: a3ViZSBzZWNyZXRzIGFyZSBjb29sIQo=
stringData:
  plain_note: This is a test
`

var secretPodYaml = `
apiVersion: v1
kind: Pod
metadata:
  name: mypod
spec:
  containers:
    - name: myctr
      image: ` + CITEST_IMAGE + `
      command:
        - top
      volumeMounts:
        - name: foo
          mountPath: /etc/foo
          readOnly: true
  volumes:
    - name: foo
      secret:
        secretName: newsecret
        optional: false
`

var secretPodYamlTwo = `
apiVersion: v1
kind: Pod
metadata:
  name: mypod2
spec:
  containers:
    - name: myctr
      image: ` + CITEST_IMAGE + `
      command:
        - top
      volumeMounts:
        - name: foo
          mountPath: /etc/foo
          readOnly: true
        - name: bar
          mountPath: /etc/bar
          readOnly: true
        - name: baz
          mountPath: /etc/baz
          readOnly: true
  volumes:
    - name: foo
      secret:
        secretName: newsecret
        optional: false
    - name: bar
      secret:
        secretName: newsecrettwo
        optional: false
    - name: baz
      secret:
        secretName: newsecrettwo
        optional: false
`

var optionalExistingSecretPodYaml = `
apiVersion: v1
kind: Pod
metadata:
  name: mypod
spec:
  containers:
    - name: myctr
      image: ` + CITEST_IMAGE + `
      command:
        - top
      volumeMounts:
        - name: foo
          mountPath: /etc/foo
          readOnly: true
  volumes:
    - name: foo
      secret:
        secretName: newsecret
        optional: true
`

var optionalNonExistingSecretPodYaml = `
apiVersion: v1
kind: Pod
metadata:
  name: mypod
spec:
  containers:
    - name: myctr
      image: ` + CITEST_IMAGE + `
      command:
        - top
      volumeMounts:
        - name: foo
          mountPath: /etc/foo
          readOnly: true
  volumes:
    - name: foo
      secret:
        secretName: oldsecret
        optional: true
`

var noOptionalExistingSecretPodYaml = `
apiVersion: v1
kind: Pod
metadata:
  name: mypod
spec:
  containers:
    - name: myctr
      image: ` + CITEST_IMAGE + `
      command:
        - top
      volumeMounts:
        - name: foo
          mountPath: /etc/foo
          readOnly: true
  volumes:
    - name: foo
      secret:
        secretName: newsecret
`

var noOptionalNonExistingSecretPodYaml = `
apiVersion: v1
kind: Pod
metadata:
  name: mypod
spec:
  containers:
    - name: myctr
      image: ` + CITEST_IMAGE + `
      command:
        - top
      volumeMounts:
        - name: foo
          mountPath: /etc/foo
          readOnly: true
  volumes:
    - name: foo
      secret:
        secretName: oldsecret`

var simplePodYaml = `
apiVersion: v1
kind: Pod
metadata:
  name: libpod-test
spec:
  containers:
  - image: ` + CITEST_IMAGE + `
    command:
      - sleep
      - "3600"`

var simpleWithoutPodPrefixYaml = `
apiVersion: v1
kind: Pod
metadata:
  name: libpod-test
spec:
  containers:
  - name: simpleWithoutPodPrefix
    image: ` + CITEST_IMAGE + `
    command:
      - sleep
      - "3600"`

var unknownKindYaml = `
apiVersion: v1
kind: UnknownKind
metadata:
  labels:
    app: app1
  name: unknown
spec:
  hostname: unknown
`

var workdirSymlinkPodYaml = `
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test-symlink
  name: test-symlink
spec:
  containers:
  - image: test-symlink
    name: test-symlink
    resources: {}
  restartPolicy: Never
`

var podnameEqualsContainerNameYaml = `
apiVersion: v1
kind: Pod
metadata:
  name: podnameEqualsContainerNameYaml
spec:
  containers:
  - name: podnameEqualsContainerNameYaml
    image: ` + CITEST_IMAGE + `
`

var podWithoutAName = `
apiVersion: v1
kind: Pod
spec:
  containers:
  - name: podDoesntHaveAName
    image: ` + CITEST_IMAGE + `
    ports:
    - containerPort: 80
`

var podWithoutAnImage = `
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: podDoesntHaveAnImage
  name: podDoesntHaveAnImage
spec:
  containers:
  - name: podDoesntHaveAnImage
    ports:
    - containerPort: 80
`

var subpathTestNamedVolume = `
apiVersion: v1
kind: Pod
metadata:
  name: testpod
spec:
    containers:
    - name: testctr
      image: ` + CITEST_IMAGE + `
      command:
      - /bin/sh
      - -c
      - |
        trap exit SIGTERM
        while :; do sleep 0.1; done
      volumeMounts:
      - mountPath: /var
        name: testing
        subPath: testing/onlythis
    volumes:
    - name: testing
      persistentVolumeClaim:
        claimName: testvol
`

var signalTest = `
apiVersion: v1
kind: Pod
metadata:
  name: testpod
spec:
    containers:
    - name: testctr
      image: ` + CITEST_IMAGE + `
      command:
        - /bin/sh
        - -c
        - |
          trap 'echo TERMINATED > /testvol/termfile; exit' SIGTERM
          while true; do sleep 0.1; done
      volumeMounts:
      - mountPath: /testvol
        name: testvol
    volumes:
    - name: testvol
      persistentVolumeClaim:
        claimName: testvol
`

var checkInfraImagePodYaml = `
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: check-infra-image
  name: check-infra-image
spec:
  containers:
    - name: testimage
      image: ` + CITEST_IMAGE + `
      command:
        - sleep
        - 24h
status: {}
`

var podWithoutConfigMapDefined = `
apiVersion: v1
kind: Pod
metadata:
  name: testpod1
spec:
  containers:
    - name: testimage
      image: ` + CITEST_IMAGE + `
      volumeMounts:
        - name: mycm
          mountPath: /mycm
  volumes:
    - name: mycm
      configMap:
        name: mycm
`

var sharedNamespacePodYaml = `
apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: "2021-05-07T17:25:01Z"
  labels:
    app: testpod1
  name: testpod1
spec:
  containers:
  - command:
    - top
    - -d
    - "1.5"
    env:
    - name: HOSTNAME
      value: label-pod
    image: ` + CITEST_IMAGE + `
    name: testimage
    resources: {}
    securityContext:
      allowPrivilegeEscalation: true
      capabilities: {}
      privileged: false
      readOnlyRootFilesystem: false
      seLinuxOptions: {}
    workingDir: /
  dnsConfig: {}
  restartPolicy: Never
  shareProcessNamespace: true
status: {}
`

var livenessProbePodYaml = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: liveness-probe
  labels:
    app: testimage
spec:
  replicas: 1
  selector:
    matchLabels:
      app: testimage
  template:
    metadata:
      labels:
        app: testimage
    spec:
      containers:
      - command:
        - top
        - -d
        - "1.5"
        name: testimage
        image: ` + CITEST_IMAGE + `
        livenessProbe:
          exec:
            command:
            - echo
            - hello
          initialDelaySeconds: 5
          periodSeconds: 5
`

var livenessProbeUnhealthyPodYaml = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: liveness-unhealthy-probe
  labels:
    app: testimage
spec:
  replicas: 1
  selector:
    matchLabels:
      app: testimage
  template:
    metadata:
      labels:
        app: testimage
    spec:
      restartPolicy: Never
      containers:
      - command:
        - top
        - -d
        - "1.5"
        name: testimage
        image: ` + CITEST_IMAGE + `
        livenessProbe:
          exec:
            command:
            - cat
            - /randomfile
          initialDelaySeconds: 0
          periodSeconds: 1
`

var startupProbePodYaml = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: startup-healthy-probe
  labels:
    app: testimage
spec:
  replicas: 1
  selector:
    matchLabels:
      app: testimage
  template:
    metadata:
      labels:
        app: testimage
    spec:
      restartPolicy: Never
      containers:
      - command:
        - top
        - -d
        - "1.5"
        name: testimage
        image: ` + CITEST_IMAGE + `
        startupProbe:
          exec:
            command:
            - /bin/sh
            - -c
            - cat /testfile
          initialDelaySeconds: 0
          periodSeconds: 1
        livenessProbe:
          exec:
            command:
            - echo
            - liveness probe
          initialDelaySeconds: 0
          periodSeconds: 1
`

var replicasPodYaml = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: replicas-pods-test
  labels:
    app: testimage
spec:
  replicas: 2
  selector:
    matchLabels:
      app: testimage
  template:
    metadata:
      labels:
        app: testimage
    spec:
      containers:
      - name: testimage
        image: ` + NGINX_IMAGE + `
`

var selinuxLabelPodYaml = `
apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: "2021-02-02T22:18:20Z"
  labels:
    app: label-pod
  name: label-pod
spec:
  containers:
  - command:
    - top
    - -d
    - "1.5"
    env:
    - name: HOSTNAME
      value: label-pod
    image: ` + CITEST_IMAGE + `
    name: test
    securityContext:
      allowPrivilegeEscalation: true
      privileged: false
      readOnlyRootFilesystem: false
      seLinuxOptions:
        user: unconfined_u
        role: system_r
        type: spc_t
        level: s0
    workingDir: /
status: {}
`

var volumesFromPodYaml = `
apiVersion: v1
kind: Pod
metadata:
  annotations:
    io.podman.annotations.volumes-from/tgtctr: srcctr:ro
  name: volspod
spec:
    containers:
    - name: srcctr
      image: ` + CITEST_IMAGE + `
      command:
        - sleep
        - inf
      volumeMounts:
      - mountPath: /mnt/vol
        name: testing
    - name: tgtctr
      image: ` + CITEST_IMAGE + `
      command:
        - sleep
        - inf
    volumes:
    - name: testing
      persistentVolumeClaim:
        claimName: testvol
`

var configMapYamlTemplate = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Name }}
data:
{{ with .Data }}
  {{ range $key, $value := . }}
    {{ $key }}: {{ $value }}
  {{ end }}
{{ end }}
`

var secretYamlTemplate = `
apiVersion: v1
kind: Secret
metadata:
  name: {{ .Name }}
data:
{{ with .Data }}
  {{ range $key, $value := . }}
    {{ $key }}: {{ $value }}
  {{ end }}
{{ end }}
`

var persistentVolumeClaimYamlTemplate = `
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: {{ .Name }}
{{ with .Annotations }}
  annotations:
  {{ range $key, $value := . }}
    {{ $key }}: {{ $value }}
  {{ end }}
{{ end }}
spec:
  accessModes:
    - "ReadWriteOnce"
  resources:
    requests:
      storage: "1Gi"
  storageClassName: default
`

var podYamlTemplate = `
apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: "2019-07-17T14:44:08Z"
  name: {{ .Name }}
  labels:
    app: {{ .Name }}
{{ with .Labels }}
  {{ range $key, $value := . }}
    {{ $key }}: {{ $value }}
  {{ end }}
{{ end }}
{{ with .Annotations }}
  annotations:
  {{ range $key, $value := . }}
    {{ $key }}: {{ $value }}
  {{ end }}
{{ end }}

spec:
  restartPolicy: {{ .RestartPolicy }}
  hostname: {{ .Hostname }}
  hostNetwork: {{ .HostNetwork }}
{{ if .HostUsers }}
  hostUsers: {{ .HostUsers }}
{{ end }}
  hostAliases:
{{ range .HostAliases }}
  - hostnames:
  {{ range .HostName }}
    - {{ . }}
  {{ end }}
    ip: {{ .IP }}
{{ end }}
  initContainers:
{{ with .InitCtrs }}
  {{ range . }}
  - command:
    {{ range .Cmd }}
    - {{.}}
    {{ end }}
    image: {{ .Image }}
    name: {{ .Name }}
  {{ end }}
{{ end }}
{{ if .SecurityContext }}
  securityContext:
    {{ if .RunAsUser }}runAsUser: {{ .RunAsUser }}{{- end }}
    {{ if .RunAsGroup }}runAsGroup: {{ .RunAsGroup }}{{- end }}
{{ end }}
  containers:
{{ with .Ctrs }}
  {{ range . }}
  - command:
    {{ range .Cmd }}
    - {{.}}
    {{ end }}
    args:
    {{ range .Arg }}
    - {{.}}
    {{ end }}
    env:
    - name: HOSTNAME
    {{ range .Env }}
    - name: {{ .Name }}
    {{ if (eq .ValueFrom "configmap") }}
      valueFrom:
        configMapKeyRef:
          name: {{ .RefName }}
          key: {{ .RefKey }}
          optional: {{ .Optional }}
    {{ end }}
    {{ if (eq .ValueFrom "secret") }}
      valueFrom:
        secretKeyRef:
          name: {{ .RefName }}
          key: {{ .RefKey }}
          optional: {{ .Optional }}
    {{ end }}
    {{ if (eq .ValueFrom "") }}
      value: {{ .Value }}
    {{ end }}
    {{ end }}
    {{ with .EnvFrom}}
    envFrom:
    {{ range . }}
    {{ if (eq .From "configmap") }}
    - configMapRef:
        name: {{ .Name }}
        optional: {{ .Optional }}
    {{ end }}
    {{ if (eq .From "secret") }}
    - secretRef:
        name: {{ .Name }}
        optional: {{ .Optional }}
    {{ end }}
    {{ end }}
    {{ end }}
    image: {{ .Image }}
    name: {{ .Name }}
    imagePullPolicy: {{ .PullPolicy }}
    {{- if or .CPURequest .CPULimit .MemoryRequest .MemoryLimit }}
    resources:
      {{- if or .CPURequest .MemoryRequest }}
      requests:
        {{if .CPURequest }}cpu: {{ .CPURequest }}{{ end }}
        {{if .MemoryRequest }}memory: {{ .MemoryRequest }}{{ end }}
      {{- end }}
      {{- if or .CPULimit .MemoryLimit }}
      limits:
        {{if .CPULimit }}cpu: {{ .CPULimit }}{{ end }}
        {{if .MemoryLimit }}memory: {{ .MemoryLimit }}{{ end }}
      {{- end }}
    {{- end }}
    {{ if .SecurityContext }}
    securityContext:
      {{ if .RunAsUser }}runAsUser: {{ .RunAsUser }}{{- end }}
      {{ if .RunAsGroup }}runAsGroup: {{ .RunAsGroup }}{{- end }}
      allowPrivilegeEscalation: true
      {{ if .Caps }}
      capabilities:
        {{ with .CapAdd }}
        add:
          {{ range . }}
          - {{.}}
          {{ end }}
        {{ end }}
        {{ with .CapDrop }}
        drop:
          {{ range . }}
          - {{.}}
          {{ end }}
        {{ end }}
      {{ end }}
      privileged: false
      readOnlyRootFilesystem: false
    ports:
    - containerPort: {{ .Port }}
      hostIP: {{ .HostIP }}
      hostPort: {{ .Port }}
      protocol: TCP
    workingDir: /
    volumeMounts:
    {{ if .VolumeMount }}
    - name: {{.VolumeName}}
      mountPath: {{ .VolumeMountPath }}
      subPath: {{ .VolumeSubPath }}
      readonly: {{.VolumeReadOnly}}
      {{ end }}
    {{ end }}
  {{ end }}
{{ end }}
{{ with .Volumes }}
  volumes:
  {{ range . }}
  - name: {{ .Name }}
    {{- if (eq .VolumeType "EmptyDir") }}
    emptyDir: {}
    {{- end }}
    {{- if (eq .VolumeType "HostPath") }}
    hostPath:
      path: {{ .HostPath.Path }}
      type: {{ .HostPath.Type }}
    {{- end }}
    {{- if (eq .VolumeType "PersistentVolumeClaim") }}
    persistentVolumeClaim:
      claimName: {{ .PersistentVolumeClaim.ClaimName }}
    {{- end }}
	{{- if (eq .VolumeType "ConfigMap") }}
    configMap:
      name: {{ .ConfigMap.Name }}
      optional: {{ .ConfigMap.Optional }}
      defaultMode: {{ .ConfigMap.DefaultMode }}
      {{- with .ConfigMap.Items }}
      items:
      {{- range . }}
        - key: {{ .key }}
          path: {{ .path }}
    {{- end }}
    {{- end }}
    {{- end }}
	{{- if (eq .VolumeType "Secret") }}
    secret:
      secretName: {{ .SecretVol.SecretName }}
      optional: {{ .SecretVol.Optional }}
      defaultMode: {{ .SecretVol.DefaultMode }}
      {{- with .SecretVol.Items }}
      items:
      {{- range . }}
        - key: {{ .key }}
          path: {{ .path }}
    {{- end }}
	{{- end }}
    {{- end }}
  {{ end }}
{{ end }}
status: {}
`

var daemonSetYamlTemplate = `
apiVersion: v1
kind: DaemonSet
metadata:
  creationTimestamp: "2019-07-17T14:44:08Z"
  name: {{ .Name }}
  labels:
    app: {{ .Name }}
{{ with .Labels }}
  {{ range $key, $value := . }}
    {{ $key }}: {{ $value }}
  {{ end }}
{{ end }}
{{ with .Annotations }}
  annotations:
  {{ range $key, $value := . }}
    {{ $key }}: {{ $value }}
  {{ end }}
{{ end }}

spec:
  selector:
    matchLabels:
      app: {{ .Name }}
  template:
  {{ with .PodTemplate }}
    metadata:
      labels:
        app: {{ .Name }}
        {{- with .Labels }}{{ range $key, $value := . }}
        {{ $key }}: {{ $value }}
        {{- end }}{{ end }}
      {{- with .Annotations }}
      annotations:
      {{- range $key, $value := . }}
        {{ $key }}: {{ $value }}
      {{- end }}
      {{- end }}
    spec:
      restartPolicy: {{ .RestartPolicy }}
      hostname: {{ .Hostname }}
      hostNetwork: {{ .HostNetwork }}
      containers:
    {{ with .Ctrs }}
      {{ range . }}
      - command:
        {{ range .Cmd }}
        - {{.}}
        {{ end }}
        args:
        {{ range .Arg }}
        - {{.}}
        {{ end }}
        env:
        - name: HOSTNAME
        {{ range .Env }}
        - name: {{ .Name }}
        {{ if (eq .ValueFrom "configmap") }}
          valueFrom:
            configMapKeyRef:
              name: {{ .RefName }}
              key: {{ .RefKey }}
              optional: {{ .Optional }}
        {{ end }}
        {{ if (eq .ValueFrom "secret") }}
          valueFrom:
            secretKeyRef:
              name: {{ .RefName }}
              key: {{ .RefKey }}
              optional: {{ .Optional }}
        {{ end }}
        {{ if (eq .ValueFrom "") }}
          value: {{ .Value }}
        {{ end }}
        {{ end }}
        {{ with .EnvFrom}}
        envFrom:
        {{ range . }}
        {{ if (eq .From "configmap") }}
        - configMapRef:
            name: {{ .Name }}
            optional: {{ .Optional }}
        {{ end }}
        {{ if (eq .From "secret") }}
        - secretRef:
            name: {{ .Name }}
            optional: {{ .Optional }}
        {{ end }}
        {{ end }}
        {{ end }}
        image: {{ .Image }}
        name: {{ .Name }}
        imagePullPolicy: {{ .PullPolicy }}
        {{- if or .CPURequest .CPULimit .MemoryRequest .MemoryLimit }}
        resources:
          {{- if or .CPURequest .MemoryRequest }}
          requests:
            {{if .CPURequest }}cpu: {{ .CPURequest }}{{ end }}
            {{if .MemoryRequest }}memory: {{ .MemoryRequest }}{{ end }}
          {{- end }}
          {{- if or .CPULimit .MemoryLimit }}
          limits:
            {{if .CPULimit }}cpu: {{ .CPULimit }}{{ end }}
            {{if .MemoryLimit }}memory: {{ .MemoryLimit }}{{ end }}
          {{- end }}
        {{- end }}
        {{ if .SecurityContext }}
        securityContext:
          allowPrivilegeEscalation: true
          {{ if .Caps }}
          capabilities:
            {{ with .CapAdd }}
            add:
              {{ range . }}
              - {{.}}
              {{ end }}
            {{ end }}
            {{ with .CapDrop }}
            drop:
              {{ range . }}
              - {{.}}
              {{ end }}
            {{ end }}
          {{ end }}
          privileged: false
          readOnlyRootFilesystem: false
        workingDir: /
        volumeMounts:
        {{ if .VolumeMount }}
        - name: {{.VolumeName}}
          mountPath: {{ .VolumeMountPath }}
          readonly: {{.VolumeReadOnly}}
        {{ end }}
        {{ end }}
      {{ end }}
    {{ end }}
    {{ with .Volumes }}
      volumes:
      {{ range . }}
      - name: {{ .Name }}
        {{- if (eq .VolumeType "HostPath") }}
        hostPath:
          path: {{ .HostPath.Path }}
          type: {{ .HostPath.Type }}
        {{- end }}
        {{- if (eq .VolumeType "PersistentVolumeClaim") }}
        persistentVolumeClaim:
          claimName: {{ .PersistentVolumeClaim.ClaimName }}
        {{- end }}
      {{ end }}
    {{ end }}
{{ end }}
`

var deploymentYamlTemplate = `
apiVersion: v1
kind: Deployment
metadata:
  creationTimestamp: "2019-07-17T14:44:08Z"
  name: {{ .Name }}
  labels:
    app: {{ .Name }}
{{ with .Labels }}
  {{ range $key, $value := . }}
    {{ $key }}: {{ $value }}
  {{ end }}
{{ end }}
{{ with .Annotations }}
  annotations:
  {{ range $key, $value := . }}
    {{ $key }}: {{ $value }}
  {{ end }}
{{ end }}

spec:
  replicas: {{ .Replicas }}
  selector:
    matchLabels:
      app: {{ .Name }}
  template:
  {{ with .PodTemplate }}
    metadata:
      labels:
        app: {{ .Name }}
        {{- with .Labels }}{{ range $key, $value := . }}
        {{ $key }}: {{ $value }}
        {{- end }}{{ end }}
      {{- with .Annotations }}
      annotations:
      {{- range $key, $value := . }}
        {{ $key }}: {{ $value }}
      {{- end }}
      {{- end }}
    spec:
      restartPolicy: {{ .RestartPolicy }}
      hostname: {{ .Hostname }}
      hostNetwork: {{ .HostNetwork }}
      containers:
    {{ with .Ctrs }}
      {{ range . }}
      - command:
        {{ range .Cmd }}
        - {{.}}
        {{ end }}
        args:
        {{ range .Arg }}
        - {{.}}
        {{ end }}
        env:
        - name: HOSTNAME
        {{ range .Env }}
        - name: {{ .Name }}
        {{ if (eq .ValueFrom "configmap") }}
          valueFrom:
            configMapKeyRef:
              name: {{ .RefName }}
              key: {{ .RefKey }}
              optional: {{ .Optional }}
        {{ end }}
        {{ if (eq .ValueFrom "secret") }}
          valueFrom:
            secretKeyRef:
              name: {{ .RefName }}
              key: {{ .RefKey }}
              optional: {{ .Optional }}
        {{ end }}
        {{ if (eq .ValueFrom "") }}
          value: {{ .Value }}
        {{ end }}
        {{ end }}
        {{ with .EnvFrom}}
        envFrom:
        {{ range . }}
        {{ if (eq .From "configmap") }}
        - configMapRef:
            name: {{ .Name }}
            optional: {{ .Optional }}
        {{ end }}
        {{ if (eq .From "secret") }}
        - secretRef:
            name: {{ .Name }}
            optional: {{ .Optional }}
        {{ end }}
        {{ end }}
        {{ end }}
        image: {{ .Image }}
        name: {{ .Name }}
        imagePullPolicy: {{ .PullPolicy }}
        {{- if or .CPURequest .CPULimit .MemoryRequest .MemoryLimit }}
        resources:
          {{- if or .CPURequest .MemoryRequest }}
          requests:
            {{if .CPURequest }}cpu: {{ .CPURequest }}{{ end }}
            {{if .MemoryRequest }}memory: {{ .MemoryRequest }}{{ end }}
          {{- end }}
          {{- if or .CPULimit .MemoryLimit }}
          limits:
            {{if .CPULimit }}cpu: {{ .CPULimit }}{{ end }}
            {{if .MemoryLimit }}memory: {{ .MemoryLimit }}{{ end }}
          {{- end }}
        {{- end }}
        {{ if .SecurityContext }}
        securityContext:
          allowPrivilegeEscalation: true
          {{ if .Caps }}
          capabilities:
            {{ with .CapAdd }}
            add:
              {{ range . }}
              - {{.}}
              {{ end }}
            {{ end }}
            {{ with .CapDrop }}
            drop:
              {{ range . }}
              - {{.}}
              {{ end }}
            {{ end }}
          {{ end }}
          privileged: false
          readOnlyRootFilesystem: false
        workingDir: /
        volumeMounts:
        {{ if .VolumeMount }}
        - name: {{.VolumeName}}
          mountPath: {{ .VolumeMountPath }}
          readonly: {{.VolumeReadOnly}}
        {{ end }}
        {{ end }}
      {{ end }}
    {{ end }}
    {{ with .Volumes }}
      volumes:
      {{ range . }}
      - name: {{ .Name }}
        {{- if (eq .VolumeType "HostPath") }}
        hostPath:
          path: {{ .HostPath.Path }}
          type: {{ .HostPath.Type }}
        {{- end }}
        {{- if (eq .VolumeType "PersistentVolumeClaim") }}
        persistentVolumeClaim:
          claimName: {{ .PersistentVolumeClaim.ClaimName }}
        {{- end }}
      {{ end }}
    {{ end }}
{{ end }}
`

var jobYamlTemplate = `
apiVersion: batch/v1
kind: Job
metadata:
  creationTimestamp: "2019-07-17T14:44:08Z"
  name: {{ .Name }}
  labels:
    app: {{ .Name }}
{{ with .Labels }}
  {{ range $key, $value := . }}
    {{ $key }}: {{ $value }}
  {{ end }}
{{ end }}
{{ with .Annotations }}
  annotations:
  {{ range $key, $value := . }}
    {{ $key }}: {{ $value }}
  {{ end }}
{{ end }}

spec:
  template:
  {{ with .PodTemplate }}
    metadata:
      labels:
        app: {{ .Name }}
        {{- with .Labels }}{{ range $key, $value := . }}
        {{ $key }}: {{ $value }}
        {{- end }}{{ end }}
      {{- with .Annotations }}
      annotations:
      {{- range $key, $value := . }}
        {{ $key }}: {{ $value }}
      {{- end }}
      {{- end }}
    spec:
      restartPolicy: {{ .RestartPolicy }}
      hostname: {{ .Hostname }}
      hostNetwork: {{ .HostNetwork }}
      containers:
    {{ with .Ctrs }}
      {{ range . }}
      - command:
        {{ range .Cmd }}
        - {{.}}
        {{ end }}
        args:
        {{ range .Arg }}
        - {{.}}
        {{ end }}
        env:
        - name: HOSTNAME
        {{ range .Env }}
        - name: {{ .Name }}
        {{ if (eq .ValueFrom "configmap") }}
          valueFrom:
            configMapKeyRef:
              name: {{ .RefName }}
              key: {{ .RefKey }}
              optional: {{ .Optional }}
        {{ end }}
        {{ if (eq .ValueFrom "secret") }}
          valueFrom:
            secretKeyRef:
              name: {{ .RefName }}
              key: {{ .RefKey }}
              optional: {{ .Optional }}
        {{ end }}
        {{ if (eq .ValueFrom "") }}
          value: {{ .Value }}
        {{ end }}
        {{ end }}
        {{ with .EnvFrom}}
        envFrom:
        {{ range . }}
        {{ if (eq .From "configmap") }}
        - configMapRef:
            name: {{ .Name }}
            optional: {{ .Optional }}
        {{ end }}
        {{ if (eq .From "secret") }}
        - secretRef:
            name: {{ .Name }}
            optional: {{ .Optional }}
        {{ end }}
        {{ end }}
        {{ end }}
        image: {{ .Image }}
        name: {{ .Name }}
        imagePullPolicy: {{ .PullPolicy }}
        {{- if or .CPURequest .CPULimit .MemoryRequest .MemoryLimit }}
        resources:
          {{- if or .CPURequest .MemoryRequest }}
          requests:
            {{if .CPURequest }}cpu: {{ .CPURequest }}{{ end }}
            {{if .MemoryRequest }}memory: {{ .MemoryRequest }}{{ end }}
          {{- end }}
          {{- if or .CPULimit .MemoryLimit }}
          limits:
            {{if .CPULimit }}cpu: {{ .CPULimit }}{{ end }}
            {{if .MemoryLimit }}memory: {{ .MemoryLimit }}{{ end }}
          {{- end }}
        {{- end }}
        {{ if .SecurityContext }}
        securityContext:
          allowPrivilegeEscalation: true
          {{ if .Caps }}
          capabilities:
            {{ with .CapAdd }}
            add:
              {{ range . }}
              - {{.}}
              {{ end }}
            {{ end }}
            {{ with .CapDrop }}
            drop:
              {{ range . }}
              - {{.}}
              {{ end }}
            {{ end }}
          {{ end }}
          privileged: false
          readOnlyRootFilesystem: false
        workingDir: /
        volumeMounts:
        {{ if .VolumeMount }}
        - name: {{.VolumeName}}
          mountPath: {{ .VolumeMountPath }}
          readonly: {{.VolumeReadOnly}}
        {{ end }}
        {{ end }}
      {{ end }}
    {{ end }}
    {{ with .Volumes }}
      volumes:
      {{ range . }}
      - name: {{ .Name }}
        {{- if (eq .VolumeType "HostPath") }}
        hostPath:
          path: {{ .HostPath.Path }}
          type: {{ .HostPath.Type }}
        {{- end }}
        {{- if (eq .VolumeType "PersistentVolumeClaim") }}
        persistentVolumeClaim:
          claimName: {{ .PersistentVolumeClaim.ClaimName }}
        {{- end }}
      {{ end }}
    {{ end }}
{{ end }}
`

var publishPortsPodWithoutPorts = `
apiVersion: v1
kind: Pod
metadata:
  name: nginx
spec:
  containers:
  - name: nginx
    image: ` + NGINX_IMAGE + `
    imagePullPolicy: missing
`

var publishPortsPodWithContainerPort = `
apiVersion: v1
kind: Pod
metadata:
  name: nginx
spec:
  containers:
  - name: nginx
    image: ` + NGINX_IMAGE + `
    imagePullPolicy: missing
    ports:
    - containerPort: 80
`

var publishPortsPodWithContainerHostPort = `
apiVersion: v1
kind: Pod
metadata:
  name: nginx
spec:
  containers:
  - name: nginx
    image: ` + NGINX_IMAGE + `
    imagePullPolicy: missing
    ports:
    - containerPort: 80
      hostPort: 19001
`

var publishPortsEchoWithHostPortUDP = `
apiVersion: v1
kind: Pod
metadata:
  name: network-echo
spec:
  containers:
  - name: udp-echo
    image: ` + CITEST_IMAGE + `
    command:
    - "/bin/sh"
    - "-c"
    - "nc -ulk -p 19008 -e /bin/cat"
    ports:
    - containerPort: 19008
      hostPort: 19009
      protocol: udp
  - name: tcp-echo
    image: ` + CITEST_IMAGE + `
    command:
    - "/bin/sh"
    - "-c"
    - "nc -lk -p 19008 -e /bin/cat"
`

var publishPortsEchoWithHostPortTCP = `
apiVersion: v1
kind: Pod
metadata:
  name: network-echo
spec:
  containers:
  - name: udp-echo
    image: ` + CITEST_IMAGE + `
    command:
    - "/bin/sh"
    - "-c"
    - "nc -ulk -p 19008 -e /bin/cat"
  - name: tcp-echo
    image: ` + CITEST_IMAGE + `
    command:
    - "/bin/sh"
    - "-c"
    - "nc -lk -p 19008 -e /bin/cat"
    ports:
    - containerPort: 19008
      hostPort: 19011
      protocol: tcp
`

var podWithHostPIDDefined = `
apiVersion: v1
kind: Pod
metadata:
  name: test-hostpid
spec:
  hostPID: true
  containers:
  - name: testimage
    image: ` + CITEST_IMAGE + `
    command: ['sh', '-c', 'echo $$']
`

var podWithHostIPCDefined = `
apiVersion: v1
kind: Pod
metadata:
  name: test-hostipc
spec:
  hostIPC: true
  containers:
  - name: testimage
    image: ` + CITEST_IMAGE + `
    command: ['ls', '-l', '/proc/self/ns/ipc']
  restartPolicy: Never
`

var podWithSysctlDefined = `
apiVersion: v1
kind: Pod
metadata:
  name: test-sysctl
spec:
  securityContext:
    sysctls:
    - name: kernel.msgmax
      value: "65535"
    - name: net.core.somaxconn
      value: "65535"
  containers:
  - name: testimage
    image: ` + CITEST_IMAGE + `
    command:
    - "/bin/sh"
    - "-c"
    - "sysctl kernel.msgmax;sysctl net.core.somaxconn"
  restartPolicy: Never
`

var podWithSysctlHostNetDefined = `
apiVersion: v1
kind: Pod
metadata:
  name: test-sysctl
spec:
  securityContext:
    sysctls:
    - name: kernel.msgmax
      value: "65535"
    - name: net.core.somaxconn
      value: "65535"
  hostNetwork: true
  containers:
  - name: testimage
    image: ` + CITEST_IMAGE + `
    command:
    - "/bin/sh"
    - "-c"
    - "sysctl kernel.msgmax"
`

var listPodAndConfigMap = `
apiVersion: v1
kind: List
items:
- apiVersion: v1
  kind: ConfigMap
  metadata:
    name: test-list-configmap
  data:
    foo: bar
- apiVersion: v1
  kind: Pod
  metadata:
    name: test-list-pod
  spec:
    containers:
    - name: container
      image: ` + CITEST_IMAGE + `
      command: [ "/bin/sh", "-c", "env" ]
      env:
      - name: FOO
        valueFrom:
          configMapKeyRef:
            name: test-list-configmap
            key: foo
    restartPolicy: Never
`

var stopSignalSIGUSR1 = `
apiVersion: v1
kind: Pod
metadata:
  name: testpod
spec:
    containers:
    - name: testctr
      image: ` + CITEST_IMAGE + `
      lifecycle:
        stopSignal: SIGUSR1
`

var stopSignalNosuchSignal = `
apiVersion: v1
kind: Pod
metadata:
  name: noSuchSigTest
spec:
    containers:
    - name: test1
      image: ` + CITEST_IMAGE + `
      lifecycle:
        stopSignal: noSuchSignal
`

var (
	defaultCtrName        = "testCtr"
	defaultCtrCmd         = []string{"top"}
	defaultCtrArg         = []string{"-d", "1.5"}
	defaultCtrImage       = CITEST_IMAGE
	defaultPodName        = "testPod"
	defaultVolName        = "testVol"
	defaultDaemonSetName  = "testDaemonSet"
	defaultDeploymentName = "testDeployment"
	defaultJobName        = "testJob"
	defaultConfigMapName  = "testConfigMap"
	defaultSecretName     = "testSecret"
	defaultPVCName        = "testPVC"
	seccompLinkEPERM      = []byte(`{"defaultAction":"SCMP_ACT_ALLOW","syscalls":[{"names":["link","linkat"],"action":"SCMP_ACT_ERRNO"}]}`)
	// CPU Period in ms
	defaultCPUPeriod = 100
	// Default secret in JSON. Note that the values ("foo" and "bar") are base64 encoded.
	defaultSecret = []byte(`{"FOO":"Zm9v","BAR":"YmFy"}`)
)

// getKubeYaml returns a kubernetes YAML document.
func getKubeYaml(kind string, object any) (string, error) {
	var yamlTemplate string
	templateBytes := &bytes.Buffer{}

	switch kind {
	case "configmap":
		yamlTemplate = configMapYamlTemplate
	case "pod":
		yamlTemplate = podYamlTemplate
	case "daemonset":
		yamlTemplate = daemonSetYamlTemplate
	case "deployment":
		yamlTemplate = deploymentYamlTemplate
	case "job":
		yamlTemplate = jobYamlTemplate
	case "persistentVolumeClaim":
		yamlTemplate = persistentVolumeClaimYamlTemplate
	case "secret":
		yamlTemplate = secretYamlTemplate
	default:
		return "", fmt.Errorf("unsupported kubernetes kind")
	}

	t, err := template.New(kind).Parse(yamlTemplate)
	if err != nil {
		return "", err
	}

	if err := t.Execute(templateBytes, object); err != nil {
		return "", err
	}

	return templateBytes.String(), nil
}

// generateKubeYaml writes a kubernetes YAML document.
func generateKubeYaml(kind string, object any, pathname string) error {
	k, err := getKubeYaml(kind, object)
	if err != nil {
		return err
	}

	return writeYaml(k, pathname)
}

// generateMultiDocKubeYaml writes multiple kube objects in one Yaml document.
func generateMultiDocKubeYaml(kubeObjects []string, pathname string) error {
	var multiKube strings.Builder

	for _, k := range kubeObjects {
		multiKube.WriteString("---\n")
		multiKube.WriteString(k)
	}

	return writeYaml(multiKube.String(), pathname)
}

func createSecret(podmanTest *PodmanTestIntegration, name string, value []byte) { //nolint:unparam
	secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
	err := os.WriteFile(secretFilePath, value, 0o755)
	Expect(err).ToNot(HaveOccurred())

	secret := podmanTest.Podman([]string{"secret", "create", name, secretFilePath})
	secret.WaitWithDefaultTimeout()
	Expect(secret).Should(ExitCleanly())
}

// Secret describes the options a kube yaml can be configured at secret level
type Secret struct {
	Name string
	Data map[string]string
}

func getSecret(options ...secretOption) *Secret {
	secret := Secret{
		Name: defaultSecretName,
		Data: map[string]string{},
	}

	for _, option := range options {
		option(&secret)
	}

	return &secret
}

type secretOption func(*Secret)

func withSecretName(name string) secretOption {
	return func(secret *Secret) {
		secret.Name = name
	}
}

func withSecretData(k, v string) secretOption {
	return func(secret *Secret) {
		secret.Data[k] = base64.StdEncoding.EncodeToString([]byte(v))
	}
}

// CM describes the options a kube yaml can be configured at configmap level
type CM struct {
	Name string
	Data map[string]string
}

func getConfigMap(options ...configMapOption) *CM {
	cm := CM{
		Name: defaultConfigMapName,
		Data: map[string]string{},
	}

	for _, option := range options {
		option(&cm)
	}

	return &cm
}

type configMapOption func(*CM)

func withConfigMapName(name string) configMapOption {
	return func(configmap *CM) {
		configmap.Name = name
	}
}

func withConfigMapData(k, v string) configMapOption {
	return func(configmap *CM) {
		configmap.Data[k] = v
	}
}

// PVC describes the options a kube yaml can be configured at persistent volume claim level
type PVC struct {
	Name        string
	Annotations map[string]string
}

func getPVC(options ...pvcOption) *PVC {
	pvc := PVC{
		Name:        defaultPVCName,
		Annotations: map[string]string{},
	}

	for _, option := range options {
		option(&pvc)
	}

	return &pvc
}

type pvcOption func(*PVC)

func withPVCName(name string) pvcOption {
	return func(pvc *PVC) {
		pvc.Name = name
	}
}

func withPVCAnnotations(k, v string) pvcOption {
	return func(pvc *PVC) {
		pvc.Annotations[k] = v
	}
}

// Pod describes the options a kube yaml can be configured at pod level
type Pod struct {
	Name            string
	RestartPolicy   string
	Hostname        string
	HostNetwork     bool
	HostUsers       *bool
	HostAliases     []HostAlias
	Ctrs            []*Ctr
	InitCtrs        []*Ctr
	Volumes         []*Volume
	Labels          map[string]string
	Annotations     map[string]string
	SecurityContext bool
	RunAsUser       string
	RunAsGroup      string
}

type HostAlias struct {
	IP       string
	HostName []string
}

// getPod takes a list of podOptions and returns a pod with sane defaults
// and the configured options
// if no containers are added, it will add the default container
func getPod(options ...podOption) *Pod {
	p := Pod{
		Name:          defaultPodName,
		RestartPolicy: "Never",
		Hostname:      "",
		HostNetwork:   false,
		HostAliases:   nil,
		Ctrs:          make([]*Ctr, 0),
		InitCtrs:      make([]*Ctr, 0),
		Volumes:       make([]*Volume, 0),
		Labels:        make(map[string]string),
		Annotations:   make(map[string]string),
	}
	for _, option := range options {
		option(&p)
	}
	if len(p.Ctrs) == 0 {
		p.Ctrs = []*Ctr{getCtr()}
	}
	return &p
}

type podOption func(*Pod)

func withPodSecurityContext(sc bool) podOption {
	return func(p *Pod) {
		p.SecurityContext = sc
	}
}

func withPodRunAsUser(runAsUser string) podOption {
	return func(p *Pod) {
		p.RunAsUser = runAsUser
	}
}

func withPodRunAsGroup(runAsGroup string) podOption {
	return func(p *Pod) {
		p.RunAsGroup = runAsGroup
	}
}

func withPodName(name string) podOption {
	return func(pod *Pod) {
		pod.Name = name
	}
}

func withHostname(h string) podOption {
	return func(pod *Pod) {
		pod.Hostname = h
	}
}

func withHostAliases(ip string, host []string) podOption {
	return func(pod *Pod) {
		pod.HostAliases = append(pod.HostAliases, HostAlias{
			IP:       ip,
			HostName: host,
		})
	}
}

func withCtr(c *Ctr) podOption {
	return func(pod *Pod) {
		pod.Ctrs = append(pod.Ctrs, c)
	}
}

func withPodInitCtr(ic *Ctr) podOption {
	return func(pod *Pod) {
		pod.InitCtrs = append(pod.InitCtrs, ic)
	}
}

func withRestartPolicy(policy string) podOption {
	return func(pod *Pod) {
		pod.RestartPolicy = policy
	}
}

func withLabel(k, v string) podOption {
	return func(pod *Pod) {
		pod.Labels[k] = v
	}
}

func withAnnotation(k, v string) podOption {
	return func(pod *Pod) {
		pod.Annotations[k] = v
	}
}

func withVolume(v *Volume) podOption {
	return func(pod *Pod) {
		pod.Volumes = append(pod.Volumes, v)
	}
}

func withHostNetwork() podOption {
	return func(pod *Pod) {
		pod.HostNetwork = true
	}
}

func withHostUsers(val bool) podOption {
	return func(pod *Pod) {
		pod.HostUsers = &val
	}
}

// Daemonset describes the options a kube yaml can be configured at daemoneset level
type DaemonSet struct {
	Name        string
	Labels      map[string]string
	Annotations map[string]string
	PodTemplate *Pod
}

func getDaemonSet(options ...daemonSetOption) *DaemonSet {
	d := DaemonSet{
		Name:        defaultDaemonSetName,
		Labels:      make(map[string]string),
		Annotations: make(map[string]string),
		PodTemplate: getPod(),
	}
	for _, option := range options {
		option(&d)
	}

	return &d
}

type daemonSetOption func(*DaemonSet)

type Deployment struct {
	Name        string
	Replicas    int32
	Labels      map[string]string
	Annotations map[string]string
	PodTemplate *Pod
}

func getDeployment(options ...deploymentOption) *Deployment {
	d := Deployment{
		Name:        defaultDeploymentName,
		Replicas:    1,
		Labels:      make(map[string]string),
		Annotations: make(map[string]string),
		PodTemplate: getPod(),
	}
	for _, option := range options {
		option(&d)
	}

	return &d
}

type deploymentOption func(*Deployment)

func withDeploymentAnnotation(k, v string) deploymentOption {
	return func(deployment *Deployment) {
		deployment.Annotations[k] = v
	}
}

func withPod(pod *Pod) deploymentOption {
	return func(d *Deployment) {
		d.PodTemplate = pod
	}
}

func withReplicas(replicas int32) deploymentOption {
	return func(d *Deployment) {
		d.Replicas = replicas
	}
}

// getPodNameInDeployment returns the Pod object
// with just its name set, so that it can be passed around
// and into getCtrNameInPod for ease of testing
func getPodNameInDaemonSet(d *DaemonSet) Pod {
	p := Pod{}
	p.Name = fmt.Sprintf("%s-pod", d.Name)

	return p
}

// getPodNameInDeployment returns the Pod object
// with just its name set, so that it can be passed around
// and into getCtrNameInPod for ease of testing
func getPodNameInDeployment(d *Deployment) Pod {
	p := Pod{}
	p.Name = fmt.Sprintf("%s-pod", d.Name)

	return p
}

type Job struct {
	Name        string
	Labels      map[string]string
	Annotations map[string]string
	PodTemplate *Pod
}

func getJob(options ...jobOption) *Job {
	j := Job{
		Name:        defaultJobName,
		Labels:      make(map[string]string),
		Annotations: make(map[string]string),
		PodTemplate: getPod(),
	}
	for _, option := range options {
		option(&j)
	}

	return &j
}

type jobOption func(*Job)

// getPodNameInJob returns the Pod object
// with just its name set, so that it can be passed around
// and into getCtrNameInPod for ease of testing
func getPodNameInJob(d *Job) Pod {
	p := Pod{}
	p.Name = fmt.Sprintf("%s-pod", d.Name)

	return p
}

// Ctr describes the options a kube yaml can be configured at container level
type Ctr struct {
	Name            string
	Image           string
	Cmd             []string
	Arg             []string
	CPURequest      string
	CPULimit        string
	MemoryRequest   string
	MemoryLimit     string
	SecurityContext bool
	Caps            bool
	CapAdd          []string
	CapDrop         []string
	PullPolicy      string
	HostIP          string
	Port            string
	VolumeMount     bool
	VolumeMountPath string
	VolumeName      string
	VolumeSubPath   string
	VolumeReadOnly  bool
	Env             []Env
	EnvFrom         []EnvFrom
	InitCtrType     string
	RunAsUser       string
	RunAsGroup      string
}

// getCtr takes a list of ctrOptions and returns a Ctr with sane defaults
// and the configured options
func getCtr(options ...ctrOption) *Ctr {
	c := Ctr{
		Name:            defaultCtrName,
		Image:           defaultCtrImage,
		Cmd:             defaultCtrCmd,
		Arg:             defaultCtrArg,
		SecurityContext: true,
		Caps:            false,
		CapAdd:          nil,
		CapDrop:         nil,
		PullPolicy:      "missing",
		HostIP:          "",
		Port:            "",
		VolumeMount:     false,
		VolumeMountPath: "",
		VolumeName:      "",
		VolumeReadOnly:  false,
		VolumeSubPath:   "",
		Env:             []Env{},
		EnvFrom:         []EnvFrom{},
		InitCtrType:     "",
	}
	for _, option := range options {
		option(&c)
	}
	return &c
}

type ctrOption func(*Ctr)

func withName(name string) ctrOption {
	return func(c *Ctr) {
		c.Name = name
	}
}

func withInitCtr() ctrOption {
	return func(c *Ctr) {
		c.InitCtrType = define.AlwaysInitContainer
	}
}

func withCmd(cmd []string) ctrOption {
	return func(c *Ctr) {
		c.Cmd = cmd
	}
}

func withArg(arg []string) ctrOption {
	return func(c *Ctr) {
		c.Arg = arg
	}
}

func withImage(img string) ctrOption {
	return func(c *Ctr) {
		c.Image = img
	}
}

func withCPURequest(request string) ctrOption {
	return func(c *Ctr) {
		c.CPURequest = request
	}
}

func withCPULimit(limit string) ctrOption {
	return func(c *Ctr) {
		c.CPULimit = limit
	}
}

func withMemoryRequest(request string) ctrOption {
	return func(c *Ctr) {
		c.MemoryRequest = request
	}
}

func withMemoryLimit(limit string) ctrOption {
	return func(c *Ctr) {
		c.MemoryLimit = limit
	}
}

func withSecurityContext(sc bool) ctrOption {
	return func(c *Ctr) {
		c.SecurityContext = sc
	}
}

func withRunAsUser(runAsUser string) ctrOption {
	return func(c *Ctr) {
		c.RunAsUser = runAsUser
	}
}

func withRunAsGroup(runAsGroup string) ctrOption {
	return func(c *Ctr) {
		c.RunAsGroup = runAsGroup
	}
}

func withCapAdd(caps []string) ctrOption {
	return func(c *Ctr) {
		c.CapAdd = caps
		c.Caps = true
	}
}

func withCapDrop(caps []string) ctrOption {
	return func(c *Ctr) {
		c.CapDrop = caps
		c.Caps = true
	}
}

func withPullPolicy(policy string) ctrOption {
	return func(c *Ctr) {
		c.PullPolicy = policy
	}
}

func withHostIP(ip string, port string) ctrOption {
	return func(c *Ctr) {
		c.HostIP = ip
		c.Port = port
	}
}

func withVolumeMount(mountPath, subpath string, readonly bool) ctrOption {
	return func(c *Ctr) {
		c.VolumeMountPath = mountPath
		c.VolumeName = defaultVolName
		c.VolumeReadOnly = readonly
		c.VolumeMount = true
		if len(subpath) > 0 {
			c.VolumeSubPath = subpath
		}
	}
}

func withEnv(name, value, valueFrom, refName, refKey string, optional bool) ctrOption {
	return func(c *Ctr) {
		e := Env{
			Name:      name,
			Value:     value,
			ValueFrom: valueFrom,
			RefName:   refName,
			RefKey:    refKey,
			Optional:  optional,
		}

		c.Env = append(c.Env, e)
	}
}

func withEnvFrom(name, from string, optional bool) ctrOption {
	return func(c *Ctr) {
		e := EnvFrom{
			Name:     name,
			From:     from,
			Optional: optional,
		}

		c.EnvFrom = append(c.EnvFrom, e)
	}
}

func makeCtrNameInPod(pod *Pod, containerName string) string {
	return fmt.Sprintf("%s-%s", pod.Name, containerName)
}

func getCtrNameInPod(pod *Pod) string {
	return makeCtrNameInPod(pod, defaultCtrName)
}

type HostPath struct {
	Path string
	Type string
}

type PersistentVolumeClaim struct {
	ClaimName string
}

type ConfigMap struct {
	Name        string
	Items       []map[string]string
	Optional    bool
	DefaultMode int32
}

type SecretVol struct {
	SecretName  string
	Items       []map[string]string
	Optional    bool
	DefaultMode int32
}

type EmptyDir struct{}

type Volume struct {
	VolumeType string
	Name       string
	HostPath
	PersistentVolumeClaim
	ConfigMap
	EmptyDir
	SecretVol
}

// getHostPathVolume takes a type and a location for a HostPath
// volume giving it a default name of volName
func getHostPathVolume(vType, vPath string) *Volume {
	return &Volume{
		VolumeType: "HostPath",
		Name:       defaultVolName,
		HostPath: HostPath{
			Path: vPath,
			Type: vType,
		},
	}
}

// getPersistentVolumeClaimVolume takes a name for a Persistentvolumeclaim
// volume giving it a default name of volName
func getPersistentVolumeClaimVolume(vName string) *Volume {
	return &Volume{
		VolumeType: "PersistentVolumeClaim",
		Name:       defaultVolName,
		PersistentVolumeClaim: PersistentVolumeClaim{
			ClaimName: vName,
		},
	}
}

// getConfigMapVolume returns a new ConfigMap Volume given the name and items
// of the ConfigMap.
func getConfigMapVolume(vName string, items []map[string]string, optional bool, defaultMode *int32) *Volume {
	vol := &Volume{
		VolumeType: "ConfigMap",
		Name:       defaultVolName,
		ConfigMap: ConfigMap{
			Name:        vName,
			Items:       items,
			Optional:    optional,
			DefaultMode: v1.ConfigMapVolumeSourceDefaultMode,
		},
	}
	if defaultMode != nil {
		vol.ConfigMap.DefaultMode = *defaultMode
	}
	return vol
}

func getSecretVolume(vName string, items []map[string]string, optional bool, defaultMode *int32) *Volume {
	vol := &Volume{
		VolumeType: "Secret",
		Name:       defaultVolName,
		SecretVol: SecretVol{
			SecretName:  vName,
			Items:       items,
			Optional:    optional,
			DefaultMode: v1.SecretVolumeSourceDefaultMode,
		},
	}
	if defaultMode != nil {
		vol.SecretVol.DefaultMode = *defaultMode
	}
	return vol
}

func getEmptyDirVolume() *Volume {
	return &Volume{
		VolumeType: "EmptyDir",
		Name:       defaultVolName,
		EmptyDir:   EmptyDir{},
	}
}

type Env struct {
	Name      string
	Value     string
	ValueFrom string
	RefName   string
	RefKey    string
	Optional  bool
}

type EnvFrom struct {
	Name     string
	From     string
	Optional bool
}

func milliCPUToQuota(milliCPU string) int {
	milli, _ := strconv.Atoi(strings.Trim(milliCPU, "m"))
	return milli * defaultCPUPeriod
}

func createSourceTarFile(fileName, fileContent, tarFilePath string) error {
	dir := GinkgoT().TempDir()

	file, err := os.Create(filepath.Join(dir, fileName))
	if err != nil {
		return err
	}

	_, err = file.WriteString(fileContent)
	if err != nil {
		return err
	}

	err = file.Close()
	if err != nil {
		return err
	}

	tarFile, err := os.Create(tarFilePath)
	if err != nil {
		return err
	}
	defer tarFile.Close()

	return utils.TarToFilesystem(dir, tarFile)
}

func createAndTestSecret(podmanTest *PodmanTestIntegration, secretYamlString, secretName, fileName string) {
	err := writeYaml(secretYamlString, fileName)
	Expect(err).ToNot(HaveOccurred())

	kube := podmanTest.Podman([]string{"kube", "play", fileName})
	kube.WaitWithDefaultTimeout()
	Expect(kube).Should(ExitCleanly())

	secretList := podmanTest.Podman([]string{"secret", "list"})
	secretList.WaitWithDefaultTimeout()
	Expect(secretList).Should(ExitCleanly())
	Expect(secretList.OutputToString()).Should(ContainSubstring(secretName))

	// test if secret ID is printed once created
	secretListQuiet := podmanTest.Podman([]string{"secret", "list", "--quiet"})
	secretListQuiet.WaitWithDefaultTimeout()
	Expect(secretListQuiet).Should(ExitCleanly())
	Expect(kube.OutputToString()).Should(ContainSubstring(secretListQuiet.OutputToString()))
}

func deleteAndTestSecret(podmanTest *PodmanTestIntegration, secretName string) {
	secretRm := podmanTest.Podman([]string{"secret", "rm", secretName})
	secretRm.WaitWithDefaultTimeout()
	Expect(secretRm).Should(ExitCleanly())
}

func testPodWithSecret(podmanTest *PodmanTestIntegration, podYamlString, fileName string, succeed, exists bool) {
	err := writeYaml(podYamlString, fileName)
	Expect(err).ToNot(HaveOccurred())

	kube := podmanTest.Podman([]string{"kube", "play", fileName})
	kube.WaitWithDefaultTimeout()
	if !succeed {
		Expect(kube).Should(Exit(-1))
		return
	}
	Expect(kube).Should(ExitCleanly())

	exec := podmanTest.Podman([]string{"exec", "mypod-myctr", "cat", "/etc/foo/username"})
	exec.WaitWithDefaultTimeout()
	if exists {
		Expect(exec).Should(ExitCleanly())
		username, _ := base64.StdEncoding.DecodeString("dXNlcg==")
		Expect(exec.OutputToString()).Should(ContainSubstring(string(username)))
	} else {
		Expect(exec).Should(Exit(-1))
	}

	podRm := podmanTest.Podman([]string{"pod", "rm", "-f", "mypod"})
	podRm.WaitWithDefaultTimeout()
	Expect(podRm).Should(ExitCleanly())
}

func testHTTPServer(port string, shouldErr bool, expectedResponse string) {
	address := url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort("localhost", port),
	}

	interval := 250 * time.Millisecond
	var err error
	var resp *http.Response
	for range 6 {
		resp, err = http.Get(address.String())
		if err != nil && shouldErr {
			Expect(err.Error()).To(ContainSubstring(expectedResponse))
			return
		}
		if err == nil {
			defer resp.Body.Close()
			break
		}
		time.Sleep(interval)
		interval *= 2
	}
	Expect(err).ToNot(HaveOccurred())

	body, err := io.ReadAll(resp.Body)
	Expect(err).ToNot(HaveOccurred())
	Expect(string(body)).Should(Equal(expectedResponse))
}

func verifyPodPorts(podmanTest *PodmanTestIntegration, podName string, ports ...string) {
	podInspect := podmanTest.Podman([]string{"pod", "inspect", podName, "--format", "{{.InfraContainerID}}"})
	podInspect.WaitWithDefaultTimeout()
	Expect(podInspect).To(ExitCleanly())
	infraID := podInspect.OutputToString()

	inspect := podmanTest.Podman([]string{"inspect", "--format", "{{.NetworkSettings.Ports}}", infraID})
	inspect.WaitWithDefaultTimeout()
	Expect(inspect).To(ExitCleanly())

	for _, port := range ports {
		Expect(inspect.OutputToString()).Should(ContainSubstring(port))
	}
}

var _ = Describe("Podman kube play", func() {
	var kubeYaml string

	BeforeEach(func() {
		kubeYaml = filepath.Join(podmanTest.TempDir, "kube.yaml")
	})

	It("all arguments should be read", func() {
		pods := []string{"testPod1", "testPod2", "testPod3", "testPod4"}
		cmd := []string{"kube", "play"}

		for _, name := range pods {
			kubeYaml = filepath.Join(podmanTest.TempDir, name+".yaml")

			cmd = append(cmd, kubeYaml)

			pod := getPod(withPodName(name))

			err := generateKubeYaml("pod", pod, kubeYaml)
			Expect(err).ToNot(HaveOccurred())
		}

		podmanTest.PodmanExitCleanly(cmd...)

		ids := []string{}
		for _, name := range pods {
			inspect := podmanTest.PodmanExitCleanly(
				"pod", "inspect", "--format", "{{.ID}}@{{.Name}}:{{.State}}", name,
			)
			output := inspect.OutputToString()
			id, state, found := strings.Cut(output, "@")
			Expect(found).To(BeTrue())
			Expect(state).To(Equal(name + ":Running"))
			ids = append(ids, id)

		}

		teardownCmd := []string{"kube", "down"}
		teardownCmd = append(teardownCmd, cmd[2:]...)
		teardown := podmanTest.PodmanExitCleanly(teardownCmd...)
		teardownOutput := teardown.OutputToString()
		for _, id := range ids {
			Expect(teardownOutput).Should(ContainSubstring(id))
		}
	})

	It("[play kube] fail with yaml of unsupported kind", func() {
		err := writeYaml(unknownKindYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		expect := "YAML document does not contain any supported kube kind"
		// On anything kube-related, podman-remote emits a magic prefix
		// that regular podman doesn't. Test for it here, but let's not
		// do so in every single test.
		if IsRemote() {
			expect = "playing YAML file: " + expect
		}
		Expect(kube).To(ExitWithError(125, expect))
	})

	It("fail with custom selinux label", func() {
		if !selinux.GetEnabled() {
			Skip("SELinux not enabled")
		}
		err := writeYaml(selinuxLabelPodYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", "label-pod-test", "--format", "'{{ .ProcessLabel }}'"})
		inspect.WaitWithDefaultTimeout()
		label := inspect.OutputToString()

		Expect(label).To(ContainSubstring("unconfined_u:system_r:spc_t:s0"))
	})

	It("--no-hostname", func() {
		err := writeYaml(checkInfraImagePodYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", "--no-hostname", kubeYaml)
		alpineHostname := podmanTest.PodmanExitCleanly("run", "--rm", "--no-hostname", ALPINE, "cat", "/etc/hostname")

		podInspect := podmanTest.PodmanExitCleanly("pod", "inspect", "check-infra-image")

		data := podInspect.InspectPodToJSON()
		for _, ctr := range data.Containers {
			if strings.HasSuffix(ctr.Name, "-infra") {
				continue
			}
			exec := podmanTest.PodmanExitCleanly("exec", ctr.ID, "cat", "/etc/hostname")
			Expect(exec.OutputToString()).To(Equal(alpineHostname.OutputToString()))
		}
	})

	It("--no-host", func() {
		err := writeYaml(checkInfraImagePodYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", "--no-hosts", kubeYaml)
		podInspect := podmanTest.PodmanExitCleanly("pod", "inspect", "check-infra-image")
		data := podInspect.InspectPodToJSON()
		for _, ctr := range data.Containers {
			if strings.HasSuffix(ctr.Name, "-infra") {
				continue
			}
			exec := podmanTest.PodmanExitCleanly("exec", ctr.ID, "cat", "/etc/hosts")
			Expect(exec.OutputToString()).To(Not(ContainSubstring("check-infra-image")))
		}
	})

	It("with non-existing configmap", func() {
		err := writeYaml(podWithoutConfigMapDefined, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitWithError(125, `failed to create volume "mycm": no such ConfigMap "mycm"`))
	})

	It("test HostAliases with --no-hosts", func() {
		pod := getPod(withHostAliases("192.168.1.2", []string{
			"test1.podman.io",
			"test2.podman.io",
		}),
			withHostAliases("192.168.1.3", []string{
				"test3.podman.io",
				"test4.podman.io",
			}),
		)
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", "--no-hosts", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitWithError(125, "HostAliases in yaml file will not work with --no-hosts"))
	})

	It("should use customized infra_image", func() {
		conffile := filepath.Join(podmanTest.TempDir, "container.conf")

		infraImage := INFRA_IMAGE
		err := os.WriteFile(conffile, fmt.Appendf(nil, "[engine]\ninfra_image=\"%s\"\n", infraImage), 0o644)
		Expect(err).ToNot(HaveOccurred())

		os.Setenv("CONTAINERS_CONF", conffile)

		if IsRemote() {
			podmanTest.RestartRemoteService()
		}

		err = writeYaml(checkInfraImagePodYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		podInspect := podmanTest.Podman([]string{"inspect", "check-infra-image", "--format", "{{ .InfraContainerID }}"})
		podInspect.WaitWithDefaultTimeout()
		infraContainerID := podInspect.OutputToString()

		conInspect := podmanTest.Podman([]string{"inspect", infraContainerID, "--format", "{{ .ImageName }}"})
		conInspect.WaitWithDefaultTimeout()
		infraContainerImage := conInspect.OutputToString()
		Expect(infraContainerImage).To(Equal(infraImage))
	})

	It("should share ipc,net,uts when shareProcessNamespace is set", func() {
		SkipIfRootless("Requires root privileges for sharing few namespaces")
		err := writeYaml(sharedNamespacePodYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", "testpod1", "--format", "'{{ .SharedNamespaces }}'"})
		inspect.WaitWithDefaultTimeout()
		sharednamespaces := inspect.OutputToString()
		Expect(sharednamespaces).To(ContainSubstring("ipc"))
		Expect(sharednamespaces).To(ContainSubstring("net"))
		Expect(sharednamespaces).To(ContainSubstring("uts"))
		Expect(sharednamespaces).To(ContainSubstring("pid"))
	})

	It("should be able to run image where workdir is a symlink", func() {
		session := podmanTest.Podman([]string{
			"build", "-f", "build/workdir-symlink/Dockerfile", "-t", "test-symlink",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		err := writeYaml(workdirSymlinkPodYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		wait := podmanTest.Podman([]string{"wait", "test-symlink-test-symlink"})
		wait.WaitWithDefaultTimeout()
		Expect(wait).Should(ExitCleanly())
		Expect(wait.OutputToString()).To(Equal("0"))

		logs := podmanTest.Podman([]string{"pod", "logs", "-c", "test-symlink-test-symlink", "test-symlink"})
		logs.WaitWithDefaultTimeout()
		Expect(logs).Should(ExitCleanly())
		Expect(logs.OutputToString()).To(ContainSubstring("hello"))
	})

	It("should not rename pod if container in pod has same name", func() {
		err := writeYaml(podnameEqualsContainerNameYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		testPodCreated := podmanTest.Podman([]string{"pod", "exists", "podnameEqualsContainerNameYaml"})
		testPodCreated.WaitWithDefaultTimeout()
		Expect(testPodCreated).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", "podnameEqualsContainerNameYaml"})
		inspect.WaitWithDefaultTimeout()
		podInspect := inspect.InspectPodArrToJSON()
		Expect(podInspect).Should(HaveLen(1))
		var containerNames []string
		for _, container := range podInspect[0].Containers {
			containerNames = append(containerNames, container.Name)
		}
		Expect(containerNames).To(ContainElement("podnameEqualsContainerNameYaml-podnameEqualsContainerNameYaml"))
	})

	It("should error if pod doesn't have a name", func() {
		err := writeYaml(podWithoutAName, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitWithError(125, "pod does not have a name"))
	})

	It("should error if pod doesn't have an image", func() {
		err := writeYaml(podWithoutAnImage, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitWithError(125, `container "podDoesntHaveAnImage" is missing the required 'image' field`))
	})

	It("support container liveness probe", func() {
		err := writeYaml(livenessProbePodYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", "liveness-probe-pod-testimage", "--format", "'{{ .Config.Healthcheck }}'"})
		inspect.WaitWithDefaultTimeout()
		healthcheckcmd := inspect.OutputToString()
		// check if CMD-SHELL based equivalent health check is added to container
		Expect(healthcheckcmd).To(ContainSubstring("[CMD echo hello]"))
	})

	It("liveness probe should fail", func() {
		err := writeYaml(livenessProbeUnhealthyPodYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		time.Sleep(2 * time.Second)
		hc := podmanTest.Podman([]string{"healthcheck", "run", "liveness-unhealthy-probe-pod-testimage"})
		hc.WaitWithDefaultTimeout()
		hcoutput := hc.OutputToString()
		Expect(hcoutput).To(ContainSubstring(define.HealthCheckUnhealthy))
	})

	It("support container startup probe", func() {
		ctrName := "startup-healthy-probe-pod-testimage"
		err := writeYaml(startupProbePodYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		time.Sleep(2 * time.Second)
		inspect := podmanTest.InspectContainer(ctrName)
		Expect(inspect[0].State.Health).To(HaveField("Status", "starting"))

		hc := podmanTest.Podman([]string{"healthcheck", "run", ctrName})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(ExitWithError(1, ""))

		exec := podmanTest.Podman([]string{"exec", ctrName, "sh", "-c", "echo 'startup probe success' > /testfile"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(ExitCleanly())

		hc = podmanTest.Podman([]string{"healthcheck", "run", ctrName})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(ExitCleanly())

		inspect = podmanTest.InspectContainer(ctrName)
		Expect(inspect[0].State.Health).To(HaveField("Status", define.HealthCheckHealthy))
	})

	It("fail with nonexistent authfile", func() {
		err := generateKubeYaml("pod", getPod(), kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", "--authfile", "/tmp/nonexistent", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError(125, "credential file is not accessible: faccessat /tmp/nonexistent: no such file or directory"))
	})

	It("test correct command", func() {
		pod := getPod()
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Cmd }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		cmd := inspect.OutputToString()

		inspect = podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Entrypoint }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		ep := inspect.OutputToString()

		// Use the defined command to override the image's command
		Expect(ep).To(ContainSubstring(strings.Join(defaultCtrCmd, " ")))
		Expect(cmd).To(ContainSubstring(strings.Join(defaultCtrArg, " ")))
	})

	// If you do not supply command or args for a Container, the defaults defined in the Docker image are used.
	It("test correct args and cmd when not specified", func() {
		pod := getPod(withCtr(getCtr(withImage(REGISTRY_IMAGE), withCmd(nil), withArg(nil))))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		// this image's ENTRYPOINT is `/entrypoint.sh`
		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Entrypoint }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(ContainSubstring(`/entrypoint.sh`))

		// and its COMMAND is `/etc/docker/registry/config.yml`
		inspect = podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Cmd }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(ContainSubstring(`[/etc/docker/registry/config.yml]`))
	})

	// If you supply a command but no args for a Container, only the supplied command is used.
	// The default EntryPoint and the default Cmd defined in the Docker image are ignored.
	It("test correct command with only set command in yaml file", func() {
		pod := getPod(withCtr(getCtr(withImage(REGISTRY_IMAGE), withCmd([]string{"echo", "hello"}), withArg(nil))))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		// Use the defined command to override the image's command, and don't set the args
		// so the full command in result should not contains the image's command
		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Entrypoint }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(ContainSubstring(`echo hello`))

		inspect = podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Cmd }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		// an empty command is reported as '[]'
		Expect(inspect.OutputToString()).To(ContainSubstring(`[]`))
	})

	// If you have an init container in the pod yaml, podman should create and run the init container with kube play
	// With annotation set to always
	It("test with init containers and annotation set", func() {
		// With the init container type annotation set to always
		pod := getPod(withAnnotation("io.podman.annotations.init.container.type", "always"), withPodInitCtr(getCtr(withImage(CITEST_IMAGE), withCmd([]string{"printenv", "container"}), withInitCtr(), withName("init-test"))), withCtr(getCtr(withImage(CITEST_IMAGE), withCmd([]string{"top"}))))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		// Expect the number of containers created to be 3, one init, infra, and regular container
		numOfCtrs := podmanTest.NumberOfContainers()
		Expect(numOfCtrs).To(Equal(3))

		// Init container should have exited after running
		inspect := podmanTest.Podman([]string{"inspect", "--format", "{{.State.Status}}", "testPod-init-test"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(ContainSubstring("exited"))

		// Regular container should be in running state
		inspect = podmanTest.Podman([]string{"inspect", "--format", "{{.State.Status}}", "testPod-" + defaultCtrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(ContainSubstring("running"))

		// Init containers should not be restarted
		inspect = podmanTest.Podman([]string{"inspect", "--format", "{{ .HostConfig.RestartPolicy.Name }}", "testPod-" + defaultCtrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(ContainSubstring(define.RestartPolicyNo))

		// Init containers need environment too! #18384
		logs := podmanTest.Podman([]string{"logs", "testPod-init-test"})
		logs.WaitWithDefaultTimeout()
		Expect(logs).Should(ExitCleanly())
		Expect(logs.OutputToString()).To(Equal("podman"))
	})

	// If you have an init container in the pod yaml, podman should create and run the init container with kube play
	// Using default init container type (once)
	It("test with init container type set to default value", func() {
		// Using the default init container type (once)
		pod := getPod(withPodInitCtr(getCtr(withImage(CITEST_IMAGE), withCmd([]string{"echo", "hello"}), withInitCtr(), withName("init-test"))), withCtr(getCtr(withImage(CITEST_IMAGE), withCmd([]string{"top"}))))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		// Expect the number of containers created to be 2, infra and regular container
		numOfCtrs := podmanTest.NumberOfContainers()
		Expect(numOfCtrs).To(Equal(2))

		// Regular container should be in running state
		inspect := podmanTest.Podman([]string{"inspect", "--format", "{{.State.Status}}", "testPod-" + defaultCtrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(ContainSubstring("running"))
	})

	// If you supply only args for a Container, the default Entrypoint defined in the Docker image is run with the args that you supplied.
	It("test correct command with only set args in yaml file", func() {
		pod := getPod(withCtr(getCtr(withImage(REGISTRY_IMAGE), withCmd(nil), withArg([]string{"echo", "hello"}))))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		// this image's ENTRYPOINT is `/entrypoint.sh`
		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Entrypoint }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(ContainSubstring(`/entrypoint.sh`))

		inspect = podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Cmd }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(ContainSubstring(`[echo hello]`))
	})

	// If you supply a command and args,
	// the default Entrypoint and the default Cmd defined in the Docker image are ignored.
	// Your command is run with your args.
	It("test correct command with both set args and cmd in yaml file", func() {
		pod := getPod(withCtr(getCtr(withImage(REGISTRY_IMAGE), withCmd([]string{"echo"}), withArg([]string{"hello"}))))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Entrypoint }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(ContainSubstring(`echo`))

		inspect = podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Cmd }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(ContainSubstring(`[hello]`))
	})

	It("test correct output", func() {
		p := getPod(withCtr(getCtr(withCmd([]string{"echo", "hello"}), withArg([]string{"world"}))))

		err := generateKubeYaml("pod", p, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		wait := podmanTest.Podman([]string{"wait", getCtrNameInPod(p)})
		wait.WaitWithDefaultTimeout()
		Expect(wait).Should(ExitCleanly())

		// Flake prevention: journalctl makes no timeliness guarantees.
		time.Sleep(1 * time.Second)
		logs := podmanTest.Podman([]string{"logs", getCtrNameInPod(p)})
		logs.WaitWithDefaultTimeout()
		Expect(logs).Should(ExitCleanly())
		Expect(logs.OutputToString()).To(ContainSubstring("hello world"))
	})

	It("podman pod logs test", func() {
		SkipIfRemote("podman-remote pod logs -c is mandatory for remote machine")
		p := getPod(withCtr(getCtr(withCmd([]string{"echo", "hello"}), withArg([]string{"world"}))))

		err := generateKubeYaml("pod", p, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		wait := podmanTest.Podman([]string{"wait", getCtrNameInPod(p)})
		wait.WaitWithDefaultTimeout()
		Expect(wait).Should(ExitCleanly())

		logs := podmanTest.Podman([]string{"pod", "logs", p.Name})
		logs.WaitWithDefaultTimeout()
		Expect(logs).Should(ExitCleanly())
		Expect(logs.OutputToString()).To(ContainSubstring("hello world"))
	})

	It("podman-remote pod logs test", func() {
		// -c or --container is required in podman-remote due to api limitation.
		p := getPod(withCtr(getCtr(withCmd([]string{"echo", "hello"}), withArg([]string{"world"}))))

		err := generateKubeYaml("pod", p, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		wait := podmanTest.Podman([]string{"wait", getCtrNameInPod(p)})
		wait.WaitWithDefaultTimeout()
		Expect(wait).Should(ExitCleanly())

		logs := podmanTest.Podman([]string{"pod", "logs", "-c", getCtrNameInPod(p), p.Name})
		logs.WaitWithDefaultTimeout()
		Expect(logs).Should(ExitCleanly())
		Expect(logs.OutputToString()).To(ContainSubstring("hello world"))
	})

	It("test restartPolicy", func() {
		// podName,  set,  expect
		testSli := [][]string{
			{"testPod1", "", "always"}, // Default equal to always
			{"testPod2", "Always", "always"},
			{"testPod3", "OnFailure", "on-failure"},
			{"testPod4", "Never", "no"},
		}
		for _, v := range testSli {
			pod := getPod(withPodName(v[0]), withRestartPolicy(v[1]))
			err := generateKubeYaml("pod", pod, kubeYaml)
			Expect(err).ToNot(HaveOccurred())

			kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
			kube.WaitWithDefaultTimeout()
			Expect(kube).Should(ExitCleanly())

			inspect := podmanTest.Podman([]string{"inspect", pod.Name, "--format", "{{.RestartPolicy}}"})
			inspect.WaitWithDefaultTimeout()
			Expect(inspect).Should(ExitCleanly())
			Expect(inspect.OutputToString()).To(Equal(v[2]))
		}
	})

	It("test env value from configmap", func() {
		cmYamlPathname := filepath.Join(podmanTest.TempDir, "foo-cm.yaml")
		cm := getConfigMap(withConfigMapName("foo"), withConfigMapData("FOO", "foo"))
		err := generateKubeYaml("configmap", cm, cmYamlPathname)
		Expect(err).ToNot(HaveOccurred())

		pod := getPod(withCtr(getCtr(withEnv("FOO", "", "configmap", "foo", "FOO", false))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml, "--configmap", cmYamlPathname})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Env }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(ContainSubstring(`FOO=foo`))
	})

	It("test env value from configmap and --replace should reuse the configmap volume", func() {
		cmYamlPathname := filepath.Join(podmanTest.TempDir, "foo-cm.yaml")
		cm := getConfigMap(withConfigMapName("foo"), withConfigMapData("FOO", "foo"))
		err := generateKubeYaml("configmap", cm, cmYamlPathname)
		Expect(err).ToNot(HaveOccurred())

		pod := getPod(withCtr(getCtr(withEnv("FOO", "", "configmap", "foo", "FOO", false))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml, "--configmap", cmYamlPathname})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		// create pod again with --replace
		kube = podmanTest.Podman([]string{"kube", "play", "--replace", kubeYaml, "--configmap", cmYamlPathname})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Env }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(ContainSubstring(`FOO=foo`))
	})

	It("test required env value from configmap with missing key", func() {
		cmYamlPathname := filepath.Join(podmanTest.TempDir, "foo-cm.yaml")
		cm := getConfigMap(withConfigMapName("foo"), withConfigMapData("FOO", "foo"))
		err := generateKubeYaml("configmap", cm, cmYamlPathname)
		Expect(err).ToNot(HaveOccurred())

		pod := getPod(withCtr(getCtr(withEnv("FOO", "", "configmap", "foo", "MISSING_KEY", false))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml, "--configmap", cmYamlPathname})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError(125, "cannot set env FOO: key MISSING_KEY not found in configmap foo"))
	})

	It("test required env value from missing configmap", func() {
		pod := getPod(withCtr(getCtr(withEnv("FOO", "", "configmap", "missing_cm", "FOO", false))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError(125, "cannot set env FOO: configmap missing_cm not found"))
	})

	It("test optional env value from configmap with missing key", func() {
		cmYamlPathname := filepath.Join(podmanTest.TempDir, "foo-cm.yaml")
		cm := getConfigMap(withConfigMapName("foo"), withConfigMapData("FOO", "foo"))
		err := generateKubeYaml("configmap", cm, cmYamlPathname)
		Expect(err).ToNot(HaveOccurred())

		pod := getPod(withCtr(getCtr(withEnv("FOO", "", "configmap", "foo", "MISSING_KEY", true))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml, "--configmap", cmYamlPathname})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ range .Config.Env }}[{{ . }}]{{end}}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(Not(ContainSubstring(`[FOO=]`)))
	})

	It("test optional env value from missing configmap", func() {
		pod := getPod(withCtr(getCtr(withEnv("FOO", "", "configmap", "missing_cm", "FOO", true))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ range .Config.Env }}[{{ . }}]{{end}}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(Not(ContainSubstring(`[FOO=]`)))
	})

	It("test get all key-value pairs from configmap as envs", func() {
		cmYamlPathname := filepath.Join(podmanTest.TempDir, "foo-cm.yaml")
		cm := getConfigMap(withConfigMapName("foo"), withConfigMapData("FOO1", "foo1"), withConfigMapData("FOO2", "foo2"))
		err := generateKubeYaml("configmap", cm, cmYamlPathname)
		Expect(err).ToNot(HaveOccurred())

		pod := getPod(withCtr(getCtr(withEnvFrom("foo", "configmap", false))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml, "--configmap", cmYamlPathname})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Env }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(ContainSubstring(`FOO1=foo1`))
		Expect(inspect.OutputToString()).To(ContainSubstring(`FOO2=foo2`))
	})

	It("test get all key-value pairs from required configmap as envs", func() {
		pod := getPod(withCtr(getCtr(withEnvFrom("missing_cm", "configmap", false))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError(125, "configmap missing_cm not found"))
	})

	It("test get all key-value pairs from optional configmap as envs", func() {
		pod := getPod(withCtr(getCtr(withEnvFrom("missing_cm", "configmap", true))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())
	})

	It("test env value from secret", func() {
		createSecret(podmanTest, "foo", defaultSecret)
		pod := getPod(withCtr(getCtr(withEnv("FOO", "", "secret", "foo", "FOO", false))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Env }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(ContainSubstring(`FOO=foo`))
	})

	It("test required env value from missing secret", func() {
		pod := getPod(withCtr(getCtr(withEnv("FOO", "", "secret", "foo", "FOO", false))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError(125, `cannot set env FOO: no secret with name or id "foo": no such secret`))
	})

	It("test required env value from secret with missing key", func() {
		createSecret(podmanTest, "foo", defaultSecret)
		pod := getPod(withCtr(getCtr(withEnv("FOO", "", "secret", "foo", "MISSING", false))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError(125, "cannot set env FOO: secret foo has not MISSING key"))
	})

	It("test optional env value from missing secret", func() {
		pod := getPod(withCtr(getCtr(withEnv("FOO", "", "secret", "foo", "FOO", true))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ range .Config.Env }}[{{ . }}]{{end}}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(Not(ContainSubstring(`[FOO=]`)))
	})

	It("test optional env value from secret with missing key", func() {
		createSecret(podmanTest, "foo", defaultSecret)
		pod := getPod(withCtr(getCtr(withEnv("FOO", "", "secret", "foo", "MISSING", true))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ range .Config.Env }}[{{ . }}]{{end}}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(Not(ContainSubstring(`[FOO=]`)))
	})

	It("test get all key-value pairs from secret as envs", func() {
		createSecret(podmanTest, "foo", defaultSecret)
		pod := getPod(withCtr(getCtr(withEnvFrom("foo", "secret", false))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Env }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(ContainSubstring(`FOO=foo`))
		Expect(inspect.OutputToString()).To(ContainSubstring(`BAR=bar`))
	})

	It("test get all key-value pairs from required secret as envs", func() {
		pod := getPod(withCtr(getCtr(withEnvFrom("missing_secret", "secret", false))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError(125, `no secret with name or id "missing_secret": no such secret`))
	})

	It("test get all key-value pairs from optional secret as envs", func() {
		pod := getPod(withCtr(getCtr(withEnvFrom("missing_secret", "secret", true))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())
	})

	It("test env value takes precedence over envFrom configmap value", func() {
		cm := getConfigMap(withConfigMapName("test-config-map"), withConfigMapData("MY_ENV_VAR", "1"))
		cmYaml, err := getKubeYaml("configmap", cm)
		Expect(err).ToNot(HaveOccurred())

		pod := getPod(withCtr(getCtr(
			withEnvFrom("test-config-map", "configmap", false),
			withEnv("MY_ENV_VAR", "2", "", "", "", false),
		)))

		podYaml, err := getKubeYaml("pod", pod)
		Expect(err).ToNot(HaveOccurred())

		yamls := []string{cmYaml, podYaml}
		err = generateMultiDocKubeYaml(yamls, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Env }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		// env should override envFrom, so value should be "2" not "1"
		Expect(inspect.OutputToString()).To(ContainSubstring(`MY_ENV_VAR=2`))
	})

	It("test duplicate container name", func() {
		p := getPod(withCtr(getCtr(withName("testctr"), withCmd([]string{"echo", "hello"}))), withCtr(getCtr(withName("testctr"), withCmd([]string{"echo", "world"}))))

		err := generateKubeYaml("pod", p, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError(125, `the pod "testPod" is invalid; duplicate container name "testctr" detected`))

		p = getPod(withPodInitCtr(getCtr(withImage(CITEST_IMAGE), withCmd([]string{"echo", "hello"}), withInitCtr(), withName("initctr"))), withCtr(getCtr(withImage(CITEST_IMAGE), withName("initctr"), withCmd([]string{"top"}))))

		err = generateKubeYaml("pod", p, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube = podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError(125, `adding pod to state: name "testPod" is in use: pod already exists`))
	})

	It("test hostname", func() {
		pod := getPod()
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "{{ .Config.Hostname }}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(Equal(defaultPodName))
	})

	It("test with customized hostname", func() {
		hostname := "myhostname"
		pod := getPod(withHostname(hostname))
		err := generateKubeYaml("pod", getPod(withHostname(hostname)), kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "{{ .Config.Hostname }}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(Equal(hostname))

		hostnameInCtr := podmanTest.Podman([]string{"exec", getCtrNameInPod(pod), "hostname"})
		hostnameInCtr.WaitWithDefaultTimeout()
		Expect(hostnameInCtr).Should(ExitCleanly())
		Expect(hostnameInCtr.OutputToString()).To(Equal(hostname))
	})

	It("test HostAliases", func() {
		pod := getPod(withHostAliases("192.168.1.2", []string{
			"test1.podman.io",
			"test2.podman.io",
		}),
			withHostAliases("192.168.1.3", []string{
				"test3.podman.io",
				"test4.podman.io",
			}),
		)
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", pod.Name, "--format", "{{ .InfraConfig.HostAdd}}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).
			To(Equal("[test1.podman.io:192.168.1.2 test2.podman.io:192.168.1.2 test3.podman.io:192.168.1.3 test4.podman.io:192.168.1.3]"))
	})

	It("cap add", func() {
		capAdd := "CAP_SYS_ADMIN"
		ctr := getCtr(withCapAdd([]string{capAdd}), withCmd([]string{"cat", "/proc/self/status"}), withArg(nil))

		pod := getPod(withCtr(ctr))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod)})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(ContainSubstring(capAdd))
	})

	It("cap drop", func() {
		capDrop := "CAP_CHOWN"
		ctr := getCtr(withCapDrop([]string{capDrop}))

		pod := getPod(withCtr(ctr))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod)})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(ContainSubstring(capDrop))
	})

	It("no security context", func() {
		// expect kube play to not fail if no security context is specified
		pod := getPod(withCtr(getCtr(withSecurityContext(false))))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod)})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
	})

	It("seccomp container level", func() {
		SkipIfRemote("podman-remote does not support --seccomp-profile-root flag")
		// expect kube play is expected to set a seccomp label if it's applied as an annotation
		jsonFile, err := podmanTest.CreateSeccompJSON(seccompLinkEPERM)
		if err != nil {
			GinkgoWriter.Println(err)
			Skip("Failed to prepare seccomp.json for test.")
		}

		ctrAnnotation := "container.seccomp.security.alpha.kubernetes.io/" + defaultCtrName
		ctr := getCtr(withCmd([]string{"ln"}), withArg([]string{"/etc/motd", "/noneShallPass"}))

		pod := getPod(withCtr(ctr), withAnnotation(ctrAnnotation, "localhost/"+filepath.Base(jsonFile)))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		// CreateSeccompJSON will put the profile into podmanTest.TempDir. Use --seccomp-profile-root to tell kube play where to look
		kube := podmanTest.Podman([]string{"kube", "play", "--seccomp-profile-root", podmanTest.TempDir, kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		ctrName := getCtrNameInPod(pod)
		wait := podmanTest.Podman([]string{"wait", ctrName})
		wait.WaitWithDefaultTimeout()
		Expect(wait).Should(Exit(0), "podman wait %s", ctrName)

		logs := podmanTest.Podman([]string{"logs", ctrName})
		logs.WaitWithDefaultTimeout()
		Expect(logs).Should(Exit(0), "podman logs %s", ctrName)
		Expect(logs.ErrorToString()).To(ContainSubstring("ln: /noneShallPass: Operation not permitted"))
	})

	It("seccomp pod level", func() {
		SkipIfRemote("podman-remote does not support --seccomp-profile-root flag")
		// expect kube play is expected to set a seccomp label if it's applied as an annotation
		jsonFile, err := podmanTest.CreateSeccompJSON(seccompLinkEPERM)
		if err != nil {
			GinkgoWriter.Println(err)
			Skip("Failed to prepare seccomp.json for test.")
		}
		defer os.Remove(jsonFile)

		ctr := getCtr(withCmd([]string{"ln"}), withArg([]string{"/etc/motd", "/noPodsShallPass"}))

		pod := getPod(withCtr(ctr), withAnnotation("seccomp.security.alpha.kubernetes.io/pod", "localhost/"+filepath.Base(jsonFile)))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		// CreateSeccompJSON will put the profile into podmanTest.TempDir. Use --seccomp-profile-root to tell kube play where to look
		kube := podmanTest.Podman([]string{"kube", "play", "--seccomp-profile-root", podmanTest.TempDir, kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		podName := getCtrNameInPod(pod)
		wait := podmanTest.Podman([]string{"wait", podName})
		wait.WaitWithDefaultTimeout()
		Expect(wait).Should(ExitCleanly())

		logs := podmanTest.Podman([]string{"logs", podName})
		logs.WaitWithDefaultTimeout()
		Expect(logs).Should(Exit(0))
		Expect(logs.ErrorToString()).To(ContainSubstring("ln: /noPodsShallPass: Operation not permitted"))
	})

	It("with pull policy of never should be 125", func() {
		ctr := getCtr(withPullPolicy("never"), withImage(BB_GLIBC))
		err := generateKubeYaml("pod", getPod(withCtr(ctr)), kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitWithError(125, BB_GLIBC+": image not known"))
	})

	It("with pull policy of missing", func() {
		ctr := getCtr(withPullPolicy("Missing"), withImage(CITEST_IMAGE))
		err := generateKubeYaml("pod", getPod(withCtr(ctr)), kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())
	})

	It("with pull always", func() {
		oldBB := "quay.io/libpod/busybox:1.30.1"
		pull := podmanTest.Podman([]string{"pull", oldBB})
		pull.WaitWithDefaultTimeout()

		tag := podmanTest.Podman([]string{"tag", oldBB, BB})
		tag.WaitWithDefaultTimeout()
		Expect(tag).Should(ExitCleanly())

		rmi := podmanTest.Podman([]string{"rmi", oldBB})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", BB})
		inspect.WaitWithDefaultTimeout()
		oldBBinspect := inspect.InspectImageJSON()

		ctr := getCtr(withPullPolicy("always"), withImage(BB))
		err := generateKubeYaml("pod", getPod(withCtr(ctr)), kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		// Cannot ExitCleanly() because pull output goes to stderr
		Expect(kube).Should(Exit(0))

		inspect = podmanTest.Podman([]string{"inspect", BB})
		inspect.WaitWithDefaultTimeout()
		newBBinspect := inspect.InspectImageJSON()
		Expect(oldBBinspect[0].Digest).To(Not(Equal(newBBinspect[0].Digest)))
	})

	It("with latest image should always pull", func() {
		oldBB := "quay.io/libpod/busybox:1.30.1"
		pull := podmanTest.Podman([]string{"pull", oldBB})
		pull.WaitWithDefaultTimeout()
		Expect(pull).Should(Exit(0))

		tag := podmanTest.Podman([]string{"tag", oldBB, BB})
		tag.WaitWithDefaultTimeout()
		Expect(tag).Should(ExitCleanly())

		rmi := podmanTest.Podman([]string{"rmi", oldBB})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", BB})
		inspect.WaitWithDefaultTimeout()
		oldBBinspect := inspect.InspectImageJSON()

		ctr := getCtr(withImage(BB), withPullPolicy(""))
		err := generateKubeYaml("pod", getPod(withCtr(ctr)), kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		// Cannot ExitCleanly() because pull output goes to stderr
		Expect(kube).Should(Exit(0))

		inspect = podmanTest.Podman([]string{"inspect", BB})
		inspect.WaitWithDefaultTimeout()
		newBBinspect := inspect.InspectImageJSON()
		Expect(oldBBinspect[0].Digest).To(Not(Equal(newBBinspect[0].Digest)))
	})

	It("with no tag and no pull policy should always pull", func() {
		oldBB := "quay.io/libpod/busybox:1.30.1"
		pull := podmanTest.Podman([]string{"pull", oldBB})
		pull.WaitWithDefaultTimeout()
		Expect(pull).Should(Exit(0))

		tag := podmanTest.Podman([]string{"tag", oldBB, BB})
		tag.WaitWithDefaultTimeout()
		Expect(tag).Should(ExitCleanly())

		rmi := podmanTest.Podman([]string{"rmi", oldBB})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", BB})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		oldBBinspect := inspect.InspectImageJSON()

		noTagBB := "quay.io/libpod/busybox"
		ctr := getCtr(withImage(noTagBB), withPullPolicy(""))
		err := generateKubeYaml("pod", getPod(withCtr(ctr)), kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))
		if IsRemote() {
			Expect(kube.ErrorToString()).To(BeEmpty())
		} else {
			Expect(kube.ErrorToString()).To(ContainSubstring("Copying blob "))
		}

		inspect = podmanTest.Podman([]string{"inspect", noTagBB})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		newBBinspect := inspect.InspectImageJSON()
		Expect(oldBBinspect[0].Digest).To(Not(Equal(newBBinspect[0].Digest)))
	})

	It("with image data", func() {
		testyaml := `
apiVersion: v1
kind: Pod
metadata:
  name: demo_pod
spec:
  containers:
  - image: demo
    name: demo_kube
`
		pull := podmanTest.Podman([]string{"create", "--workdir", "/etc", "--name", "newBB", "--label", "key1=value1", CITEST_IMAGE})

		pull.WaitWithDefaultTimeout()
		Expect(pull).Should(ExitCleanly())

		c := podmanTest.Podman([]string{"commit", "-q", "-c", "STOPSIGNAL=51", "newBB", "demo"})
		c.WaitWithDefaultTimeout()
		Expect(c).Should(ExitCleanly())

		conffile := filepath.Join(podmanTest.TempDir, "kube.yaml")

		err := os.WriteFile(conffile, []byte(testyaml), 0o755)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", conffile})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", "demo_pod-demo_kube"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())

		ctr := inspect.InspectContainerToJSON()
		Expect(ctr[0].Config.WorkingDir).To(ContainSubstring("/etc"))
		Expect(ctr[0].Config.Labels).To(HaveKeyWithValue("key1", ContainSubstring("value1")))
		Expect(ctr[0].Config.Labels).To(HaveKeyWithValue("key1", ContainSubstring("value1")))
		Expect(ctr[0].Config).To(HaveField("StopSignal", "SIGRTMAX-13"))
	})

	It("daemonset sanity", func() {
		daemonset := getDaemonSet()
		err := generateKubeYaml("daemonset", daemonset, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		podName := getPodNameInDaemonSet(daemonset)
		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(&podName), "--format", "'{{ .Config.Entrypoint }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		// yaml's command should override the image's Entrypoint
		Expect(inspect.OutputToString()).To(ContainSubstring(strings.Join(defaultCtrCmd, " ")))
	})

	// Deployment related tests
	It("deployment 1 replica test correct command", func() {
		deployment := getDeployment()
		err := generateKubeYaml("deployment", deployment, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		podName := getPodNameInDeployment(deployment)
		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(&podName), "--format", "'{{ .Config.Entrypoint }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		// yaml's command should override the image's Entrypoint
		Expect(inspect.OutputToString()).To(ContainSubstring(strings.Join(defaultCtrCmd, " ")))
	})

	It("deployment more than 1 replica test correct command", func() {
		var numReplicas int32 = 5
		deployment := getDeployment(withReplicas(numReplicas))
		err := generateKubeYaml("deployment", deployment, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))
		if IsRemote() {
			Expect(kube.ErrorToString()).To(BeEmpty())
		} else {
			Expect(kube.ErrorToString()).To(ContainSubstring("Limiting replica count to 1, use `--replicas` to enable more than one replica"))
		}

		podName := getPodNameInDeployment(deployment)

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(&podName), "--format", "'{{ .Config.Entrypoint }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(ContainSubstring(strings.Join(defaultCtrCmd, " ")))
	})

	It("job sanity", func() {
		job := getJob()
		err := generateKubeYaml("job", job, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		podName := getPodNameInJob(job)
		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(&podName), "--format", "'{{ .Config.Entrypoint }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		// yaml's command should override the image's Entrypoint
		Expect(inspect.OutputToString()).To(ContainSubstring(strings.Join(defaultCtrCmd, " ")))
	})

	It("--ip and --mac-address", func() {
		var i, numReplicas int32
		numReplicas = 3
		deployment := getDeployment(withReplicas(numReplicas))
		err := generateKubeYaml("deployment", deployment, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		net := "playkube" + stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"network", "create", "--subnet", "10.25.31.0/24", net})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net)
		Expect(session).Should(ExitCleanly())

		ips := []string{"10.25.31.5", "10.25.31.10", "10.25.31.15"}
		playArgs := []string{"kube", "play", "--network", net}
		for _, ip := range ips {
			playArgs = append(playArgs, "--ip", ip)
		}
		macs := []string{"e8:d8:82:c9:80:40", "e8:d8:82:c9:80:50", "e8:d8:82:c9:80:60"}
		for _, mac := range macs {
			playArgs = append(playArgs, "--mac-address", mac)
		}

		kube := podmanTest.Podman(append(playArgs, kubeYaml))
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))
		if IsRemote() {
			Expect(kube.ErrorToString()).To(BeEmpty())
		} else {
			Expect(kube.ErrorToString()).To(ContainSubstring("Limiting replica count to 1, use `--replicas` to enable more than one replica"))
		}

		podName := getPodNameInDeployment(deployment)

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(&podName), "--format", "{{ .NetworkSettings.Networks." + net + ".IPAddress }}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(Equal(ips[i]))

		inspect = podmanTest.Podman([]string{"inspect", getCtrNameInPod(&podName), "--format", "{{ .NetworkSettings.Networks." + net + ".MacAddress }}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(Equal(macs[i]))
	})

	It("with multiple networks", func() {
		ctr := getCtr(withImage(CITEST_IMAGE))
		pod := getPod(withCtr(ctr))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		net1 := "net1" + stringid.GenerateRandomID()
		net2 := "net2" + stringid.GenerateRandomID()

		net := podmanTest.Podman([]string{"network", "create", "--subnet", "10.0.11.0/24", net1})
		net.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net1)
		Expect(net).Should(ExitCleanly())

		net = podmanTest.Podman([]string{"network", "create", "--subnet", "10.0.12.0/24", net2})
		net.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net2)
		Expect(net).Should(ExitCleanly())

		ip1 := "10.0.11.5"
		ip2 := "10.0.12.10"

		kube := podmanTest.Podman([]string{"kube", "play", "--network", net1 + ":ip=" + ip1, "--network", net2 + ":ip=" + ip2, kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"exec", getCtrNameInPod(pod), "ip", "addr"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(ContainSubstring(ip1))
		Expect(inspect.OutputToString()).To(ContainSubstring(ip2))
		Expect(inspect.OutputToString()).To(ContainSubstring("eth0"))
		Expect(inspect.OutputToString()).To(ContainSubstring("eth1"))
	})

	It("test with network portbindings", func() {
		ip := "127.0.0.100"
		port := "8087"
		ctr := getCtr(withHostIP(ip, port), withImage(CITEST_IMAGE))

		pod := getPod(withCtr(ctr))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"port", getCtrNameInPod(pod)})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(Equal("8087/tcp -> 127.0.0.100:8087"))
	})

	It("test with nonexistent empty HostPath type volume", func() {
		hostPathLocation := filepath.Join(tempdir, "file")

		pod := getPod(withVolume(getHostPathVolume(`""`, hostPathLocation)))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError(125, fmt.Sprintf(`failed to create volume "testVol": in parsing HostPath in YAML: faccessat %s: no such file or directory`, hostPathLocation)))
		Expect(kube.ErrorToString()).To(ContainSubstring(defaultVolName))
	})

	It("test with empty HostPath type volume", func() {
		hostPathLocation := filepath.Join(tempdir, "file")
		f, err := os.Create(hostPathLocation)
		Expect(err).ToNot(HaveOccurred())
		f.Close()

		pod := getPod(withVolume(getHostPathVolume(`""`, hostPathLocation)))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())
	})

	It("test with nonexistent File HostPath type volume", func() {
		hostPathLocation := filepath.Join(tempdir, "file")

		pod := getPod(withVolume(getHostPathVolume("File", hostPathLocation)))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError(125, fmt.Sprintf(`failed to create volume "testVol": in parsing HostPath in YAML: faccessat %s: no such file or directory`, hostPathLocation)))
	})

	It("test with File HostPath type volume", func() {
		hostPathLocation := filepath.Join(tempdir, "file")
		f, err := os.Create(hostPathLocation)
		Expect(err).ToNot(HaveOccurred())
		f.Close()

		pod := getPod(withVolume(getHostPathVolume("File", hostPathLocation)))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())
	})

	It("test with FileOrCreate HostPath type volume", func() {
		hostPathLocation := filepath.Join(tempdir, "file")

		pod := getPod(withVolume(getHostPathVolume("FileOrCreate", hostPathLocation)))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		// the file should have been created
		_, err = os.Stat(hostPathLocation)
		Expect(err).ToNot(HaveOccurred())
	})

	It("test with DirectoryOrCreate HostPath type volume", func() {
		hostPathLocation := filepath.Join(tempdir, "file")

		pod := getPod(withVolume(getHostPathVolume("DirectoryOrCreate", hostPathLocation)))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		// the file should have been created
		st, err := os.Stat(hostPathLocation)
		Expect(err).ToNot(HaveOccurred())
		Expect(st.Mode().IsDir()).To(BeTrue(), "hostPathLocation is a directory")
	})

	It("test with DirectoryOrCreate HostPath type volume and non-existent directory path", func() {
		hostPathLocation := filepath.Join(filepath.Join(tempdir, "dir1"), "dir2")

		pod := getPod(withVolume(getHostPathVolume("DirectoryOrCreate", hostPathLocation)))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		// the full path should have been created
		st, err := os.Stat(hostPathLocation)
		Expect(err).ToNot(HaveOccurred())
		Expect(st.Mode().IsDir()).To(BeTrue(), "hostPathLocation is a directory")
	})

	It("test with DirectoryOrCreate HostPath type volume and existent directory path", func() {
		hostPathLocation := filepath.Join(filepath.Join(tempdir, "dir1"), "dir2")
		Expect(os.MkdirAll(hostPathLocation, os.ModePerm)).To(Succeed())

		pod := getPod(withVolume(getHostPathVolume("DirectoryOrCreate", hostPathLocation)))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())
	})

	It("test with Socket HostPath type volume should fail if not socket", func() {
		hostPathLocation := filepath.Join(tempdir, "file")
		f, err := os.Create(hostPathLocation)
		Expect(err).ToNot(HaveOccurred())
		f.Close()

		pod := getPod(withVolume(getHostPathVolume("Socket", hostPathLocation)))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError(125, fmt.Sprintf(`failed to create volume "testVol": checking HostPathSocket: path %s is not a socket`, hostPathLocation)))
	})

	It("test with read-only HostPath volume", func() {
		hostPathLocation := filepath.Join(tempdir, "file")
		f, err := os.Create(hostPathLocation)
		Expect(err).ToNot(HaveOccurred())
		f.Close()

		ctr := getCtr(withVolumeMount(hostPathLocation, "", true), withImage(CITEST_IMAGE))
		pod := getPod(withVolume(getHostPathVolume("File", hostPathLocation)), withCtr(ctr))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{.HostConfig.Binds}}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())

		correct := fmt.Sprintf("%s:%s:%s", hostPathLocation, hostPathLocation, "ro")
		Expect(inspect.OutputToString()).To(ContainSubstring(correct))
	})

	It("test duplicate volume destination between host path and image volumes", func() {
		// Create host test directory and file
		testdir := "testdir"
		testfile := "testfile"

		hostPathDir := filepath.Join(tempdir, testdir)
		err := os.Mkdir(hostPathDir, 0o755)
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(hostPathDir)

		hostPathDirFile := filepath.Join(hostPathDir, testfile)
		f, err := os.Create(hostPathDirFile)
		Expect(err).ToNot(HaveOccurred())
		f.Close()

		if selinux.GetEnabled() {
			label := SystemExec("chcon", []string{"-t", "container_file_t", hostPathDirFile})
			Expect(label).Should(ExitCleanly())
		}

		// Create container image with named volume
		containerfile := fmt.Sprintf(`
FROM  %s
VOLUME %s`, CITEST_IMAGE, hostPathDir+"/")

		image := "podman-kube-test:podman"
		podmanTest.BuildImage(containerfile, image, "false")

		// Create and kube play pod
		ctr := getCtr(withVolumeMount(hostPathDir+"/", "", false), withImage(image))
		pod := getPod(withCtr(ctr), withVolume(getHostPathVolume("Directory", hostPathDir+"/")))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"exec", getCtrNameInPod(pod), "ls", filepath.Join(hostPathDir, testfile)})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod)})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())

		// If two volumes are specified and share the same destination,
		// only one will be mounted. Host path volumes take precedence.
		ctrJSON := inspect.InspectContainerToJSON()
		Expect(ctrJSON[0].Mounts).To(HaveLen(1))
		Expect(ctrJSON[0].Mounts[0]).To(HaveField("Type", define.TypeBind))
	})

	It("test with PersistentVolumeClaim volume", func() {
		volumeName := "namedVolume"

		ctr := getCtr(withVolumeMount("/test", "", false), withImage(CITEST_IMAGE))
		pod := getPod(withVolume(getPersistentVolumeClaimVolume(volumeName)), withCtr(ctr))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "{{ (index .Mounts 0).Type }}:{{ (index .Mounts 0).Name }}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())

		correct := fmt.Sprintf("volume:%s", volumeName)
		Expect(inspect.OutputToString()).To(Equal(correct))
	})

	It("ConfigMap volume with no items", func() {
		volumeName := "cmVol"
		cm := getConfigMap(withConfigMapName(volumeName), withConfigMapData("FOO", "foobar"))
		cmYaml, err := getKubeYaml("configmap", cm)
		Expect(err).ToNot(HaveOccurred())

		ctr := getCtr(withVolumeMount("/test", "", false), withImage(CITEST_IMAGE))
		pod := getPod(withVolume(getConfigMapVolume(volumeName, []map[string]string{}, false, nil)), withCtr(ctr))
		podYaml, err := getKubeYaml("pod", pod)
		Expect(err).ToNot(HaveOccurred())
		yamls := []string{cmYaml, podYaml}
		err = generateMultiDocKubeYaml(yamls, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		cmData := podmanTest.Podman([]string{"exec", getCtrNameInPod(pod), "cat", "/test/FOO"})
		cmData.WaitWithDefaultTimeout()
		Expect(cmData).Should(ExitCleanly())
		Expect(cmData.OutputToString()).To(Equal("foobar"))
	})

	It("ConfigMap volume with items", func() {
		volumeName := "cmVol"
		cm := getConfigMap(withConfigMapName(volumeName), withConfigMapData("FOO", "foobar"))
		cmYaml, err := getKubeYaml("configmap", cm)
		Expect(err).ToNot(HaveOccurred())
		volumeContents := []map[string]string{{
			"key":  "FOO",
			"path": "BAR",
		}}

		ctr := getCtr(withVolumeMount("/test", "", false), withImage(CITEST_IMAGE))
		pod := getPod(withVolume(getConfigMapVolume(volumeName, volumeContents, false, nil)), withCtr(ctr))
		podYaml, err := getKubeYaml("pod", pod)
		Expect(err).ToNot(HaveOccurred())
		yamls := []string{cmYaml, podYaml}
		err = generateMultiDocKubeYaml(yamls, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		cmData := podmanTest.Podman([]string{"exec", getCtrNameInPod(pod), "cat", "/test/BAR"})
		cmData.WaitWithDefaultTimeout()
		Expect(cmData).Should(ExitCleanly())
		Expect(cmData.OutputToString()).To(Equal("foobar"))

		cmData = podmanTest.Podman([]string{"exec", getCtrNameInPod(pod), "cat", "/test/FOO"})
		cmData.WaitWithDefaultTimeout()
		Expect(cmData).Should(Not(ExitCleanly()))
	})

	It("with a missing optional ConfigMap volume", func() {
		volumeName := "cmVol"

		ctr := getCtr(withVolumeMount("/test", "", false), withImage(CITEST_IMAGE))
		pod := getPod(withVolume(getConfigMapVolume(volumeName, []map[string]string{}, true, nil)), withCtr(ctr))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())
	})

	It("ConfigMap volume with defaultMode set", func() {
		volumeName := "cmVol"
		cm := getConfigMap(withConfigMapName(volumeName), withConfigMapData("FOO", "foobar"))
		cmYaml, err := getKubeYaml("configmap", cm)
		Expect(err).ToNot(HaveOccurred())

		ctr := getCtr(withVolumeMount("/test", "", false), withImage(CITEST_IMAGE))
		defaultMode := int32(0o777)
		pod := getPod(withVolume(getConfigMapVolume(volumeName, []map[string]string{}, false, &defaultMode)), withCtr(ctr))
		podYaml, err := getKubeYaml("pod", pod)
		Expect(err).ToNot(HaveOccurred())
		yamls := []string{cmYaml, podYaml}
		err = generateMultiDocKubeYaml(yamls, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		cmData := podmanTest.PodmanExitCleanly("exec", getCtrNameInPod(pod), "cat", "/test/FOO")
		Expect(cmData.OutputToString()).To(Equal("foobar"))

		inspect := podmanTest.PodmanExitCleanly("volume", "inspect", volumeName, "--format", "{{.Mountpoint}}")
		Expect(inspect.OutputToStringArray()).To(HaveLen(1))
		path := inspect.OutputToString()

		permData := SystemExec("stat", []string{"-c", "%a", path + "/FOO"})
		permData.WaitWithDefaultTimeout()
		Expect(permData).Should(ExitCleanly())
		Expect(permData.OutputToString()).To(Equal("777"))
	})

	It("configMap as volume with no defaultMode set", func() {
		cmYaml := `
kind: ConfigMap
apiVersion: v1
metadata:
  name: example-configmap
data:
  foo: bar
---
apiVersion: v1
kind: Pod
metadata:
  name: youthfulshaw-pod
spec:
  containers:
  - command:
    - sleep
    - "1000"
    image: alpine
    name: youthfulshaw
    volumeMounts:
    - name: cm-volume
      mountPath: /test
  volumes:
  - name: cm-volume
    configMap:
     name: example-configmap
`

		err := writeYaml(cmYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())
		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		cmData := podmanTest.PodmanExitCleanly("exec", "youthfulshaw-pod-youthfulshaw", "cat", "/test/foo")
		Expect(cmData.OutputToString()).To(Equal("bar"))

		inspect := podmanTest.PodmanExitCleanly("volume", "inspect", "example-configmap", "--format", "{{.Mountpoint}}")
		Expect(inspect.OutputToStringArray()).To(HaveLen(1))
		path := inspect.OutputToString()

		permData := SystemExec("stat", []string{"-c", "%a", path + "/foo"})
		permData.WaitWithDefaultTimeout()
		Expect(permData).Should(ExitCleanly())
		Expect(permData.OutputToString()).To(Equal("644"))
	})

	It("with emptyDir volume", func() {
		podName := "test-pod"
		ctrName1 := "vol-test-ctr"
		ctrName2 := "vol-test-ctr-2"
		ctr1 := getCtr(withVolumeMount("/test-emptydir", "", false), withImage(CITEST_IMAGE), withName(ctrName1))
		ctr2 := getCtr(withVolumeMount("/test-emptydir-2", "", false), withImage(CITEST_IMAGE), withName(ctrName2))
		pod := getPod(withPodName(podName), withVolume(getEmptyDirVolume()), withCtr(ctr1), withCtr(ctr2))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		podmanTest.PodmanExitCleanly("exec", podName+"-"+ctrName1, "ls", "/test-emptydir")
		podmanTest.PodmanExitCleanly("exec", podName+"-"+ctrName2, "ls", "/test-emptydir-2")

		volList1 := podmanTest.PodmanExitCleanly("volume", "ls", "-q")
		Expect(volList1.OutputToString()).To(Equal(defaultVolName))

		podmanTest.PodmanExitCleanly("pod", "rm", "-f", podName)
		volList2 := podmanTest.PodmanExitCleanly("volume", "ls", "-q")
		Expect(volList2.OutputToString()).To(Equal(""))
	})

	It("applies labels to pods", func() {
		var numReplicas int32 = 5
		expectedLabelKey := "key1"
		expectedLabelValue := "value1"
		deployment := getDeployment(
			withReplicas(numReplicas),
			withPod(getPod(withLabel(expectedLabelKey, expectedLabelValue))),
		)
		err := generateKubeYaml("deployment", deployment, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))
		if IsRemote() {
			Expect(kube.ErrorToString()).To(BeEmpty())
		} else {
			Expect(kube.ErrorToString()).To(ContainSubstring("Limiting replica count to 1, use `--replicas` to enable more than one replica"))
		}

		correctLabels := expectedLabelKey + ":" + expectedLabelValue
		pod := getPodNameInDeployment(deployment)
		inspect := podmanTest.PodmanExitCleanly("pod", "inspect", pod.Name, "--format", "'{{ .Labels }}'")
		Expect(inspect.OutputToString()).To(ContainSubstring(correctLabels))
	})

	It("allows setting resource limits", func() {
		SkipIfContainerized("Resource limits require a running systemd")
		SkipIfRootless("CPU limits require root")
		podmanTest.CgroupManager = "systemd"

		var (
			numReplicas           int32 = 3
			expectedCPURequest          = "100m"
			expectedCPULimit            = "200m"
			expectedMemoryRequest       = "10000000"
			expectedMemoryLimit         = "20000000"
		)

		expectedCPUQuota := milliCPUToQuota(expectedCPULimit)

		deployment := getDeployment(
			withReplicas(numReplicas),
			withPod(getPod(withCtr(getCtr(
				withCPURequest(expectedCPURequest),
				withCPULimit(expectedCPULimit),
				withMemoryRequest(expectedMemoryRequest),
				withMemoryLimit(expectedMemoryLimit),
			)))))
		err := generateKubeYaml("deployment", deployment, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))
		if IsRemote() {
			Expect(kube.ErrorToString()).To(BeEmpty())
		} else {
			Expect(kube.ErrorToString()).To(ContainSubstring("Limiting replica count to 1, use `--replicas` to enable more than one replica"))
		}

		pod := getPodNameInDeployment(deployment)
		inspect := podmanTest.PodmanExitCleanly("inspect", getCtrNameInPod(&pod), "--format", `
CpuPeriod: {{ .HostConfig.CpuPeriod }}
CpuQuota: {{ .HostConfig.CpuQuota }}
Memory: {{ .HostConfig.Memory }}
MemoryReservation: {{ .HostConfig.MemoryReservation }}`)
		Expect(inspect.OutputToString()).To(ContainSubstring(fmt.Sprintf("%s: %d", "CpuQuota", expectedCPUQuota)))
		Expect(inspect.OutputToString()).To(ContainSubstring("MemoryReservation: " + expectedMemoryRequest))
		Expect(inspect.OutputToString()).To(ContainSubstring("Memory: " + expectedMemoryLimit))
	})

	It("allows setting resource limits with --cpus 1", func() {
		SkipIfContainerized("Resource limits require a running systemd")
		SkipIfRootless("CPU limits require root")
		podmanTest.CgroupManager = "systemd"

		expectedCPULimit := "1"

		deployment := getDeployment(
			withPod(getPod(withCtr(getCtr(
				withCPULimit(expectedCPULimit),
			)))))
		err := generateKubeYaml("deployment", deployment, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		pod := getPodNameInDeployment(deployment)
		inspect := podmanTest.PodmanExitCleanly("inspect", getCtrNameInPod(&pod), "--format", `{{ .HostConfig.CpuPeriod }}:{{ .HostConfig.CpuQuota }}`)

		parts := strings.Split(strings.Trim(inspect.OutputToString(), "\n"), ":")
		Expect(parts).To(HaveLen(2))

		Expect(parts[0]).To(Equal(parts[1]))
	})

	It("reports invalid image name", func() {
		invalidImageName := "./myimage"

		pod := getPod(
			withCtr(
				getCtr(
					withImage(invalidImageName),
				),
			),
		)
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitWithError(125, "invalid reference format"))
	})

	It("applies log driver to containers", func() {
		SkipIfInContainer("journald inside a container doesn't work")
		pod := getPod()
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", "--log-opt=max-size=10k", "--log-driver", "journald", kubeYaml)

		cid := getCtrNameInPod(pod)
		inspect := podmanTest.PodmanExitCleanly("inspect", cid, "--format", "'{{ .HostConfig.LogConfig.Type }}'")
		Expect(inspect.OutputToString()).To(ContainSubstring("journald"))
		inspect = podmanTest.PodmanExitCleanly("container", "inspect", "--format", "{{.HostConfig.LogConfig.Size}}", cid)
		Expect(inspect.OutputToString()).To(Equal("10kB"))
	})

	It("test only creating the containers", func() {
		pod := getPod()
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", "--start=false", kubeYaml)
		inspect := podmanTest.PodmanExitCleanly("inspect", getCtrNameInPod(pod), "--format", "{{ .State.Running }}")
		Expect(inspect.OutputToString()).To(Equal("false"))
	})

	It("test with HostNetwork", func() {
		pod := getPod(withHostNetwork(), withCtr(getCtr(withCmd([]string{"readlink", "/proc/self/ns/net"}), withArg(nil))))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		inspect := podmanTest.PodmanExitCleanly("inspect", pod.Name, "--format", "{{ .InfraConfig.HostNetwork }}")
		Expect(inspect.OutputToString()).To(Equal("true"))

		ns := SystemExec("readlink", []string{"/proc/self/ns/net"})
		ns.WaitWithDefaultTimeout()
		Expect(ns).Should(ExitCleanly())
		netns := ns.OutputToString()
		Expect(netns).ToNot(BeEmpty())

		logs := podmanTest.PodmanExitCleanly("logs", getCtrNameInPod(pod))
		Expect(logs.OutputToString()).To(Equal(netns))
	})

	It("test with kube default network", func() {
		pod := getPod()
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		inspect := podmanTest.PodmanExitCleanly("inspect", pod.Name, "--format", "{{ .InfraConfig.Networks }}")
		Expect(inspect.OutputToString()).To(Equal("[podman-default-kube-network]"))
	})

	It("persistentVolumeClaim", func() {
		volName := "myvol"
		volDevice := define.TypeTmpfs
		volType := define.TypeTmpfs
		volOpts := "nodev,noexec"

		pvc := getPVC(withPVCName(volName),
			withPVCAnnotations(util.VolumeDeviceAnnotation, volDevice),
			withPVCAnnotations(util.VolumeTypeAnnotation, volType),
			withPVCAnnotations(util.VolumeMountOptsAnnotation, volOpts))
		err = generateKubeYaml("persistentVolumeClaim", pvc, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		inspect := podmanTest.PodmanExitCleanly("inspect", volName, "--format", `
Name: {{ .Name }}
Device: {{ .Options.device }}
Type: {{ .Options.type }}
o: {{ .Options.o }}`)
		Expect(inspect.OutputToString()).To(ContainSubstring("Name: " + volName))
		Expect(inspect.OutputToString()).To(ContainSubstring("Device: " + volDevice))
		Expect(inspect.OutputToString()).To(ContainSubstring("Type: " + volType))
		Expect(inspect.OutputToString()).To(ContainSubstring("o: " + volOpts))
	})

	It("persistentVolumeClaim with source", func() {
		fileName := "data"
		expectedFileContent := "Test"
		tarFilePath := filepath.Join(podmanTest.TempDir, "podmanVolumeSource.tgz")
		err := createSourceTarFile(fileName, expectedFileContent, tarFilePath)
		Expect(err).ToNot(HaveOccurred())

		volName := "myVolWithStorage"
		pvc := getPVC(withPVCName(volName),
			withPVCAnnotations(util.VolumeImportSourceAnnotation, tarFilePath),
		)
		err = generateKubeYaml("persistentVolumeClaim", pvc, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		if IsRemote() {
			Expect(kube).Should(ExitWithError(125, "importing volumes is not supported for remote requests"))
			return
		}
		Expect(kube).Should(ExitCleanly())

		inspect := podmanTest.PodmanExitCleanly("inspect", volName, "--format", `
{
	"Name": "{{ .Name }}",
	"Mountpoint": "{{ .Mountpoint }}"
}`)
		mp := make(map[string]string)
		err = json.Unmarshal([]byte(inspect.OutputToString()), &mp)
		Expect(err).ToNot(HaveOccurred())
		Expect(mp["Name"]).To(Equal(volName))
		files, err := os.ReadDir(mp["Mountpoint"])
		Expect(err).ToNot(HaveOccurred())
		Expect(files).To(HaveLen(1))
		Expect(files[0].Name()).To(Equal(fileName))
	})

	It("persistentVolumeClaim - image based", func() {
		SkipIfNotAMD64() // https://github.com/containers/podman/issues/28272
		volName := "myVolWithStorage"
		imageName := NGINX_IMAGE
		pvc := getPVC(withPVCName(volName),
			withPVCAnnotations(util.VolumeDriverAnnotation, "image"),
			withPVCAnnotations(util.VolumeImageAnnotation, imageName),
		)
		err = generateKubeYaml("persistentVolumeClaim", pvc, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		inspect := podmanTest.PodmanExitCleanly("inspect", volName, "--format", `
{
	"Name": "{{ .Name }}",
	"Driver": "{{ .Driver }}",
	"Image": "{{ .Options.image }}"
}`)
		mp := make(map[string]string)
		err = json.Unmarshal([]byte(inspect.OutputToString()), &mp)
		Expect(err).ToNot(HaveOccurred())
		Expect(mp["Name"]).To(Equal(volName))
		Expect(mp["Driver"]).To(Equal("image"))
		Expect(mp["Image"]).To(Equal(imageName))
	})

	// Multi doc related tests
	It("multi doc yaml with persistentVolumeClaim, service and deployment", func() {
		yamlDocs := []string{}

		serviceTemplate := `apiVersion: v1
kind: Service
metadata:
  name: %s
spec:
  ports:
  - port: 80
    protocol: TCP
    targetPort: 9376
  selector:
    app: %s
`
		// generate persistentVolumeClaim
		volName := "multiFoo"
		pvc := getPVC(withPVCName(volName))

		// generate deployment
		deploymentName := "multiFoo"
		podName := "multiFoo"
		ctrName := "ctr-01"
		ctr := getCtr(withVolumeMount("/test", "", false))
		ctr.Name = ctrName
		pod := getPod(withPodName(podName), withVolume(getPersistentVolumeClaimVolume(volName)), withCtr(ctr))
		deployment := getDeployment(withPod(pod))
		deployment.Name = deploymentName

		// add pvc
		k, err := getKubeYaml("persistentVolumeClaim", pvc)
		Expect(err).ToNot(HaveOccurred())
		yamlDocs = append(yamlDocs, k)

		// add service
		yamlDocs = append(yamlDocs, fmt.Sprintf(serviceTemplate, deploymentName, deploymentName))

		// add deployment
		k, err = getKubeYaml("deployment", deployment)
		Expect(err).ToNot(HaveOccurred())
		yamlDocs = append(yamlDocs, k)

		// generate multi doc yaml
		err = generateMultiDocKubeYaml(yamlDocs, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		inspectVolume := podmanTest.PodmanExitCleanly("inspect", volName, "--format", "'{{ .Name }}'")
		Expect(inspectVolume.OutputToString()).To(ContainSubstring(volName))

		inspectPod := podmanTest.PodmanExitCleanly("inspect", podName+"-pod", "--format", "'{{ .State }}'")
		Expect(inspectPod.OutputToString()).To(ContainSubstring(`Running`))

		inspectMounts := podmanTest.PodmanExitCleanly("inspect", podName+"-pod-"+ctrName, "--format", "{{ (index .Mounts 0).Type }}:{{ (index .Mounts 0).Name }}")

		correct := fmt.Sprintf("volume:%s", volName)
		Expect(inspectMounts.OutputToString()).To(Equal(correct))
	})

	It("multi doc yaml with multiple services, pods and deployments", func() {
		yamlDocs := []string{}
		podNames := []string{}

		serviceTemplate := `apiVersion: v1
kind: Service
metadata:
  name: %s
spec:
  ports:
  - port: 80
    protocol: TCP
    targetPort: 9376
  selector:
    app: %s
`
		// generate services, pods and deployments
		for i := range 2 {
			podName := fmt.Sprintf("testPod%d", i)
			deploymentName := fmt.Sprintf("testDeploy%d", i)
			deploymentPodName := fmt.Sprintf("%s-pod", deploymentName)

			podNames = append(podNames, podName)
			podNames = append(podNames, deploymentPodName)

			pod := getPod(withPodName(podName))
			podDeployment := getPod(withPodName(deploymentName))
			deployment := getDeployment(withPod(podDeployment))
			deployment.Name = deploymentName

			// add services
			yamlDocs = append([]string{
				fmt.Sprintf(serviceTemplate, podName, podName),
				fmt.Sprintf(serviceTemplate, deploymentPodName, deploymentPodName),
			}, yamlDocs...)

			// add pods
			k, err := getKubeYaml("pod", pod)
			Expect(err).ToNot(HaveOccurred())
			yamlDocs = append(yamlDocs, k)

			// add deployments
			k, err = getKubeYaml("deployment", deployment)
			Expect(err).ToNot(HaveOccurred())
			yamlDocs = append(yamlDocs, k)
		}

		// generate multi doc yaml
		err = generateMultiDocKubeYaml(yamlDocs, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		for _, n := range podNames {
			inspect := podmanTest.PodmanExitCleanly("inspect", n, "--format", "'{{ .State }}'")
			Expect(inspect.OutputToString()).To(ContainSubstring(`Running`))
		}
	})

	It("invalid multi doc yaml", func() {
		yamlDocs := []string{}

		serviceTemplate := `apiVersion: v1
kind: Service
metadata:
  name: %s
spec:
  ports:
  - port: 80
    protocol: TCP
    targetPort: 9376
  selector:
	app: %s
---
invalid kube kind
`
		// add invalid multi doc yaml
		yamlDocs = append(yamlDocs, fmt.Sprintf(serviceTemplate, "foo", "foo"))

		// add pod
		pod := getPod()
		k, err := getKubeYaml("pod", pod)
		Expect(err).ToNot(HaveOccurred())
		yamlDocs = append(yamlDocs, k)

		// generate multi doc yaml
		err = generateMultiDocKubeYaml(yamlDocs, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError(125, "multi doc yaml could not be split: yaml: line 12: found character that cannot start any token"))
	})

	It("with auto update annotations for all containers", func() {
		ctr01Name := "ctr01"
		ctr02Name := "infra"
		podName := "foo"
		autoUpdateRegistry := "io.containers.autoupdate"
		autoUpdateRegistryValue := "registry"
		autoUpdateAuthfile := "io.containers.autoupdate.authfile"
		autoUpdateAuthfileValue := "/some/authfile.json"

		ctr01 := getCtr(withName(ctr01Name))
		ctr02 := getCtr(withName(ctr02Name))

		pod := getPod(
			withPodName(podName),
			withCtr(ctr01),
			withCtr(ctr02),
			withAnnotation(autoUpdateRegistry, autoUpdateRegistryValue),
			withAnnotation(autoUpdateAuthfile, autoUpdateAuthfileValue))

		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		for _, ctr := range []string{podName + "-" + ctr01Name, podName + "-" + ctr02Name} {
			inspect := podmanTest.PodmanExitCleanly("inspect", ctr, "--format", "'{{.Config.Labels}}'")
			Expect(inspect.OutputToString()).To(ContainSubstring(autoUpdateRegistry + ":" + autoUpdateRegistryValue))
			Expect(inspect.OutputToString()).To(ContainSubstring(autoUpdateAuthfile + ":" + autoUpdateAuthfileValue))
		}
	})

	It("with auto update annotations for first container only", func() {
		ctr01Name := "ctr01"
		ctr02Name := "ctr02"
		autoUpdateRegistry := "io.containers.autoupdate"
		autoUpdateRegistryValue := "local"

		ctr01 := getCtr(withName(ctr01Name))
		ctr02 := getCtr(withName(ctr02Name))

		pod := getPod(
			withCtr(ctr01),
			withCtr(ctr02),
		)

		deployment := getDeployment(
			withPod(pod),
			withDeploymentAnnotation(autoUpdateRegistry+"/"+ctr01Name, autoUpdateRegistryValue),
		)

		err = generateKubeYaml("deployment", deployment, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)

		podName := getPodNameInDeployment(deployment).Name

		inspect := podmanTest.PodmanExitCleanly("inspect", podName+"-"+ctr01Name, "--format", "'{{.Config.Labels}}'")
		Expect(inspect.OutputToString()).To(ContainSubstring(autoUpdateRegistry + ":" + autoUpdateRegistryValue))

		inspect = podmanTest.PodmanExitCleanly("inspect", podName+"-"+ctr02Name, "--format", "'{{.Config.Labels}}'")
		Expect(inspect.OutputToString()).NotTo(ContainSubstring(autoUpdateRegistry + ":" + autoUpdateRegistryValue))
	})

	It("teardown", func() {
		pod := getPod()
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		ls := podmanTest.PodmanExitCleanly("pod", "ps", "--format", "'{{.ID}}'")
		Expect(ls.OutputToStringArray()).To(HaveLen(1))

		podmanTest.PodmanExitCleanly("kube", "play", "--down", kubeYaml)
		// Removing a 2nd time to make sure no "no such error" is returned (see #19711)
		podmanTest.PodmanExitCleanly("kube", "play", "--down", kubeYaml)

		checkls := podmanTest.PodmanExitCleanly("pod", "ps", "--format", "'{{.ID}}'")
		Expect(checkls.OutputToStringArray()).To(BeEmpty())
	})

	It("teardown with secret", func() {
		err := writeYaml(secretYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		ls := podmanTest.PodmanExitCleanly("secret", "ls", "--format", "{{.ID}}")
		Expect(ls.OutputToStringArray()).To(HaveLen(1))

		teardown := podmanTest.PodmanExitCleanly("kube", "down", kubeYaml)
		Expect(teardown.OutputToString()).Should(ContainSubstring(ls.OutputToString()))

		// Removing a 2nd time to make sure no "no such error" is returned (see #19711)
		podmanTest.PodmanExitCleanly("kube", "down", kubeYaml)
		checkls := podmanTest.PodmanExitCleanly("secret", "ls", "--format", "'{{.ID}}'")
		Expect(checkls.OutputToStringArray()).To(BeEmpty())
	})

	It("teardown pod does not exist", func() {
		err := writeYaml(simplePodYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", "--down", kubeYaml)
	})

	It("teardown volume --force", func() {
		volName := RandomString(12)
		volDevice := define.TypeTmpfs
		volType := define.TypeTmpfs
		volOpts := "nodev,noexec"

		pvc := getPVC(withPVCName(volName),
			withPVCAnnotations(util.VolumeDeviceAnnotation, volDevice),
			withPVCAnnotations(util.VolumeTypeAnnotation, volType),
			withPVCAnnotations(util.VolumeMountOptsAnnotation, volOpts))
		err = generateKubeYaml("persistentVolumeClaim", pvc, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		podmanTest.PodmanExitCleanly("volume", "exists", volName)
		podmanTest.PodmanExitCleanly("kube", "play", "--down", kubeYaml)
		// volume should not be deleted on teardown without --force
		podmanTest.PodmanExitCleanly("volume", "exists", volName)
		// volume gets deleted with --force
		podmanTest.PodmanExitCleanly("kube", "play", "--down", "--force", kubeYaml)
		// Removing a 2nd should succeed as well even if no volume matches
		podmanTest.PodmanExitCleanly("kube", "play", "--down", "--force", kubeYaml)

		// volume should not be deleted on teardown
		exists := podmanTest.Podman([]string{"volume", "exists", volName})
		exists.WaitWithDefaultTimeout()
		Expect(exists).To(ExitWithError(1, ""))
	})

	It("after teardown with volume reuse", func() {
		volName := RandomString(12)
		volDevice := define.TypeTmpfs
		volType := define.TypeTmpfs
		volOpts := "nodev,noexec"

		pvc := getPVC(withPVCName(volName),
			withPVCAnnotations(util.VolumeDeviceAnnotation, volDevice),
			withPVCAnnotations(util.VolumeTypeAnnotation, volType),
			withPVCAnnotations(util.VolumeMountOptsAnnotation, volOpts))
		err = generateKubeYaml("persistentVolumeClaim", pvc, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		podmanTest.PodmanExitCleanly("volume", "exists", volName)
		podmanTest.PodmanExitCleanly("kube", "play", "--down", kubeYaml)
		// volume should not be deleted on teardown
		podmanTest.PodmanExitCleanly("volume", "exists", volName)
		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
	})

	It("use network mode from config", func() {
		confPath, err := filepath.Abs("config/containers-netns2.conf")
		Expect(err).ToNot(HaveOccurred())
		os.Setenv("CONTAINERS_CONF", confPath)
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}

		pod := getPod()
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		inspect := podmanTest.PodmanExitCleanly("pod", "inspect", pod.Name, "--format", "{{.InfraContainerID}}")
		infraID := inspect.OutputToString()

		inspect = podmanTest.PodmanExitCleanly("inspect", "--format", "{{.HostConfig.NetworkMode}}", infraID)
		Expect(inspect.OutputToString()).To(Equal("bridge"))
	})

	It("replace", func() {
		pod := getPod()
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		ls := podmanTest.PodmanExitCleanly("pod", "ps", "--format", "'{{.ID}}'")
		Expect(ls.OutputToStringArray()).To(HaveLen(1))

		containerLen := podmanTest.PodmanExitCleanly("pod", "inspect", pod.Name, "--format", "{{len .Containers}}")
		Expect(containerLen.OutputToString()).To(Equal("2"))
		ctr01Name := "ctr01"
		ctr02Name := "ctr02"

		ctr01 := getCtr(withName(ctr01Name))
		ctr02 := getCtr(withName(ctr02Name))

		newPod := getPod(
			withCtr(ctr01),
			withCtr(ctr02),
		)
		err = generateKubeYaml("pod", newPod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", "--replace", kubeYaml)
		newContainerLen := podmanTest.PodmanExitCleanly("pod", "inspect", newPod.Name, "--format", "{{len .Containers}}")
		Expect(newContainerLen.OutputToString()).NotTo(Equal(containerLen.OutputToString()))
	})

	It("replace non-existing pod", func() {
		pod := getPod()
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", "--replace", kubeYaml)
		ls := podmanTest.PodmanExitCleanly("pod", "ps", "--format", "'{{.ID}}'")
		Expect(ls.OutputToStringArray()).To(HaveLen(1))
	})

	It("RunAsUser", func() {
		ctr1Name := "ctr1"
		ctr2Name := "ctr2"
		ctr1 := getCtr(withName(ctr1Name), withSecurityContext(true), withRunAsUser("101"), withRunAsGroup("102"))
		ctr2 := getCtr(withName(ctr2Name), withSecurityContext(true))

		pod := getPod(
			withCtr(ctr1),
			withCtr(ctr2),
			withPodSecurityContext(true),
			withPodRunAsUser("103"),
			withPodRunAsGroup("104"),
		)

		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		// we expect the user:group as configured for the container
		inspect := podmanTest.PodmanExitCleanly("container", "inspect", "--format", "'{{.Config.User}}'", makeCtrNameInPod(pod, ctr1Name))
		Expect(inspect.OutputToString()).To(Equal("'101:102'"))

		// we expect the user:group as configured for the pod
		inspect = podmanTest.PodmanExitCleanly("container", "inspect", "--format", "'{{.Config.User}}'", makeCtrNameInPod(pod, ctr2Name))
		Expect(inspect.OutputToString()).To(Equal("'103:104'"))
	})

	Describe("verify environment variables", func() {
		var maxLength int
		BeforeEach(func() {
			maxLength = format.MaxLength
			format.MaxLength = 0
		})
		AfterEach(func() {
			format.MaxLength = maxLength
		})

		It("values containing equal sign", func() {
			javaToolOptions := `-XX:+IgnoreUnrecognizedVMOptions -XX:+IdleTuningGcOnIdle -Xshareclasses:name=openj9_system_scc,cacheDir=/opt/java/.scc,readonly,nonFatal`
			openj9JavaOptions := `-XX:+IgnoreUnrecognizedVMOptions -XX:+IdleTuningGcOnIdle -Xshareclasses:name=openj9_system_scc,cacheDir=/opt/java/.scc,readonly,nonFatal -Dosgi.checkConfiguration=false`

			containerfile := fmt.Sprintf(`FROM %s
ENV JAVA_TOOL_OPTIONS=%q
ENV OPENJ9_JAVA_OPTIONS=%q
`,
				CITEST_IMAGE, javaToolOptions, openj9JavaOptions)

			image := "podman-kube-test:env"
			podmanTest.BuildImage(containerfile, image, "false")
			ctnr := getCtr(withImage(image))
			pod := getPod(withCtr(ctnr))
			Expect(generateKubeYaml("pod", pod, kubeYaml)).Should(Succeed())

			podmanTest.PodmanExitCleanly("kube", "play", "--start", kubeYaml)
			inspect := podmanTest.PodmanExitCleanly("container", "inspect", "--format=json", getCtrNameInPod(pod))
			contents := string(inspect.Out.Contents())
			Expect(contents).To(ContainSubstring(javaToolOptions))
			Expect(contents).To(ContainSubstring(openj9JavaOptions))
		})
	})

	Context("with configmap in multi-doc yaml", func() {
		It("uses env value", func() {
			cm := getConfigMap(withConfigMapName("foo"), withConfigMapData("FOO", "foo"))

			cmYaml, err := getKubeYaml("configmap", cm)
			Expect(err).ToNot(HaveOccurred())

			pod := getPod(withCtr(getCtr(withEnv("FOO", "", "configmap", "foo", "FOO", false))))

			podYaml, err := getKubeYaml("pod", pod)
			Expect(err).ToNot(HaveOccurred())

			yamls := []string{cmYaml, podYaml}
			err = generateMultiDocKubeYaml(yamls, kubeYaml)
			Expect(err).ToNot(HaveOccurred())

			podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
			inspect := podmanTest.PodmanExitCleanly("inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Env }}'")
			Expect(inspect.OutputToString()).To(ContainSubstring(`FOO=foo`))
		})

		It("fails for required env value with missing key", func() {
			cm := getConfigMap(withConfigMapName("foo"), withConfigMapData("FOO", "foo"))

			cmYaml, err := getKubeYaml("configmap", cm)
			Expect(err).ToNot(HaveOccurred())

			pod := getPod(withCtr(getCtr(withEnv("FOO", "", "configmap", "foo", "MISSING_KEY", false))))

			podYaml, err := getKubeYaml("pod", pod)
			Expect(err).ToNot(HaveOccurred())

			yamls := []string{cmYaml, podYaml}
			err = generateMultiDocKubeYaml(yamls, kubeYaml)
			Expect(err).ToNot(HaveOccurred())

			kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
			kube.WaitWithDefaultTimeout()
			Expect(kube).To(ExitWithError(125, "cannot set env FOO: key MISSING_KEY not found in configmap foo"))
		})

		It("succeeds for optional env value with missing key", func() {
			cm := getConfigMap(withConfigMapName("foo"), withConfigMapData("FOO", "foo"))

			cmYaml, err := getKubeYaml("configmap", cm)
			Expect(err).ToNot(HaveOccurred())

			pod := getPod(withCtr(getCtr(withEnv("FOO", "", "configmap", "foo", "MISSING_KEY", true))))

			podYaml, err := getKubeYaml("pod", pod)
			Expect(err).ToNot(HaveOccurred())

			yamls := []string{cmYaml, podYaml}
			err = generateMultiDocKubeYaml(yamls, kubeYaml)
			Expect(err).ToNot(HaveOccurred())

			podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
			inspect := podmanTest.PodmanExitCleanly("inspect", getCtrNameInPod(pod), "--format", "'{{ range .Config.Env }}[{{ . }}]{{end}}'")
			Expect(inspect.OutputToString()).To(Not(ContainSubstring(`[FOO=]`)))
		})

		It("uses all key-value pairs as envs", func() {
			cm := getConfigMap(withConfigMapName("foo"), withConfigMapData("FOO1", "foo1"), withConfigMapData("FOO2", "foo2"))
			cmYaml, err := getKubeYaml("configmap", cm)
			Expect(err).ToNot(HaveOccurred())

			pod := getPod(withCtr(getCtr(withEnvFrom("foo", "configmap", false))))

			podYaml, err := getKubeYaml("pod", pod)
			Expect(err).ToNot(HaveOccurred())

			yamls := []string{cmYaml, podYaml}
			err = generateMultiDocKubeYaml(yamls, kubeYaml)
			Expect(err).ToNot(HaveOccurred())

			podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
			inspect := podmanTest.PodmanExitCleanly("inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Env }}'")
			Expect(inspect.OutputToString()).To(ContainSubstring(`FOO1=foo1`))
			Expect(inspect.OutputToString()).To(ContainSubstring(`FOO2=foo2`))
		})

		It("deployment uses variable from config map", func() {
			cm := getConfigMap(withConfigMapName("foo"), withConfigMapData("FOO", "foo"))

			cmYaml, err := getKubeYaml("configmap", cm)
			Expect(err).ToNot(HaveOccurred())

			pod := getPod(withCtr(getCtr(withEnv("FOO", "", "configmap", "foo", "FOO", false))))

			deployment := getDeployment(withPod(pod))
			deploymentYaml, err := getKubeYaml("deployment", deployment)
			Expect(err).ToNot(HaveOccurred(), "getKubeYaml(deployment)")
			yamls := []string{cmYaml, deploymentYaml}
			err = generateMultiDocKubeYaml(yamls, kubeYaml)
			Expect(err).ToNot(HaveOccurred())

			podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
			inspect := podmanTest.PodmanExitCleanly("inspect", fmt.Sprintf("%s-%s-%s", deployment.Name, "pod", defaultCtrName), "--format", "'{{ .Config }}'")
			Expect(inspect.OutputToString()).To(ContainSubstring(`FOO=foo`))
		})

		It("uses env value from configmap for HTTP API client", func() {
			SkipIfRemote("cannot run in a remote setup")
			address := url.URL{
				Scheme: "tcp",
				Host:   net.JoinHostPort("localhost", "8080"),
			}

			session := podmanTest.Podman([]string{
				"system", "service", "--log-level=debug", "--time=0", address.String(),
			})
			defer session.Kill()

			WaitForService(address)

			cm := getConfigMap(withConfigMapName("foo"), withConfigMapData("FOO", "foo"))
			cmYaml, err := getKubeYaml("configmap", cm)
			Expect(err).ToNot(HaveOccurred())

			pod := getPod(withCtr(getCtr(withEnv("FOO", "", "configmap", "foo", "FOO", false))))
			podYaml, err := getKubeYaml("pod", pod)
			Expect(err).ToNot(HaveOccurred())
			yamls := []string{cmYaml, podYaml}
			err = generateMultiDocKubeYaml(yamls, kubeYaml)
			Expect(err).ToNot(HaveOccurred())

			podmanConnection, err := bindings.NewConnection(context.Background(), address.String())
			Expect(err).ToNot(HaveOccurred())

			_, err = play.Kube(podmanConnection, kubeYaml, nil)
			Expect(err).ToNot(HaveOccurred())

			inspect := podmanTest.PodmanExitCleanly("inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Env }}'")
			Expect(inspect.OutputToString()).To(ContainSubstring(`FOO=foo`))
		})
	})

	Context("with configmap in multi-doc yaml and files", func() {
		It("uses env values from both sources", func() {
			fsCmYamlPathname := filepath.Join(podmanTest.TempDir, "foo-cm.yaml")
			fsCm := getConfigMap(withConfigMapName("fooFs"), withConfigMapData("FOO_FS", "fooFS"))
			err := generateKubeYaml("configmap", fsCm, fsCmYamlPathname)
			Expect(err).ToNot(HaveOccurred())

			cm := getConfigMap(withConfigMapName("foo"), withConfigMapData("FOO", "foo"))

			cmYaml, err := getKubeYaml("configmap", cm)
			Expect(err).ToNot(HaveOccurred())

			pod := getPod(withCtr(getCtr(
				withEnv("FOO_FS", "", "configmap", "fooFs", "FOO_FS", false),
				withEnv("FOO", "", "configmap", "foo", "FOO", false),
			)))

			podYaml, err := getKubeYaml("pod", pod)
			Expect(err).ToNot(HaveOccurred())

			yamls := []string{cmYaml, podYaml}
			err = generateMultiDocKubeYaml(yamls, kubeYaml)
			Expect(err).ToNot(HaveOccurred())

			podmanTest.PodmanExitCleanly("kube", "play", kubeYaml, "--configmap", fsCmYamlPathname)
			inspect := podmanTest.PodmanExitCleanly("inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Env }}'")
			Expect(inspect.OutputToString()).To(And(
				ContainSubstring(`FOO=foo`),
				ContainSubstring(`FOO_FS=fooFS`),
			))
		})

		It("uses all env values from both sources", func() {
			fsCmYamlPathname := filepath.Join(podmanTest.TempDir, "foo-cm.yaml")
			fsCm := getConfigMap(withConfigMapName("fooFs"),
				withConfigMapData("FOO_FS_1", "fooFS1"),
				withConfigMapData("FOO_FS_2", "fooFS2"))
			err := generateKubeYaml("configmap", fsCm, fsCmYamlPathname)
			Expect(err).ToNot(HaveOccurred())

			cm := getConfigMap(withConfigMapName("foo"),
				withConfigMapData("FOO_1", "foo1"),
				withConfigMapData("FOO_2", "foo2"),
			)

			cmYaml, err := getKubeYaml("configmap", cm)
			Expect(err).ToNot(HaveOccurred())

			pod := getPod(withCtr(getCtr(
				withEnvFrom("foo", "configmap", false),
				withEnvFrom("fooFs", "configmap", false),
			)))

			podYaml, err := getKubeYaml("pod", pod)
			Expect(err).ToNot(HaveOccurred())

			yamls := []string{cmYaml, podYaml}
			err = generateMultiDocKubeYaml(yamls, kubeYaml)
			Expect(err).ToNot(HaveOccurred())

			podmanTest.PodmanExitCleanly("kube", "play", kubeYaml, "--configmap", fsCmYamlPathname)
			inspect := podmanTest.PodmanExitCleanly("inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Env }}'")
			Expect(inspect.OutputToString()).To(And(
				ContainSubstring(`FOO_1=foo1`),
				ContainSubstring(`FOO_2=foo2`),
				ContainSubstring(`FOO_FS_1=fooFS1`),
				ContainSubstring(`FOO_FS_2=fooFS2`),
			))
		})

		It("reports error when the same configmap name is present in both sources", func() {
			// We will never hit this error in the remote case as the configmap content is appended to the main yaml content
			SkipIfRemote("--configmaps is appended to the main yaml for the remote case")

			fsCmYamlPathname := filepath.Join(podmanTest.TempDir, "foo-cm.yaml")
			fsCm := getConfigMap(withConfigMapName("foo"), withConfigMapData("FOO", "fooFS"))
			err := generateKubeYaml("configmap", fsCm, fsCmYamlPathname)
			Expect(err).ToNot(HaveOccurred())

			cm := getConfigMap(withConfigMapName("foo"), withConfigMapData("FOO", "foo"))

			cmYaml, err := getKubeYaml("configmap", cm)
			Expect(err).ToNot(HaveOccurred())

			pod := getPod(withCtr(getCtr(
				withEnv("FOO", "", "configmap", "foo", "FOO", false),
			)))

			podYaml, err := getKubeYaml("pod", pod)
			Expect(err).ToNot(HaveOccurred())

			yamls := []string{cmYaml, podYaml}
			err = generateMultiDocKubeYaml(yamls, kubeYaml)
			Expect(err).ToNot(HaveOccurred())

			kube := podmanTest.Podman([]string{"kube", "play", kubeYaml, "--configmap", fsCmYamlPathname})
			kube.WaitWithDefaultTimeout()
			Expect(kube).Should(ExitWithError(125, "ambiguous configuration: the same config map foo is present in YAML and in --configmaps"))
		})
	})

	It("--log-opt = tag test", func() {
		SkipIfContainerized("journald does not work inside the container")
		pod := getPod()
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml, "--log-driver", "journald", "--log-opt", "tag={{.ImageName}},withcomma")
		podmanTest.PodmanExitCleanly("start", getCtrNameInPod(pod))
		inspect := podmanTest.PodmanExitCleanly("inspect", getCtrNameInPod(pod))
		Expect((inspect.InspectContainerToJSON()[0]).HostConfig.LogConfig.Tag).To(Equal("{{.ImageName}},withcomma"))
	})

	It("using a user namespace", func() {
		u, err := user.Current()
		Expect(err).ToNot(HaveOccurred())
		name := u.Username
		if name == "root" {
			name = "containers"
		}
		content, err := os.ReadFile("/etc/subuid")
		if err != nil {
			Skip("cannot read /etc/subuid")
		}
		if !strings.Contains(string(content), name) {
			Skip("cannot find mappings for the current user")
		}

		initialUsernsConfig, err := os.ReadFile("/proc/self/uid_map")
		Expect(err).ToNot(HaveOccurred())
		if isRootless() {
			// Use podmanTest.PodmanBinary because podman-remote unshare cannot be used
			cmd := exec.Command(podmanTest.PodmanBinary, "unshare", "cat", "/proc/self/uid_map")
			session, err := Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(session, DefaultWaitTimeout).Should(Exit(0))
			initialUsernsConfig = session.Out.Contents()
		}

		pod := getPod()
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		usernsInCtr := podmanTest.PodmanExitCleanly("exec", getCtrNameInPod(pod), "cat", "/proc/self/uid_map")
		// the conversion to string is needed for better error messages
		Expect(string(usernsInCtr.Out.Contents())).To(Equal(string(initialUsernsConfig)))

		// -q necessary for ExitCleanly() because --replace pulls image
		podmanTest.PodmanExitCleanly("kube", "play", "-q", "--replace", "--userns=auto", kubeYaml)
		usernsInCtr = podmanTest.PodmanExitCleanly("exec", getCtrNameInPod(pod), "cat", "/proc/self/uid_map")
		Expect(string(usernsInCtr.Out.Contents())).To(Not(Equal(string(initialUsernsConfig))))

		kube := podmanTest.PodmanNoCache([]string{"kube", "play", "-q", "--replace", "--userns=keep-id", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		usernsInCtr = podmanTest.PodmanExitCleanly("exec", getCtrNameInPod(pod), "id", "-u")
		uid := strconv.Itoa(os.Geteuid())
		Expect(string(usernsInCtr.Out.Contents())).To(ContainSubstring(uid))

		kube = podmanTest.PodmanNoCache([]string{"kube", "play", "--replace", "--userns=keep-id:uid=10,gid=12", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		usernsInCtr = podmanTest.PodmanExitCleanly("exec", getCtrNameInPod(pod), "sh", "-c", "echo $(id -u):$(id -g)")
		Expect(string(usernsInCtr.Out.Contents())).To(ContainSubstring("10:12"))

		// Now try with hostUsers in the pod spec
		for _, hostUsers := range []bool{true, false} {
			pod = getPod(withHostUsers(hostUsers))
			err = generateKubeYaml("pod", pod, kubeYaml)
			Expect(err).ToNot(HaveOccurred())

			kube = podmanTest.PodmanNoCache([]string{"kube", "play", "--replace", kubeYaml})
			kube.WaitWithDefaultTimeout()
			Expect(kube).Should(ExitCleanly())

			usernsInCtr = podmanTest.PodmanExitCleanly("exec", getCtrNameInPod(pod), "cat", "/proc/self/uid_map")
			if hostUsers {
				Expect(string(usernsInCtr.Out.Contents())).To(Equal(string(initialUsernsConfig)))
			} else {
				Expect(string(usernsInCtr.Out.Contents())).To(Not(Equal(string(initialUsernsConfig))))
			}
		}
	})

	// Check the block devices are exposed inside container
	It("expose block device inside container", func() {
		SkipIfRootless("It needs root access to create devices")

		// randomize the folder name to avoid error when running tests with multiple nodes
		uuid, err := uuid.NewUUID()
		Expect(err).ToNot(HaveOccurred())
		devFolder := fmt.Sprintf("/dev/foodev%x", uuid[:6])
		Expect(os.MkdirAll(devFolder, os.ModePerm)).To(Succeed())
		defer os.RemoveAll(devFolder)

		devicePath := fmt.Sprintf("%s/blockdevice", devFolder)
		mknod := SystemExec("mknod", []string{devicePath, "b", "7", "0"})
		mknod.WaitWithDefaultTimeout()
		Expect(mknod).Should(ExitCleanly())

		blockVolume := getHostPathVolume("BlockDevice", devicePath)

		pod := getPod(withVolume(blockVolume), withCtr(getCtr(withImage(REGISTRY_IMAGE), withCmd(nil), withArg(nil), withVolumeMount(devicePath, "", false))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		// Container should be in running state
		inspect := podmanTest.PodmanExitCleanly("inspect", "--format", "{{.State.Status}}", "testPod-"+defaultCtrName)
		Expect(inspect.OutputToString()).To(ContainSubstring("running"))

		// Container should have a block device /dev/loop1
		inspect = podmanTest.PodmanExitCleanly("inspect", "--format", "{{.HostConfig.Devices}}", "testPod-"+defaultCtrName)
		Expect(inspect.OutputToString()).To(ContainSubstring(devicePath))
	})

	// Check the char devices are exposed inside container
	It("expose character device inside container", func() {
		SkipIfRootless("It needs root access to create devices")

		// randomize the folder name to avoid error when running tests with multiple nodes
		uuid, err := uuid.NewUUID()
		Expect(err).ToNot(HaveOccurred())
		devFolder := fmt.Sprintf("/dev/foodev%x", uuid[:6])
		Expect(os.MkdirAll(devFolder, os.ModePerm)).To(Succeed())
		defer os.RemoveAll(devFolder)

		devicePath := fmt.Sprintf("%s/chardevice", devFolder)
		mknod := SystemExec("mknod", []string{devicePath, "c", "3", "1"})
		mknod.WaitWithDefaultTimeout()
		Expect(mknod).Should(ExitCleanly())

		charVolume := getHostPathVolume("CharDevice", devicePath)

		pod := getPod(withVolume(charVolume), withCtr(getCtr(withImage(REGISTRY_IMAGE), withCmd(nil), withArg(nil), withVolumeMount(devicePath, "", false))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		// Container should be in running state
		inspect := podmanTest.PodmanExitCleanly("inspect", "--format", "{{.State.Status}}", "testPod-"+defaultCtrName)
		Expect(inspect.OutputToString()).To(ContainSubstring("running"))

		// Container should have a block device /dev/loop1
		inspect = podmanTest.PodmanExitCleanly("inspect", "--format", "{{.HostConfig.Devices}}", "testPod-"+defaultCtrName)
		Expect(inspect.OutputToString()).To(ContainSubstring(devicePath))
	})

	It("reports error when the device does not exist", func() {
		SkipIfRootless("It needs root access to create devices")

		devicePath := "/dev/foodevdir/baddevice"

		blockVolume := getHostPathVolume("BlockDevice", devicePath)

		pod := getPod(withVolume(blockVolume), withCtr(getCtr(withImage(REGISTRY_IMAGE), withCmd(nil), withArg(nil), withVolumeMount(devicePath, "", false))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitWithError(125, fmt.Sprintf(`failed to create volume "testVol": checking HostPathBlockDevice: stat %s: no such file or directory`, devicePath)))
	})

	It("reports error when we try to expose char device as block device", func() {
		SkipIfRootless("It needs root access to create devices")

		// randomize the folder name to avoid error when running tests with multiple nodes
		uuid, err := uuid.NewUUID()
		Expect(err).ToNot(HaveOccurred())
		devFolder := fmt.Sprintf("/dev/foodev%x", uuid[:6])
		Expect(os.MkdirAll(devFolder, os.ModePerm)).To(Succeed())
		defer os.RemoveAll(devFolder)

		devicePath := fmt.Sprintf("%s/chardevice", devFolder)
		mknod := SystemExec("mknod", []string{devicePath, "c", "3", "1"})
		mknod.WaitWithDefaultTimeout()
		Expect(mknod).Should(ExitCleanly())

		charVolume := getHostPathVolume("BlockDevice", devicePath)

		pod := getPod(withVolume(charVolume), withCtr(getCtr(withImage(REGISTRY_IMAGE), withCmd(nil), withArg(nil), withVolumeMount(devicePath, "", false))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitWithError(125, fmt.Sprintf(`failed to create volume "testVol": checking HostPathDevice: path %s is not a block device`, devicePath)))
	})

	It("reports error when we try to expose block device as char device", func() {
		SkipIfRootless("It needs root access to create devices")

		// randomize the folder name to avoid error when running tests with multiple nodes
		uuid, err := uuid.NewUUID()
		Expect(err).ToNot(HaveOccurred())
		devFolder := fmt.Sprintf("/dev/foodev%x", uuid[:6])
		Expect(os.MkdirAll(devFolder, os.ModePerm)).To(Succeed())

		devicePath := fmt.Sprintf("%s/blockdevice", devFolder)
		mknod := SystemExec("mknod", []string{devicePath, "b", "7", "0"})
		mknod.WaitWithDefaultTimeout()
		Expect(mknod).Should(ExitCleanly())

		charVolume := getHostPathVolume("CharDevice", devicePath)

		pod := getPod(withVolume(charVolume), withCtr(getCtr(withImage(REGISTRY_IMAGE), withCmd(nil), withArg(nil), withVolumeMount(devicePath, "", false))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitWithError(125, fmt.Sprintf(`failed to create volume "testVol": checking HostPathCharDevice: path %s is not a character device`, devicePath)))
	})

	It("secret as volume support - simple", func() {
		createAndTestSecret(podmanTest, secretYaml, "newsecret", kubeYaml)
		testPodWithSecret(podmanTest, secretPodYaml, kubeYaml, true, true)
		deleteAndTestSecret(podmanTest, "newsecret")
	})

	It("secret as volume support - error on invalid secret format", func() {
		const secretName = "newsecret"

		// Create text file secret
		kubeTxt := strings.Replace(kubeYaml, ".yaml", ".txt", 1)
		err := writeYaml(secretTxt, kubeTxt)
		Expect(err).ToNot(HaveOccurred())

		createSecret := podmanTest.Podman([]string{"secret", "create", secretName, kubeTxt})
		createSecret.WaitWithDefaultTimeout()
		Expect(createSecret).Should(ExitCleanly())

		// Run kube play and expect error
		err = writeYaml(secretPodYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(-1))
		Expect(kube.ErrorToString()).To(ContainSubstring("only secrets created via the kube yaml file are supported"))

		// Delete secret
		deleteAndTestSecret(podmanTest, secretName)
	})

	It("secret as volume support - multiple volumes", func() {
		yamls := []string{secretYaml, secretPodYaml}
		err = generateMultiDocKubeYaml(yamls, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		// do not remove newsecret to test that we auto remove on collision

		yamls = []string{secretYaml, complexSecretYaml}
		err = generateMultiDocKubeYaml(yamls, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		err = writeYaml(secretPodYamlTwo, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		exec := podmanTest.PodmanExitCleanly("exec", "mypod2-myctr", "cat", "/etc/foo/username")

		username, _ := base64.StdEncoding.DecodeString("dXNlcg==")
		Expect(exec.OutputToString()).Should(ContainSubstring(string(username)))

		exec = podmanTest.PodmanExitCleanly("exec", "mypod2-myctr", "cat", "/etc/bar/username")
		username, _ = base64.StdEncoding.DecodeString("Y2RvZXJu")
		Expect(exec.OutputToString()).Should(ContainSubstring(string(username)))

		exec = podmanTest.PodmanExitCleanly("exec", "mypod2-myctr", "cat", "/etc/baz/plain_note")
		Expect(exec.OutputToString()).Should(ContainSubstring("This is a test"))
	})

	It("secret as volume support - optional field", func() {
		createAndTestSecret(podmanTest, secretYaml, "newsecret", kubeYaml)

		testPodWithSecret(podmanTest, optionalExistingSecretPodYaml, kubeYaml, true, true)
		testPodWithSecret(podmanTest, optionalNonExistingSecretPodYaml, kubeYaml, true, false)
		testPodWithSecret(podmanTest, noOptionalExistingSecretPodYaml, kubeYaml, true, true)
		testPodWithSecret(podmanTest, noOptionalNonExistingSecretPodYaml, kubeYaml, false, false)

		deleteAndTestSecret(podmanTest, "newsecret")
	})

	It("secret as volume with no items", func() {
		volumeName := "secretVol"
		secret := getSecret(withSecretName(volumeName), withSecretData("FOO", "testuser"))
		secretYaml, err := getKubeYaml("secret", secret)
		Expect(err).ToNot(HaveOccurred())

		ctr := getCtr(withVolumeMount("/test", "", false), withImage(CITEST_IMAGE))
		pod := getPod(withVolume(getSecretVolume(volumeName, []map[string]string{}, false, nil)), withCtr(ctr))
		podYaml, err := getKubeYaml("pod", pod)
		Expect(err).ToNot(HaveOccurred())
		yamls := []string{secretYaml, podYaml}
		err = generateMultiDocKubeYaml(yamls, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		secretData := podmanTest.PodmanExitCleanly("exec", getCtrNameInPod(pod), "cat", "/test/FOO")
		Expect(secretData.OutputToString()).To(Equal("testuser"))
	})

	It("secret as volume with items", func() {
		volumeName := "secretVol"
		secret := getSecret(withSecretName(volumeName), withSecretData("FOO", "foobar"))
		secretYaml, err := getKubeYaml("secret", secret)
		Expect(err).ToNot(HaveOccurred())
		volumeContents := []map[string]string{{
			"key":  "FOO",
			"path": "BAR",
		}}

		ctr := getCtr(withVolumeMount("/test", "", false), withImage(CITEST_IMAGE))
		pod := getPod(withVolume(getSecretVolume(volumeName, volumeContents, false, nil)), withCtr(ctr))
		podYaml, err := getKubeYaml("pod", pod)
		Expect(err).ToNot(HaveOccurred())
		yamls := []string{secretYaml, podYaml}
		err = generateMultiDocKubeYaml(yamls, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		secretData := podmanTest.PodmanExitCleanly("exec", getCtrNameInPod(pod), "cat", "/test/BAR")
		Expect(secretData.OutputToString()).To(Equal("foobar"))

		secretData = podmanTest.Podman([]string{"exec", getCtrNameInPod(pod), "cat", "/test/FOO"})
		secretData.WaitWithDefaultTimeout()
		Expect(secretData).Should(Not(ExitCleanly()))
	})

	It("secret as volume with defaultMode set", func() {
		volumeName := "secretVol"
		secret := getSecret(withSecretName(volumeName), withSecretData("FOO", "testuser"))
		secretYaml, err := getKubeYaml("secret", secret)
		Expect(err).ToNot(HaveOccurred())

		ctr := getCtr(withVolumeMount("/test", "", false), withImage(CITEST_IMAGE))
		defaultMode := int32(0o777)
		pod := getPod(withVolume(getSecretVolume(volumeName, []map[string]string{}, false, &defaultMode)), withCtr(ctr))
		podYaml, err := getKubeYaml("pod", pod)
		Expect(err).ToNot(HaveOccurred())
		yamls := []string{secretYaml, podYaml}
		err = generateMultiDocKubeYaml(yamls, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		secretData := podmanTest.PodmanExitCleanly("exec", getCtrNameInPod(pod), "cat", "/test/FOO")
		Expect(secretData.OutputToString()).To(Equal("testuser"))

		inspect := podmanTest.PodmanExitCleanly("volume", "inspect", volumeName, "--format", "{{.Mountpoint}}")
		Expect(inspect.OutputToStringArray()).To(HaveLen(1))
		path := inspect.OutputToString()

		permData := SystemExec("stat", []string{"-c", "%a", path + "/FOO"})
		permData.WaitWithDefaultTimeout()
		Expect(permData).Should(ExitCleanly())
		Expect(permData.OutputToString()).To(Equal("777"))
	})

	It("secret as volume with no defaultMode set", func() {
		secretYaml := `
apiVersion: v1
kind: Secret
metadata:
  name: newsecret
type: Opaque
data:
  foo: dXNlcg==
---
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  containers:
  - command:
    - sleep
    - "1000"
    image: alpine
    name: test
    volumeMounts:
    - name: secret-volume
      mountPath: /test
  volumes:
  - name: secret-volume
    secret:
     secretName: newsecret
`

		err := writeYaml(secretYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())
		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		inspect := podmanTest.PodmanExitCleanly("volume", "inspect", "newsecret", "--format", "{{.Mountpoint}}")
		Expect(inspect.OutputToStringArray()).To(HaveLen(1))
		path := inspect.OutputToString()

		permData := SystemExec("stat", []string{"-c", "%a", path + "/foo"})
		permData.WaitWithDefaultTimeout()
		Expect(permData).Should(ExitCleanly())
		Expect(permData.OutputToString()).To(Equal("644"))
	})

	It("with disabled cgroup", func() {
		SkipIfRunc(podmanTest, "Test not supported with runc (issue 17436, wontfix)")
		conffile := filepath.Join(podmanTest.TempDir, "container.conf")
		// Disabled ipcns and cgroupfs in the config file
		// Since shmsize (Inherit from infra container) cannot be set if ipcns is "host", we should remove the default value.
		// Also, cgroupfs config should be loaded into SpecGenerator when playing kube.
		err := os.WriteFile(conffile, []byte(`
[containers]
ipcns="host"
cgroups="disabled"`), 0o644)
		Expect(err).ToNot(HaveOccurred())
		os.Setenv("CONTAINERS_CONF", conffile)
		err = writeYaml(simplePodYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
	})

	It("podman kube --quiet with error", func() {
		SkipIfNotRootless("We need to create an error trying to bind to port 80")
		yaml := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: frontend
spec:
  replicas: 2
  template:
    metadata:
      labels:
        app: guestbook
        tier: frontend
    spec:
      containers:
      - name: php-redis
        image: quay.io/libpod/alpine_nginx:latest
        ports:
        - containerPort: 1234
          hostPort: 80
`

		err = writeYaml(yaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", "--quiet", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError(125, "rootlessport cannot expose privileged port 80,"))
		// The ugly format-error exited once in Podman. The test makes
		// sure it's not coming back.
		Expect(kube.ErrorToString()).To(Not(ContainSubstring("Error: %!s(<nil>)")))
	})

	It("invalid yaml should clean up pod that was created before failure", func() {
		podTemplate := `---
apiVersion: v1
kind: Pod
metadata:
	creationTimestamp: "2022-08-02T04:05:53Z"
	labels:
	app: vol-test-3-pod
	name: vol-test-3
spec:
	containers:
	- command:
	- sleep
	- "1000"
	image: non-existing-image
	name: vol-test-3
`

		err = writeYaml(podTemplate, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		// the image is incorrect so the kube play will fail, but it will clean up the pod that was created for it before the failure happened
		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError(125, "multi doc yaml could not be split: yaml: line 5: found character that cannot start any token"))

		ps := podmanTest.PodmanExitCleanly("pod", "ps", "-q")
		Expect(ps.OutputToStringArray()).To(BeEmpty())
	})

	It("with named volume subpaths", func() {
		SkipIfRemote("volume export does not exist on remote")
		podmanTest.PodmanExitCleanly("volume", "create", "testvol1")
		podmanTest.PodmanExitCleanly("run", "--volume", "testvol1:/data", CITEST_IMAGE, "sh", "-c", "mkdir -p /data/testing/onlythis && touch /data/testing/onlythis/123.txt && echo hi >> /data/testing/onlythis/123.txt")

		tar := filepath.Join(podmanTest.TempDir, "out.tar")
		podmanTest.PodmanExitCleanly("volume", "export", "--output", tar, "testvol1")

		podmanTest.PodmanExitCleanly("volume", "create", "testvol")

		podmanTest.PodmanExitCleanly("volume", "import", "testvol", filepath.Join(podmanTest.TempDir, "out.tar"))

		err = writeYaml(subpathTestNamedVolume, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)

		exec := podmanTest.PodmanExitCleanly("exec", "testpod-testctr", "cat", "/var/123.txt")
		Expect(exec.OutputToString()).Should(Equal("hi"))

		teardown := podmanTest.PodmanExitCleanly("kube", "down", "--force", kubeYaml)
		Expect(teardown.OutputToString()).Should(ContainSubstring("testvol"))

		// kube down --force should remove volumes
		// specified in the manifest but not externally
		// created volumes, testvol1 in this case
		checkVol := podmanTest.PodmanExitCleanly("volume", "ls", "--format", "{{.Name}}")
		Expect(checkVol.OutputToString()).To(Equal("testvol1"))
	})

	It("with graceful shutdown", func() {
		podmanTest.PodmanExitCleanly("volume", "create", "testvol")
		err = writeYaml(signalTest, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		podmanTest.PodmanExitCleanly("kube", "down", kubeYaml)
		session := podmanTest.PodmanExitCleanly("run", "--volume", "testvol:/testvol", CITEST_IMAGE, "sh", "-c", "cat /testvol/termfile")
		Expect(session.OutputToString()).Should(ContainSubstring("TERMINATED"))
	})

	It("with hostPath subpaths", func() {
		if !Containerized() {
			Skip("something is wrong with file permissions in CI or in the yaml creation. cannot ls or cat the fs unless in a container")
		}

		hostPathLocation := podmanTest.TempDir
		Expect(os.MkdirAll(filepath.Join(hostPathLocation, "testing", "onlythis"), 0o755)).To(Succeed())
		file, err := os.Create(filepath.Join(hostPathLocation, "testing", "onlythis", "123.txt"))
		Expect(err).ToNot(HaveOccurred())

		_, err = file.WriteString("hi")
		Expect(err).ToNot(HaveOccurred())

		err = file.Close()
		Expect(err).ToNot(HaveOccurred())

		pod := getPod(withPodName("testpod"), withCtr(getCtr(withImage(CITEST_IMAGE), withName("testctr"), withCmd([]string{"top"}), withVolumeMount("/var", "testing/onlythis", false))), withVolume(getHostPathVolume("DirectoryOrCreate", hostPathLocation)))

		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).To(Not(HaveOccurred()))
		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		exec := podmanTest.PodmanExitCleanly("exec", "testpod-testctr", "ls", "/var")
		Expect(exec.OutputToString()).Should(ContainSubstring("123.txt"))
	})

	It("with unsafe subpaths", func() {
		SkipIfRemote("volume export does not exist on remote")
		podmanTest.PodmanExitCleanly("volume", "create", "testvol1")
		podmanTest.PodmanExitCleanly("run", "--volume", "testvol1:/data", CITEST_IMAGE, "sh", "-c", "mkdir -p /data/testing && ln -s /etc /data/testing/onlythis")

		tar := filepath.Join(podmanTest.TempDir, "out.tar")
		podmanTest.PodmanExitCleanly("volume", "export", "--output", tar, "testvol1")

		podmanTest.PodmanExitCleanly("volume", "create", "testvol")
		podmanTest.PodmanExitCleanly("volume", "import", "testvol", filepath.Join(podmanTest.TempDir, "out.tar"))

		err = writeYaml(subpathTestNamedVolume, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		playKube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		playKube.WaitWithDefaultTimeout()
		Expect(playKube).Should(ExitWithError(125, fmt.Sprintf("securejoin.OpenInRoot testing/onlythis: openat2 %s/root/volumes/testvol/_data/testing/onlythis: no such file or directory", podmanTest.TempDir)))
	})

	It("with unsafe hostPath subpaths", func() {
		hostPathLocation := filepath.Join(podmanTest.TempDir, "vol")

		Expect(os.MkdirAll(filepath.Join(hostPathLocation, "testing"), 0o755)).To(Succeed())
		Expect(os.Symlink("/", filepath.Join(hostPathLocation, "testing", "symlink"))).To(Succeed())

		pod := getPod(withPodName("testpod"), withCtr(getCtr(withImage(CITEST_IMAGE), withName("testctr"), withCmd([]string{"top"}), withVolumeMount("/foo", "testing/symlink", false))), withVolume(getHostPathVolume("DirectoryOrCreate", hostPathLocation)))

		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).To(Not(HaveOccurred()))

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
	})

	It("with configMap subpaths", func() {
		volumeName := "cmVol"
		cm := getConfigMap(withConfigMapName(volumeName), withConfigMapData("FOO", "foobar"))
		cmYaml, err := getKubeYaml("configmap", cm)
		Expect(err).ToNot(HaveOccurred())
		volumeContents := []map[string]string{{
			"key":  "FOO",
			"path": "BAR",
		}}

		ctr := getCtr(withPullPolicy("always"), withName("testctr"), withCmd([]string{"top"}), withVolumeMount("/etc/BAR", "BAR", false), withImage(CITEST_IMAGE))
		pod := getPod(withPodName("testpod"), withVolume(getConfigMapVolume(volumeName, volumeContents, false, nil)), withCtr(ctr))

		podYaml, err := getKubeYaml("pod", pod)
		Expect(err).ToNot(HaveOccurred())

		yamls := []string{cmYaml, podYaml}
		err = generateMultiDocKubeYaml(yamls, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		out, _ := os.ReadFile(kubeYaml)
		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0), string(out))

		exec := podmanTest.PodmanExitCleanly("exec", "testpod-testctr", "ls", "/etc/")
		Expect(exec.OutputToString()).ShouldNot(HaveLen(3))
		Expect(exec.OutputToString()).Should(ContainSubstring("BAR"))
		// we want to check that we can mount a subpath but not replace the entire dir
	})

	It("without Ports - curl should fail", func() {
		err := writeYaml(publishPortsPodWithoutPorts, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		curlTest := podmanTest.Podman([]string{"run", "--network", "host", NGINX_IMAGE, "curl", "-s", "localhost:19000"})
		curlTest.WaitWithDefaultTimeout()
		Expect(curlTest).Should(ExitWithError(7, ""))
	})

	It("without Ports, publish in command line - curl should succeed", func() {
		SkipIfNotAMD64() // https://github.com/containers/podman/issues/28272
		err := writeYaml(publishPortsPodWithoutPorts, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", "--publish", "19002:80", kubeYaml)
		testHTTPServer("19002", false, "podman rulez")
	})

	It("with privileged container ports - should fail", func() {
		SkipIfNotRootless("rootlessport can expose privileged port 80, no point in checking for failure")
		err := writeYaml(publishPortsPodWithContainerPort, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", "--publish-all=true", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitWithError(125, "rootlessport cannot expose privileged port 80"))
	})

	// Prevent these two tests from running in parallel
	Describe("with containerPort", Serial, func() {
		It("should not publish containerPort by default", func() {
			err := writeYaml(publishPortsPodWithContainerPort, kubeYaml)
			Expect(err).ToNot(HaveOccurred())

			podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
			testHTTPServer("80", true, "connection refused")
		})

		It("should publish containerPort with --publish-all", func() {
			SkipIfNotAMD64() // https://github.com/containers/podman/issues/28272
			SkipIfRootless("rootlessport can't expose privileged port 80")
			err := writeYaml(publishPortsPodWithContainerPort, kubeYaml)
			Expect(err).ToNot(HaveOccurred())

			podmanTest.PodmanExitCleanly("kube", "play", "--publish-all=true", kubeYaml)
			testHTTPServer("80", false, "podman rulez")
		})
	})

	It("with privileged containers ports and publish in command line - curl should succeed", func() {
		SkipIfNotAMD64() // https://github.com/containers/podman/issues/28272
		err := writeYaml(publishPortsPodWithContainerPort, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", "--publish", "19003:80", kubeYaml)
		testHTTPServer("19003", false, "podman rulez")
	})

	It("with Host Ports - curl should succeed", func() {
		SkipIfNotAMD64() // https://github.com/containers/podman/issues/28272
		err := writeYaml(publishPortsPodWithContainerHostPort, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", "--publish", "19004:80", kubeYaml)
		testHTTPServer("19004", false, "podman rulez")
	})

	It("with Host Ports and publish in command line - curl should succeed only on overriding port", func() {
		SkipIfNotAMD64() // https://github.com/containers/podman/issues/28272
		err := writeYaml(publishPortsPodWithContainerHostPort, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", "--publish", "19005:80", kubeYaml)
		testHTTPServer("19001", true, "connection refused")
		testHTTPServer("19005", false, "podman rulez")
	})

	It("multiple publish ports", func() {
		SkipIfNotAMD64() // https://github.com/containers/podman/issues/28272
		err := writeYaml(publishPortsPodWithoutPorts, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", "--publish", "19006:80", "--publish", "19007:80", kubeYaml)
		testHTTPServer("19006", false, "podman rulez")
		testHTTPServer("19007", false, "podman rulez")
	})

	It("override with tcp should keep udp from YAML file", func() {
		err := writeYaml(publishPortsEchoWithHostPortUDP, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", "--publish", "19010:19008/tcp", kubeYaml)
		verifyPodPorts(podmanTest, "network-echo", "19008/tcp:[{0.0.0.0 19010}]", "19008/udp:[{0.0.0.0 19009}]")
	})

	It("override with udp should keep tcp from YAML file", func() {
		err := writeYaml(publishPortsEchoWithHostPortTCP, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", "--publish", "19012:19008/udp", kubeYaml)

		verifyPodPorts(podmanTest, "network-echo", "19008/tcp:[{0.0.0.0 19011}]", "19008/udp:[{0.0.0.0 19012}]")
	})

	It("with replicas limits the count to 1 and emits a warning", func() {
		deployment := getDeployment(withReplicas(10))
		err := generateKubeYaml("deployment", deployment, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		// warnings are only propagated to local clients
		if !IsRemote() {
			Expect(kube.ErrorToString()).Should(ContainSubstring("Limiting replica count to 1, use `--replicas` to enable more than one replica"))
		}

		Expect(strings.Count(kube.OutputToString(), "Pod:")).To(Equal(1))
		Expect(strings.Count(kube.OutputToString(), "Container:")).To(Equal(1))
	})

	It("test with hostPID", func() {
		err := writeYaml(podWithHostPIDDefined, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		logs := podmanTest.PodmanExitCleanly("pod", "logs", "-c", "test-hostpid-testimage", "test-hostpid")
		Expect(logs.OutputToString()).To(Not(Equal("1")), "PID should never be 1 because of host pidns")

		inspect := podmanTest.PodmanExitCleanly("inspect", "test-hostpid-testimage", "--format", "{{ .HostConfig.PidMode }}")
		Expect(inspect.OutputToString()).To(Equal("host"))
	})

	It("test with hostIPC", func() {
		err := writeYaml(podWithHostIPCDefined, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		podmanTest.PodmanExitCleanly("wait", "test-hostipc-testimage")
		inspect := podmanTest.PodmanExitCleanly("inspect", "test-hostipc-testimage", "--format", "{{ .HostConfig.IpcMode }}")
		Expect(inspect.OutputToString()).To(Equal("host"))

		cmd := exec.Command("ls", "-l", "/proc/self/ns/ipc")
		res, err := cmd.Output()
		Expect(err).ToNot(HaveOccurred())
		fields := strings.Split(string(res), " ")
		hostIpcNS := strings.TrimSuffix(fields[len(fields)-1], "\n")

		logs := podmanTest.PodmanExitCleanly("pod", "logs", "-c", "test-hostipc-testimage", "test-hostipc")
		fields = strings.Split(logs.OutputToString(), " ")
		ctrIpcNS := strings.TrimSuffix(fields[len(fields)-1], "\n")

		Expect(ctrIpcNS).To(Equal(hostIpcNS), "container IPC NS == host IPC NS")
	})

	It("with ctrName should be in network alias", func() {
		ctrName := "test-ctr"
		ctrNameInKubePod := ctrName + "-pod-" + ctrName
		podmanTest.PodmanExitCleanly("run", "-d", "--name", ctrName, CITEST_IMAGE, "top")

		outputFile := filepath.Join(podmanTest.RunRoot, "pod.yaml")
		podmanTest.PodmanExitCleanly("kube", "generate", ctrName, "-f", outputFile)
		podmanTest.PodmanExitCleanly("pod", "rm", "-t", "0", "-f", ctrName)

		podmanTest.PodmanExitCleanly("kube", "play", outputFile)
		inspect := podmanTest.PodmanExitCleanly("inspect", ctrNameInKubePod)
		Expect(inspect.OutputToString()).To(ContainSubstring("\"Aliases\": [ \"" + ctrName + "\""))
	})

	It("test with sysctl defined", func() {
		SkipIfRootless("Network sysctls are not available for rootless")
		err := writeYaml(podWithSysctlDefined, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		podmanTest.PodmanExitCleanly("wait", "test-sysctl-testimage")
		logs := podmanTest.PodmanExitCleanly("pod", "logs", "-c", "test-sysctl-testimage", "test-sysctl")
		Expect(logs.OutputToString()).To(ContainSubstring("kernel.msgmax = 65535"))
		Expect(logs.OutputToString()).To(ContainSubstring("net.core.somaxconn = 65535"))
	})

	It("test with sysctl & host network defined", func() {
		SkipIfRootless("Network sysctls are not available for rootless")
		err := writeYaml(podWithSysctlHostNetDefined, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitWithError(125, "since Network Namespace set to host: invalid argument"))
	})

	It("test with annotation size beyond limits", func() {
		key := "name"
		val := RandomString(define.TotalAnnotationSizeLimitB - len(key) + 1)
		pod := getPod(withAnnotation(key, val))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitWithError(125, "annotations size "+strconv.Itoa(len(key+val))+" is larger than limit "+strconv.Itoa(define.TotalAnnotationSizeLimitB)))
	})

	It("test with annotation size within limits", func() {
		key := "name"
		val := RandomString(define.TotalAnnotationSizeLimitB - len(key))
		pod := getPod(withAnnotation(key, val))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
	})

	It("test pod with volumes-from annotation in yaml", func() {
		ctr1 := "ctr1"
		ctr2 := "ctr2"
		ctrNameInKubePod := ctr2 + "-pod-" + ctr2
		outputFile := filepath.Join(podmanTest.TempDir, "pod.yaml")
		vol1 := filepath.Join(podmanTest.TempDir, "vol-test1")

		err := os.MkdirAll(vol1, 0o755)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("create", "--name", ctr1, "-v", vol1, CITEST_IMAGE)
		podmanTest.PodmanExitCleanly("create", "--volumes-from", ctr1, "--name", ctr2, CITEST_IMAGE)
		podmanTest.PodmanExitCleanly("kube", "generate", "-f", outputFile, ctr2)
		podmanTest.PodmanExitCleanly("kube", "play", outputFile)

		inspectCtr2 := podmanTest.PodmanExitCleanly("inspect", "-f", "'{{ .HostConfig.Binds }}'", ctrNameInKubePod)
		Expect(inspectCtr2.OutputToString()).To(ContainSubstring(":" + vol1))

		inspectCtr1 := podmanTest.PodmanExitCleanly("inspect", "-f", "'{{ .HostConfig.Binds }}'", ctr1)
		Expect(inspectCtr2.OutputToString()).To(Equal(inspectCtr1.OutputToString()))

		// see https://github.com/containers/podman/pull/19637, we should not see any warning/errors here
		podmanTest.PodmanExitCleanly("kube", "down", outputFile)
	})

	It("test volumes-from annotation with source containers external", func() {
		// Assert that volumes of multiple source containers, listed in
		// volumes-from annotation, running outside the pod are
		// getting mounted inside the target container.

		srcctr1, srcctr2, tgtctr := "srcctr1", "srcctr2", "tgtctr"
		frmopt1, frmopt2 := srcctr1+":ro", srcctr2+":ro"
		vol1 := filepath.Join(podmanTest.TempDir, "vol-test1")
		vol2 := filepath.Join(podmanTest.TempDir, "vol-test2")

		volsFromAnnotaton := define.VolumesFromAnnotation + "/" + tgtctr
		volsFromValue := frmopt1 + ";" + frmopt2

		err1 := os.MkdirAll(vol1, 0o755)
		Expect(err1).ToNot(HaveOccurred())

		err2 := os.MkdirAll(vol2, 0o755)
		Expect(err2).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("create", "--name", srcctr1, "-v", vol1, CITEST_IMAGE)
		podmanTest.PodmanExitCleanly("create", "--name", srcctr2, "-v", vol2, CITEST_IMAGE)

		podName := tgtctr
		pod := getPod(
			withPodName(podName),
			withCtr(getCtr(withName(tgtctr))),
			withAnnotation(volsFromAnnotaton, volsFromValue))

		err3 := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err3).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		// Assert volumes are accessible inside the target container
		ctrNameInKubePod := podName + "-" + tgtctr

		inspect := podmanTest.InspectContainer(ctrNameInKubePod)
		Expect(inspect).To(HaveLen(1))

		exec := podmanTest.PodmanExitCleanly("exec", ctrNameInKubePod, "ls", "-d", vol1, vol2)
		Expect(exec.OutputToString()).To(ContainSubstring(vol1))
		Expect(exec.OutputToString()).To(ContainSubstring(vol2))
	})

	It("test volumes-from annotation with source container in pod", func() {
		// Assert that volume of source container, member of the pod,
		// listed in volumes-from annotation is getting mounted inside
		// the target container.

		srcctr, tgtctr, podName := "srcctr", "tgtctr", "volspod"
		vol := "/mnt/vol"

		srcctrInKubePod := podName + "-" + srcctr
		tgtctrInKubePod := podName + "-" + tgtctr

		err := writeYaml(volumesFromPodYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)

		inspect := podmanTest.InspectContainer(tgtctrInKubePod)
		Expect(inspect).To(HaveLen(1))

		// Assert volume is accessible inside the target container
		// by creating contents in the volume and accessing that from
		// the target container.
		volFile := filepath.Join(vol, RandomString(10)+".txt")

		podmanTest.PodmanExitCleanly("exec", srcctrInKubePod, "touch", volFile)
		exec := podmanTest.PodmanExitCleanly("exec", srcctrInKubePod, "ls", volFile)
		Expect(exec.OutputToString()).To(ContainSubstring(volFile))

		exec = podmanTest.PodmanExitCleanly("exec", tgtctrInKubePod, "ls", volFile)
		Expect(exec.OutputToString()).To(ContainSubstring(volFile))
	})

	It("test with reserved volumes-from annotation in yaml", func() {
		// Assert that volumes-from annotation without target container
		// errors out.

		pod := getPod(withAnnotation(define.VolumesFromAnnotation, "reserved"))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitWithError(125, "annotation "+define.VolumesFromAnnotation+" without target volume is reserved for internal use"))
	})

	It("test with reserved autoremove annotation in yaml", func() {
		ctr := "ctr"
		ctrNameInKubePod := ctr + "-pod-" + ctr
		outputFile := filepath.Join(podmanTest.TempDir, "pod.yaml")

		podmanTest.PodmanExitCleanly("create", "--rm", "--name", ctr, CITEST_IMAGE)

		podmanTest.PodmanExitCleanly("kube", "generate", "--podman-only", "-f", outputFile, ctr)
		podmanTest.PodmanExitCleanly("kube", "play", outputFile)
		inspect := podmanTest.PodmanExitCleanly("inspect", "-f", "{{ .HostConfig.AutoRemove }}", ctrNameInKubePod)
		Expect(inspect.OutputToString()).To(Equal("true"))
	})

	It("test with reserved privileged annotation in yaml", func() {
		ctr := "ctr"
		ctrNameInKubePod := ctr + "-pod-" + ctr
		outputFile := filepath.Join(podmanTest.TempDir, "pod.yaml")

		podmanTest.PodmanExitCleanly("create", "--privileged", "--name", ctr, CITEST_IMAGE)
		podmanTest.PodmanExitCleanly("kube", "generate", "--podman-only", "-f", outputFile, ctr)
		podmanTest.PodmanExitCleanly("kube", "play", outputFile)
		inspect := podmanTest.PodmanExitCleanly("inspect", "-f", "{{ .HostConfig.Privileged }}", ctrNameInKubePod)
		Expect(inspect.OutputToString()).To(Equal("true"))
	})

	It("test with reserved init annotation in yaml", func() {
		ctr := "ctr"
		ctrNameInKubePod := ctr + "-pod-" + ctr
		outputFile := filepath.Join(podmanTest.TempDir, "pod.yaml")

		podmanTest.PodmanExitCleanly("create", "--init", "--name", ctr, CITEST_IMAGE)
		podmanTest.PodmanExitCleanly("kube", "generate", "--podman-only", "-f", outputFile, ctr)
		podmanTest.PodmanExitCleanly("kube", "play", outputFile)
		inspect := podmanTest.PodmanExitCleanly("inspect", "-f", "{{ .Path }}", ctrNameInKubePod)
		Expect(inspect.OutputToString()).To(Equal("/run/podman-init"))
	})

	It("test with reserved CIDFile annotation in yaml", func() {
		ctr := "ctr"
		ctrNameInKubePod := ctr + "-pod-" + ctr
		outputFile := filepath.Join(podmanTest.TempDir, "pod.yaml")
		cidFile := filepath.Join(podmanTest.TempDir, RandomString(10)+".txt")

		podmanTest.PodmanExitCleanly("create", "--cidfile", cidFile, "--name", ctr, CITEST_IMAGE)
		podmanTest.PodmanExitCleanly("kube", "generate", "--podman-only", "-f", outputFile, ctr)
		podmanTest.PodmanExitCleanly("kube", "play", outputFile)
		inspect := podmanTest.PodmanExitCleanly("inspect", "-f", "{{ .HostConfig.ContainerIDFile }}", ctrNameInKubePod)
		Expect(inspect.OutputToString()).To(Equal(cidFile))
	})

	It("test with reserved Seccomp annotation in yaml", func() {
		ctr := "ctr"
		ctrNameInKubePod := ctr + "-pod-" + ctr
		outputFile := filepath.Join(podmanTest.TempDir, "pod.yaml")

		podmanTest.PodmanExitCleanly("create", "--security-opt", "seccomp=unconfined", "--name", ctr, CITEST_IMAGE)
		podmanTest.PodmanExitCleanly("kube", "generate", "--podman-only", "-f", outputFile, ctr)
		podmanTest.PodmanExitCleanly("kube", "play", outputFile)
		inspect := podmanTest.PodmanExitCleanly("inspect", "-f", "{{ .HostConfig.SecurityOpt }}", ctrNameInKubePod)
		Expect(inspect.OutputToString()).To(Equal("[seccomp=unconfined]"))
	})

	It("test with reserved Apparmor annotation in yaml", func() {
		ctr := "ctr"
		ctrNameInKubePod := ctr + "-pod-" + ctr
		outputFile := filepath.Join(podmanTest.TempDir, "pod.yaml")

		podmanTest.PodmanExitCleanly("create", "--security-opt", "apparmor=unconfined", "--name", ctr, CITEST_IMAGE)
		podmanTest.PodmanExitCleanly("kube", "generate", "--podman-only", "-f", outputFile, ctr)
		podmanTest.PodmanExitCleanly("kube", "play", outputFile)
		inspect := podmanTest.PodmanExitCleanly("inspect", "-f", "{{ .HostConfig.SecurityOpt }}", ctrNameInKubePod)
		Expect(inspect.OutputToString()).To(Equal("[apparmor=unconfined]"))
	})

	It("test with reserved Label annotation in yaml", func() {
		ctr := "ctr"
		ctrNameInKubePod := ctr + "-pod-" + ctr
		outputFile := filepath.Join(podmanTest.TempDir, "pod.yaml")

		podmanTest.PodmanExitCleanly("create", "--security-opt", "label=level:s0", "--name", ctr, CITEST_IMAGE)
		podmanTest.PodmanExitCleanly("kube", "generate", "--podman-only", "-f", outputFile, ctr)
		podmanTest.PodmanExitCleanly("kube", "play", outputFile)
		inspect := podmanTest.PodmanExitCleanly("inspect", "-f", "{{ .HostConfig.SecurityOpt }}", ctrNameInKubePod)
		Expect(inspect.OutputToString()).To(Equal("[label=level:s0]"))
	})

	It("test with reserved PublishAll annotation in yaml", func() {
		podmanTest.AddImageToRWStore(CITEST_IMAGE)
		dockerfile := fmt.Sprintf(`FROM %s
EXPOSE 2002
EXPOSE 2001-2003
EXPOSE 2004-2005/tcp`, CITEST_IMAGE)
		imageName := "testimg"
		podmanTest.BuildImage(dockerfile, imageName, "false")

		// Verify that the buildah is just passing through the EXPOSE keys
		inspect := podmanTest.PodmanExitCleanly("inspect", imageName)
		image := inspect.InspectImageJSON()
		Expect(image).To(HaveLen(1))
		Expect(image[0].Config.ExposedPorts).To(HaveLen(3))
		Expect(image[0].Config.ExposedPorts).To(HaveKey("2002/tcp"))
		Expect(image[0].Config.ExposedPorts).To(HaveKey("2001-2003/tcp"))
		Expect(image[0].Config.ExposedPorts).To(HaveKey("2004-2005/tcp"))

		ctr := "ctr"
		ctrNameInKubePod := ctr + "-pod-" + ctr
		outputFile := filepath.Join(podmanTest.TempDir, "pod.yaml")

		podmanTest.PodmanExitCleanly("create", "--publish-all", "--name", ctr, imageName, "true")
		podmanTest.PodmanExitCleanly("kube", "generate", "--podman-only", "-f", outputFile, ctr)
		podmanTest.PodmanExitCleanly("kube", "play", outputFile)
		inspect = podmanTest.PodmanExitCleanly("inspect", "-f", "{{ .HostConfig.PublishAllPorts }}", ctrNameInKubePod)
		Expect(inspect.OutputToString()).To(Equal("true"))
	})

	It("test with valid Umask value", func() {
		defaultUmask := "0022"
		ctrName := "ctr"
		ctrNameInPod := "ctr-pod-ctr"
		outputFile := filepath.Join(podmanTest.TempDir, "pod.yaml")

		podmanTest.PodmanExitCleanly("create", "-t", "--restart", "never", "--name", ctrName, CITEST_IMAGE, "top")
		podmanTest.PodmanExitCleanly("kube", "generate", "-f", outputFile, ctrName)
		podmanTest.PodmanExitCleanly("kube", "play", outputFile)
		exec := podmanTest.PodmanExitCleanly("exec", ctrNameInPod, "/bin/sh", "-c", "umask")
		Expect(exec.OutputToString()).To(Equal(defaultUmask))

		inspect := podmanTest.PodmanExitCleanly("inspect", ctrNameInPod, "-f", "{{ .Config.Umask }}")
		Expect(inspect.OutputToString()).To(Equal(defaultUmask))
	})

	// podman play with infra name annotation
	It("test with infra name annotation set", func() {
		infraName := "infra-ctr"
		podName := "mypod"
		outputFile := filepath.Join(podmanTest.TempDir, "pod.yaml")
		podmanTest.PodmanExitCleanly("pod", "create", "--infra-name", infraName, podName)
		podmanTest.PodmanExitCleanly("create", "--pod", podName, CITEST_IMAGE, "top")
		// Generate kube yaml and it should have the infra name annotation set
		podmanTest.PodmanExitCleanly("kube", "generate", "-f", outputFile, podName)
		//  Remove the pod so it can be recreated via kube play
		podmanTest.PodmanExitCleanly("pod", "rm", "-f", podName)
		podmanTest.PodmanExitCleanly("kube", "play", outputFile)
		// Expect the number of containers created to be 2, infra, and regular container
		numOfCtrs := podmanTest.NumberOfContainers()
		Expect(numOfCtrs).To(Equal(2))

		ps := podmanTest.PodmanExitCleanly("ps", "--format", "{{.Names}}")
		Expect(ps.OutputToString()).To(ContainSubstring(infraName))
	})

	// podman play with default infra name
	It("test with default infra name", func() {
		podName := "mypod"
		outputFile := filepath.Join(podmanTest.TempDir, "pod.yaml")
		podmanTest.PodmanExitCleanly("pod", "create", podName)
		podmanTest.PodmanExitCleanly("create", "--pod", podName, CITEST_IMAGE, "top")

		// Generate kube yaml and it should have the infra name annotation set
		podmanTest.PodmanExitCleanly("kube", "generate", "-f", outputFile, podName)
		//  Remove the pod so it can be recreated via kube play
		podmanTest.PodmanExitCleanly("pod", "rm", "-f", podName)
		podmanTest.PodmanExitCleanly("kube", "play", outputFile)

		// Expect the number of containers created to be 2, infra, and regular container
		numOfCtrs := podmanTest.NumberOfContainers()
		Expect(numOfCtrs).To(Equal(2))

		podPs := podmanTest.PodmanExitCleanly("pod", "ps", "-q")
		podID := podPs.OutputToString()

		ps := podmanTest.PodmanExitCleanly("ps", "--format", "{{.Names}}")
		Expect(ps.OutputToString()).To(ContainSubstring(podID[:12] + "-infra"))
	})

	It("support List kind", func() {
		listYamlPathname := filepath.Join(podmanTest.TempDir, "list.yaml")
		err = writeYaml(listPodAndConfigMap, listYamlPathname)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", listYamlPathname)
		inspect := podmanTest.PodmanExitCleanly("inspect", "test-list-pod-container", "--format", "'{{ .Config.Env }}'")
		Expect(inspect.OutputToString()).To(ContainSubstring(`FOO=bar`))
	})

	It("with TerminationGracePeriodSeconds set", func() {
		ctrName := "ctr"
		ctrNameInPod := "ctr-pod-ctr"
		outputFile := filepath.Join(podmanTest.TempDir, "pod.yaml")

		podmanTest.PodmanExitCleanly("create", "--restart", "never", "--stop-timeout", "20", "--name", ctrName, CITEST_IMAGE)
		podmanTest.PodmanExitCleanly("kube", "generate", "-f", outputFile, ctrName)
		podmanTest.PodmanExitCleanly("kube", "play", outputFile)
		inspect := podmanTest.PodmanExitCleanly("inspect", ctrNameInPod, "-f", "{{ .Config.StopTimeout }}")
		Expect(inspect.OutputToString()).To(Equal("20"))
	})

	It("hostname should be node name when hostNetwork=true", func() {
		netYaml := `
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  hostNetwork: true
  hostname: blah
  containers:
    - name: alpine
      image: alpine
      command:
        - sleep
        - "100"
`

		err := writeYaml(netYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())
		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)

		// Get the name of the host
		hostname, err := os.Hostname()
		Expect(err).ToNot(HaveOccurred())

		exec := podmanTest.PodmanExitCleanly("exec", "test-pod-alpine", "hostname")
		Expect(exec.OutputToString()).To(Equal(hostname))

		// Check that the UTS namespace is set to host also
		hostUts := SystemExec("ls", []string{"-l", "/proc/self/ns/uts"})
		Expect(hostUts).Should(ExitCleanly())
		arr := strings.Split(hostUts.OutputToString(), " ")
		exec = podmanTest.PodmanExitCleanly("exec", "test-pod-alpine", "ls", "-l", "/proc/self/ns/uts")
		execArr := strings.Split(exec.OutputToString(), " ")
		Expect(execArr[len(execArr)-1]).To(ContainSubstring(arr[len(arr)-1]))
	})

	It("hostname should be pod name when hostNetwork=false", func() {
		netYaml := `
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  containers:
    - name: alpine
      image: alpine
      command:
        - sleep
        - "100"
`

		err := writeYaml(netYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())
		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		exec := podmanTest.PodmanExitCleanly("exec", "test-pod-alpine", "hostname")
		Expect(exec.OutputToString()).To(Equal("test-pod"))

		// Check that the UTS namespace is set to host also
		hostUts := SystemExec("ls", []string{"-l", "/proc/self/ns/uts"})
		Expect(hostUts).Should(ExitCleanly())
		arr := strings.Split(hostUts.OutputToString(), " ")
		exec = podmanTest.PodmanExitCleanly("exec", "test-pod-alpine", "ls", "-l", "/proc/self/ns/uts")
		execArr := strings.Split(exec.OutputToString(), " ")
		Expect(execArr[len(execArr)-1]).To(Not(ContainSubstring(arr[len(arr)-1])))
	})

	It("test pids-limit annotation", func() {
		ctrAnnotation := "io.podman.annotations.pids-limit/" + defaultCtrName
		pod := getPod(withAnnotation(ctrAnnotation, "10"), withPodInitCtr(getCtr(withImage(CITEST_IMAGE), withCmd([]string{"printenv", "container"}), withInitCtr(), withName("init-test"))), withCtr(getCtr(withImage(CITEST_IMAGE), withCmd([]string{"top"}))))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		exec := podmanTest.PodmanExitCleanly("exec", "testPod-"+defaultCtrName, "cat", "/sys/fs/cgroup/pids.max")
		Expect(exec.OutputToString()).To(Equal("10"))
	})

	It("test cpuset annotation", func() {
		SkipIfRootless("access to cpuset cgroup controller is needed")
		ctrAnnotation := "io.podman.annotations.cpuset/" + defaultCtrName
		pod := getPod(withAnnotation(ctrAnnotation, "0"), withPodInitCtr(getCtr(withImage(CITEST_IMAGE), withCmd([]string{"printenv", "container"}), withInitCtr(), withName("init-test"))), withCtr(getCtr(withImage(CITEST_IMAGE), withCmd([]string{"top"}))))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		exec := podmanTest.PodmanExitCleanly("exec", "testPod-"+defaultCtrName, "cat", "/sys/fs/cgroup/cpuset.cpus.effective")
		Expect(exec.OutputToString()).To(Equal("0"))
	})

	It("test memory-nodes annotation", func() {
		SkipIfRootless("access to cpuset cgroup controller is needed")
		ctrAnnotation := "io.podman.annotations.memory-nodes/" + defaultCtrName
		pod := getPod(withAnnotation(ctrAnnotation, "0"), withPodInitCtr(getCtr(withImage(CITEST_IMAGE), withCmd([]string{"printenv", "container"}), withInitCtr(), withName("init-test"))), withCtr(getCtr(withImage(CITEST_IMAGE), withCmd([]string{"top"}))))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		exec := podmanTest.PodmanExitCleanly("exec", "testPod-"+defaultCtrName, "cat", "/sys/fs/cgroup/cpuset.mems.effective")
		Expect(exec.OutputToString()).To(Equal("0"))
	})

	It("test Lifecycle stopSignal", func() {
		// Default StopSignal SIGTERM
		err = writeYaml(simplePodYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		inspect := podmanTest.PodmanExitCleanly("inspect", "libpod-test-")

		ctr := inspect.InspectContainerToJSON()
		Expect(ctr[0].Config).To(HaveField("StopSignal", "SIGTERM"))

		// Custom StopSignal SIGUSR1
		err = writeYaml(stopSignalSIGUSR1, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		inspect = podmanTest.PodmanExitCleanly("inspect", "testpod-testctr")

		ctr = inspect.InspectContainerToJSON()
		Expect(ctr[0].Config).To(HaveField("StopSignal", "SIGUSR1"))

		// No such StopSignal
		err = writeYaml(stopSignalNosuchSignal, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "invalid signal: noSuchSignal"))
	})

	It("test with custom log path from containers.conf", func() {
		customLogPath := filepath.Join(podmanTest.TempDir, "custom-logs")
		expectedMessage := "Pod started, checking logs from test"

		conffile := filepath.Join(podmanTest.TempDir, "containers.conf")
		configContent := fmt.Sprintf(`[containers]
log_driver = "k8s-file"
log_path = "%s"
`, customLogPath)

		err := os.WriteFile(conffile, []byte(configContent), 0o644)
		Expect(err).ToNot(HaveOccurred())

		err = os.MkdirAll(customLogPath, 0o755)
		Expect(err).ToNot(HaveOccurred())

		os.Setenv("CONTAINERS_CONF_OVERRIDE", conffile)
		defer os.Unsetenv("CONTAINERS_CONF_OVERRIDE")

		if IsRemote() {
			podmanTest.RestartRemoteService()
		}

		kubeYaml := filepath.Join(podmanTest.TempDir, "test-pod.yaml")
		podYamlContent := fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: log-test-pod
spec:
  restartPolicy: Never
  containers:
  - name: logger-container
    image: %s
    command: ["/bin/sh", "-c", "echo '%s'; sleep 2"]
`, CITEST_IMAGE, expectedMessage)

		err = os.WriteFile(kubeYaml, []byte(podYamlContent), 0o644)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)

		podmanTest.PodmanExitCleanly("wait", "log-test-pod-logger-container")

		customLogDirs, err := os.ReadDir(customLogPath)
		Expect(err).ToNot(HaveOccurred())
		Expect(customLogDirs).To(HaveLen(2), "Should have exactly two container log directories (infra + app container)")

		var appContainerLogDir string
		var logContent string
		found := false

		for _, dir := range customLogDirs {
			if !dir.IsDir() {
				continue
			}
			containerLogDir := dir.Name()
			logFilePath := filepath.Join(customLogPath, containerLogDir, "ctr.log")

			if _, err := os.Stat(logFilePath); err != nil {
				continue
			}

			content, err := os.ReadFile(logFilePath)
			if err != nil {
				continue
			}

			if strings.Contains(string(content), expectedMessage) {
				appContainerLogDir = containerLogDir
				logContent = string(content)
				found = true
				break
			}
		}

		Expect(found).To(BeTrue(), "Should find log file with expected message")

		Expect(appContainerLogDir).ToNot(BeEmpty(), "Should have found application container log directory")
		Expect(logContent).To(ContainSubstring(expectedMessage), "Log file should contain the expected message")
	})

	It("CVE-2025-9566 regression test - ConfigMap mount", func() {
		testfile := filepath.Join(podmanTest.TempDir, "testfile")
		volumeName := "cm-vol"
		cm := getConfigMap(withConfigMapName(volumeName), withConfigMapData("foo", "content1"))
		cmYaml, err := getKubeYaml("configmap", cm)
		Expect(err).ToNot(HaveOccurred())

		ctrName := "ctr1"
		podName := "pod1"
		// create a symlink at the volume mount location so we can make sure we don't resolve that to the host location.
		ctr := getCtr(withName(ctrName), withVolumeMount("/test", "", false), withImage(CITEST_IMAGE), withCmd([]string{"sh", "-c", "ln -sf " + testfile + " /test/foo"}))
		pod := getPod(withPodName(podName), withVolume(getConfigMapVolume(volumeName, nil, false, nil)), withCtr(ctr))
		podYaml, err := getKubeYaml("pod", pod)
		Expect(err).ToNot(HaveOccurred())
		yamls := []string{cmYaml, podYaml}
		err = generateMultiDocKubeYaml(yamls, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", kubeYaml)
		// wait for the container to finish to ensure the symlink was created
		podmanTest.PodmanExitCleanly("wait", podName+"-"+ctrName)
		podmanTest.PodmanExitCleanly("kube", "down", kubeYaml)
		kube := podmanTest.Podman([]string{"kube", "play", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError(125, `cannot create file "foo" at volume mountpoint`))

		Expect(testfile).ToNot(BeAnExistingFile(), "file should never be created on the host")
	})

	It("test container name without Pod name prefix", func() {
		err := writeYaml(simpleWithoutPodPrefixYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		podmanTest.PodmanExitCleanly("kube", "play", "--no-pod-prefix", kubeYaml)
		inspect := podmanTest.PodmanExitCleanly("inspect", "simpleWithoutPodPrefix")
		Expect(inspect.InspectContainerToJSON()[0].Name).Should(Equal("simpleWithoutPodPrefix"))
	})

	It("multiple Pod replicas", func() {
		err := writeYaml(replicasPodYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", "--replicas", "--publish", "8080:80,8081:80", kubeYaml})
		kube.WaitWithDefaultTimeout()

		podsCount := podmanTest.PodmanExitCleanly("pod", "ps", "-n")
		podsCount.WaitWithDefaultTimeout()
		Expect(podsCount.OutputToStringArray()).To(HaveLen(2))
	})
})
