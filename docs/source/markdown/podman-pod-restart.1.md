% podman-pod-restart 1

## NAME
podman\-pod\-restart - Restart one or more pods

## SYNOPSIS
**podman pod restart** [*options*] *pod* ...

## DESCRIPTION
Restart containers in one or more pods. Running containers are stopped and restarted.
Stopped containers are started. You may use pod IDs or names as input.
The pod ID is printed upon successful restart.
When restarting multiple pods, an error from restarting one pod does not effect restarting other pods.

## OPTIONS

#### **--all**, **-a**

Restarts all pods

#### **--latest**, **-l**

Instead of providing the pod name or ID, restart the last created pod. (This option is not available with the remote Podman client, including Mac and Windows (excluding WSL2) machines)

## EXAMPLE

Restart pod with a given name:
```
podman pod restart mywebserverpod
cc8f0bea67b1a1a11aec1ecd38102a1be4b145577f21fc843c7c83b77fc28907
```

Restart multiple pods with given IDs:
```
podman pod restart 490eb 3557fb
490eb241aaf704d4dd2629904410fe4aa31965d9310a735f8755267f4ded1de5
3557fbea6ad61569de0506fe037479bd9896603c31d3069a6677f23833916fab
```

Restart the last created pod:
```
podman pod restart --latest
3557fbea6ad61569de0506fe037479bd9896603c31d3069a6677f23833916fab
```

Restart all pods:
```
podman pod restart --all
19456b4cd557eaf9629825113a552681a6013f8c8cad258e36ab825ef536e818
3557fbea6ad61569de0506fe037479bd9896603c31d3069a6677f23833916fab
490eb241aaf704d4dd2629904410fe4aa31965d9310a735f8755267f4ded1de5
70c358daecf71ef9be8f62404f926080ca0133277ef7ce4f6aa2d5af6bb2d3e9
cc8f0bea67b1a1a11aec1ecd38102a1be4b145577f21fc843c7c83b77fc28907
```
## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-pod(1)](podman-pod.1.md)**, **[podman-restart(1)](podman-restart.1.md)**

## HISTORY
July 2018, Originally compiled by Peter Hunt <pehunt@redhat.com>
