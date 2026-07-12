> - 译文出自：[掘金翻译计划](https://juejin.cn/translate)

![PODMAN logo](https://raw.githubusercontent.com/containers/common/main/logos/podman-logo-full-vert.png)

Podman是由libpod库提供一个实用的程序，可以被用于创建和管理容器。

下面的教程会教你如何启动 Podman 并使用 Podman 执行一些基本的命令。

如果你正在使用 Mac 或者 Windows
，你应该先查看[Mac 和 Windows 使用说明](https://github.com/containers/podman/blob/main/docs/tutorials/mac_win_client.md)来设置 Podman
远程客户端。

**注意**：示例中所有命令皆以非 root 的用户运行，必要的时候通过 `sudo` 命令来获取 root 权限。

## 安装Podman

安装或者编译 Podman ，请参照[安装说明](https://podman.io/getting-started/installation)。

## 熟悉podman

### 运行一个示例容器

这个示例容器会运行一个简单的只有主页的 httpd 服务器。

```console
podman run --name basic_httpd -d -p 8080:80/tcp docker.io/nginx
```

因为命令中的 *-d* 参数表明容器以 "detached" 模式运行，所以 Podman 会在容器运行后打印容器的 ID。

注意为了访问这个 HTTP 服务器，我们将使用端口转发。

Podman 的 *ps* 命令用于列出正在创建和运行的容器。

```console
podman ps
```

**注意**：如果为 *ps* 命令添加 *-a* 参数，Podman 将展示所有的容器。

### 查看正在运行的容器

你可以 "inspect" (查看)一个正在运行的容器的元数据以及其他详细信息。我们甚至可以使用 inspect 的子命令查看分配给容器的 IP 地址。由于容器以非 root 模式运行，通常不会分配 IP 地址，该字段可能为空。

```console
podman inspect basic_httpd --format 'IP address: {{.NetworkSettings.IPAddress}}'
```

示例输出（非 root 模式）：

```text
IP address:
```

**注意**：你也可以使用 *-l* 查看**最近的容器**，或使用容器 ID 代替容器名称。

### 测试httpd服务器

由于我们没有容器的 IP 地址，我们可以使用 curl 测试主机和容器之间的网络通信。下面的命令应该显示我们的容器化 httpd 服务器 的主页。

```console
curl http://localhost:8080
```

### 查看容器的日志

你也可以使用 podman 查看容器的日志

```console
podman logs --latest
```

示例输出：

```text
10.88.0.1 - - [07/Feb/2018:15:22:11 +0000] "GET / HTTP/1.1" 200 612 "-" "curl/7.55.1" "-"
10.88.0.1 - - [07/Feb/2018:15:22:30 +0000] "GET / HTTP/1.1" 200 612 "-" "curl/7.55.1" "-"
10.88.0.1 - - [07/Feb/2018:15:22:30 +0000] "GET / HTTP/1.1" 200 612 "-" "curl/7.55.1" "-"
10.88.0.1 - - [07/Feb/2018:15:22:31 +0000] "GET / HTTP/1.1" 200 612 "-" "curl/7.55.1" "-"
10.88.0.1 - - [07/Feb/2018:15:22:31 +0000] "GET / HTTP/1.1" 200 612 "-" "curl/7.55.1" "-"
```

### 查看容器中的进程pid

你可以使用 *top* 命令查看容器中 httpd 的 pid

```console
podman top <container_id>
```

示例输出：

```text
  UID   PID  PPID  C STIME TTY          TIME CMD
    0 31873 31863  0 09:21 ?        00:00:00 nginx: master process nginx -g daemon off;
  101 31889 31873  0 09:21 ?        00:00:00 nginx: worker process
```

### 设置容器的检查点

设置检查点会在停止容器的同时把容器中所有进程的状态写入磁盘。

有了它，容器后续可以被恢复，并在与检查点完全相同的时间点继续运行。 这个功能需要在系统上安装 CRIU 的 3.11+ 版本。

这个功能不支持非 root 模式，因此，如果你想尝试使用它，你需要使用 sudo 方式重新创建容器。

设置容器检查点请使用：

```console
sudo podman container checkpoint <container_id>
```

### 恢复容器

恢复容器只能在以前设置过检查点的容器上使用。恢复的容器会在与设置检查点时完全相同的时间点继续运行。

恢复容器请使用：

```console
sudo podman container restore <container_id>
```

恢复之后。容器会像设置检查点之前一样回复请求

```console
curl http://<IP_address>:8080
```

### 迁移容器

为了将容器从一个主机上热迁移到另一个主机，容器可以在在源系统上创建检查点，传输到目的系统，然后再在目的系统上恢复。
为了便于传输容器的检查点，可以将其存储在一个指定的输出文件中。

在源系统上：

```console
sudo podman container checkpoint <container_id> -e /tmp/checkpoint.tar.gz
scp /tmp/checkpoint.tar.gz <destination_system>:/tmp
```

在目标系统上：

```console
sudo podman container restore -i /tmp/checkpoint.tar.gz
```

恢复之后，容器会像设置检查点之前一样回复请求。这时，容器会在目标系统上继续运行。

```console
curl http://<IP_address>:8080
```

### 停止容器

停止 httpd 容器

```console
podman stop --latest
```

你还可以使用 *ps* 命令检查一个或多个容器的状态，在这个例子中，我们使用 *-a* 参数列出所有的容器。

```console
podman ps -a
```

### 移除容器

移除 httpd 容器

```console
podman rm --latest
```

你可以使用 *podman ps -a* 验证容器的删除。

## 集成测试

在环境中如何设置并运行集成测试请查看集成测试的[自述页面](../../test/README.md)

## 更多信息

有关podman 和它的子命令的更多信息请查看 podman 的[自述页面](../../README.md#commands)
