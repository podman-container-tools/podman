#!/usr/bin/env bats   -*- bats -*-
#
# Tests for podman quadlet install with multi-quadlet files
#

load helpers
load helpers.systemd

function setup() {
    skip_if_remote "podman quadlet is not implemented for remote setup yet"
    skip_if_journald_unavailable "Needed for RHEL. FIXME: we might be able to re-enable a subset of tests."

    basic_setup
}

function teardown() {
    # remove any remaining quadlets from tests
    run_podman quadlet rm --all --recursive -f
    systemctl daemon-reload
    basic_teardown
}

# Helper function to get the systemd install directory based on rootless/root mode
function get_quadlet_install_dir() {
    if is_rootless; then
        # For rootless: $XDG_CONFIG_HOME/containers/systemd or ~/.config/containers/systemd
        local config_home=${XDG_CONFIG_HOME:-$HOME/.config}
        echo "$config_home/containers/systemd"
    else
        # For root: /etc/containers/systemd
        echo "/etc/containers/systemd"
    fi
}

@test "quadlet verb - install multi-quadlet file as application" {
    # Determine the install directory path based on rootless/root
    local install_dir=$(get_quadlet_install_dir)

    # Generate random names for parallelism
    local app_name="webapp_$(random_string)"
    local container_name="webserver_$(random_string)"
    local volume_name="appstorage_$(random_string)"
    local network_name="appnetwork_$(random_string)"

    # Create a multi-quadlet file with additional systemd sections
    local multi_quadlet_file=$PODMAN_TMPDIR/${app_name}.quadlets
    cat > $multi_quadlet_file <<EOF
# FileName=$container_name
# Web application stack
[Unit]
Description=Web server container for application
After=network.target
Wants=network.target

[Container]
Image=$IMAGE
ContainerName=web-server-$(random_string)
PublishPort=8080:80
Environment=APP_ENV=production

[Service]
Restart=always
TimeoutStartSec=900

[Install]
WantedBy=multi-user.target

---

# FileName=$volume_name
# Database volume
[Unit]
Description=Database storage volume
Documentation=https://example.com/storage-docs

[Volume]
Label=app=$app_name
Label=component=database
Driver=local

[Install]
WantedBy=multi-user.target

---

# FileName=$network_name
# Application network
[Unit]
Description=Application network for web services

[Network]
Subnet=10.0.0.0/24
Gateway=10.0.0.1
Label=app=$app_name

[Install]
WantedBy=multi-user.target
EOF

    # Test quadlet install with multi-quadlet file
    run_podman quadlet install --application=$app_name $multi_quadlet_file

    # Verify install output contains all three quadlet names
    assert "$output" =~ "${container_name}.container" "install output should contain ${container_name}.container"
    assert "$output" =~ "${volume_name}.volume" "install output should contain ${volume_name}.volume"
    assert "$output" =~ "${network_name}.network" "install output should contain ${network_name}.network"

    # Count lines in output (should be 3 lines, one for each quadlet)
    assert "${#lines[@]}" -eq 3 "install output should contain exactly three lines"

    # Test quadlet list to verify all quadlets were installed
    run_podman quadlet list
    assert "$output" =~ "${container_name}.container" "list should contain ${container_name}.container"
    assert "$output" =~ "${volume_name}.volume" "list should contain ${volume_name}.volume"
    assert "$output" =~ "${network_name}.network" "list should contain ${network_name}.network"

    # Verify the files exist on disk
    [[ -f "$install_dir/$app_name/${container_name}.container" ]] || die "${container_name}.container should exist on disk"
    [[ -f "$install_dir/$app_name/${volume_name}.volume" ]] || die "${volume_name}.volume should exist on disk"
    [[ -f "$install_dir/$app_name/${network_name}.network" ]] || die "${network_name}.network should exist on disk"

    # Test quadlet print for each installed quadlet and verify systemd sections are preserved
    run_podman quadlet print ${container_name}.container
    assert "$output" =~ "\\[Unit\\]" "print should show Unit section"
    assert "$output" =~ "Description=Web server container" "print should show Unit description"
    assert "$output" =~ "After=network.target" "print should show After directive"
    assert "$output" =~ "Wants=network.target" "print should show Wants directive"
    assert "$output" =~ "\\[Container\\]" "print should show container section"
    assert "$output" =~ "Image=$IMAGE" "print should show correct image"
    assert "$output" =~ "Environment=APP_ENV=production" "print should show environment variable"
    assert "$output" =~ "\\[Service\\]" "print should show Service section"
    assert "$output" =~ "Restart=always" "print should show Restart directive"
    assert "$output" =~ "TimeoutStartSec=900" "print should show TimeoutStartSec directive"
    assert "$output" =~ "\\[Install\\]" "print should show Install section"
    assert "$output" =~ "WantedBy=multi-user.target" "print should show WantedBy directive"

    run_podman quadlet print ${volume_name}.volume
    assert "$output" =~ "\\[Unit\\]" "print should show Unit section"
    assert "$output" =~ "Description=Database storage volume" "print should show Unit description"
    assert "$output" =~ "Documentation=https://example.com/storage-docs" "print should show Documentation directive"
    assert "$output" =~ "\\[Volume\\]" "print should show volume section"
    assert "$output" =~ "Label=app=$app_name" "print should show app label"
    assert "$output" =~ "Driver=local" "print should show Driver directive"
    assert "$output" =~ "\\[Install\\]" "print should show Install section"
    assert "$output" =~ "WantedBy=multi-user.target" "print should show WantedBy directive"

    run_podman quadlet print ${network_name}.network
    assert "$output" =~ "\\[Unit\\]" "print should show Unit section"
    assert "$output" =~ "Description=Application network" "print should show Unit description"
    assert "$output" =~ "\\[Network\\]" "print should show network section"
    assert "$output" =~ "Subnet=10.0.0.0/24" "print should show subnet"
    assert "$output" =~ "\\[Install\\]" "print should show Install section"
    assert "$output" =~ "WantedBy=multi-user.target" "print should show WantedBy directive"

    # Test quadlet list to verify all quadlets belongs the app
    run_podman quadlet list --filter "name=${container_name}" --format "{{.Name}}/{{.App}}"
    assert "$output" == "${container_name}.container/${app_name}" "${container_name} belongs to app ${app_name}"
    run_podman quadlet list --filter "name=${volume_name}" --format "{{.Name}}/{{.App}}"
    assert "$output" == "${volume_name}.volume/${app_name}" "${volume_name} belongs to app ${app_name}"
    run_podman quadlet list --filter "name=${network_name}" --format "{{.Name}}/{{.App}}"
    assert "$output" == "${network_name}.network/${app_name}" "${network_name} belongs to app ${app_name}"

    # Test quadlet rm for application
    run_podman quadlet rm --recursive $app_name
    assert "$output" =~ "${container_name}.container" "remove output should contain ${container_name}.container"

    # Verify all quadlets were removed since they're part of the same app
    run_podman quadlet list
    assert "$output" !~ "${container_name}.container" "list should not contain removed ${container_name}.container"
    assert "$output" !~ "${volume_name}.volume" "list should not contain ${volume_name}.volume as app is removed"
    assert "$output" !~ "${network_name}.network" "list should not contain ${network_name}.network as app is removed"
}

@test "quadlet verb - install multi-quadlet file with empty sections" {
    # Test handling of empty sections between separators
    local container_name="testcontainer_$(random_string)"
    local volume_name="testvolume_$(random_string)"
    local multi_quadlet_file=$PODMAN_TMPDIR/with-empty_$(random_string).quadlets
    cat > $multi_quadlet_file <<EOF
# FileName=$container_name
[Container]
Image=$IMAGE
ContainerName=test-container-$(random_string)

---

---

# FileName=$volume_name
[Volume]
Label=test=value

---

EOF

    # Test quadlet install
    run_podman quadlet install $multi_quadlet_file

    # Should only install 2 quadlets (empty sections should be skipped)
    assert "$output" =~ "${container_name}.container" "install output should contain ${container_name}.container"
    assert "$output" =~ "${volume_name}.volume" "install output should contain ${volume_name}.volume"
    assert "${#lines[@]}" -eq 2 "install output should contain exactly two lines"

    # Test quadlet list to verify quadlets don't belong to an app
    run_podman quadlet list --filter "name=${container_name}" --format "{{.Name}}/{{.App}}"
    assert "$output" == "${container_name}.container/" "${container_name}.container shouldn't belong to an app"
    run_podman quadlet list --filter "name=${volume_name}" --format "{{.Name}}/{{.App}}"
    assert "$output" == "${volume_name}.volume/" "${volume_name}.container shouldn't belong to an app"

    # Clean up
    run_podman quadlet rm ${container_name}.container ${volume_name}.volume
}

@test "quadlet verb - install multi-quadlet file missing FileName" {
    # Test error handling when FileName is missing in multi-quadlet file
    local volume_name="testvolume_$(random_string)"
    local multi_quadlet_file=$PODMAN_TMPDIR/missing-filename_$(random_string).quadlets
    cat > $multi_quadlet_file <<EOF
[Container]
Image=$IMAGE
ContainerName=test-container-$(random_string)

---

# FileName=$volume_name
[Volume]
Label=test=value
EOF

    # Test quadlet install should fail
    run_podman 125 quadlet install $multi_quadlet_file
    assert "$output" =~ "missing required.*FileName" "error should mention missing FileName"
}

@test "quadlet verb - install single-section .quadlets file missing FileName" {
    # Test error handling when FileName is missing in a .quadlets file with only one section
    local multi_quadlet_file=$PODMAN_TMPDIR/single-missing-filename_$(random_string).quadlets
    cat > $multi_quadlet_file <<EOF
[Container]
Image=$IMAGE
ContainerName=test-container-$(random_string)
EOF

    # Test quadlet install should fail
    run_podman 125 quadlet install $multi_quadlet_file
    assert "$output" =~ "missing required.*FileName" "error should mention missing FileName"
}

@test "quadlet verb - install directory with mixed individual and .quadlets files" {
    # Test installing from a directory containing both individual quadlet files and .quadlets files
    local install_dir=$(get_quadlet_install_dir)
    local app_name="mixed-app_$(random_string)"
    local app_dir=$PODMAN_TMPDIR/$app_name
    mkdir -p "$app_dir"

    # Generate random names for all components
    local frontend_name="frontend_$(random_string)"
    local data_name="data_$(random_string)"
    local api_name="api-server_$(random_string)"
    local cache_name="cache_$(random_string)"
    local network_name="app-network_$(random_string)"
    local nested_server_name="nested-server_$(random_string)"

    # Create an individual container quadlet file
    cat > "$app_dir/${frontend_name}.container" <<EOF
[Container]
Image=$IMAGE
ContainerName=frontend-app-$(random_string)
PublishPort=3000:3000
EOF

    # Create an individual volume quadlet file
    cat > "$app_dir/${data_name}.volume" <<EOF
[Volume]
Label=app=$app_name
Label=component=storage
EOF

    # Create a .quadlets file with multiple quadlets
    cat > "$app_dir/backend_$(random_string).quadlets" <<EOF
# FileName=$api_name
[Container]
Image=$IMAGE
ContainerName=api-server-$(random_string)
PublishPort=8080:8080

---

# FileName=$cache_name
[Volume]
Label=app=$app_name
Label=component=cache

---

# FileName=$network_name
[Network]
Subnet=192.168.1.0/24
Gateway=192.168.1.1
Label=app=$app_name
EOF

    # Create a non-quadlet asset file (config file)
    cat > "$app_dir/app.conf" <<EOF
# Application configuration
debug=true
port=3000
EOF

    # Create a nested .quadlets file
    mkdir "$app_dir/a"
    cat > "$app_dir/a/backend_$(random_string).quadlets" <<EOF
# FileName=$nested_server_name
[Container]
Image=$IMAGE
ContainerName=nested-server-$(random_string)
PublishPort=8080:8080
EOF


    # Install the directory
    run_podman quadlet install "$app_dir" --application=$app_name

    # Verify all quadlets were installed (2 individual + 3 from .quadlets file + 1 from nested .quadlet file = 6 total)
    assert "$output" =~ "${frontend_name}.container" "install output should contain ${frontend_name}.container"
    assert "$output" =~ "${data_name}.volume" "install output should contain ${data_name}.volume"
    assert "$output" =~ "${api_name}.container" "install output should contain ${api_name}.container"
    assert "$output" =~ "${cache_name}.volume" "install output should contain ${cache_name}.volume"
    assert "$output" =~ "${network_name}.network" "install output should contain ${network_name}.network"
    assert "$output" =~ "${nested_server_name}.container" "install output should contain ${nested_server_name}.container"

    # Count lines in output (should be 7 lines: 6 quadlets + 1 asset file)
    assert "${#lines[@]}" -eq 7 "install output should contain exactly seven lines"

    # Verify all files exist on disk
    [[ -f "$install_dir/$app_name/${frontend_name}.container" ]] || die "${frontend_name}.container should exist on disk"
    [[ -f "$install_dir/$app_name/${data_name}.volume" ]] || die "${data_name}.volume should exist on disk"
    [[ -f "$install_dir/$app_name/${api_name}.container" ]] || die "${api_name}.container should exist on disk"
    [[ -f "$install_dir/$app_name/${cache_name}.volume" ]] || die "${cache_name}.volume should exist on disk"
    [[ -f "$install_dir/$app_name/${network_name}.network" ]] || die "${network_name}.network should exist on disk"
    [[ -f "$install_dir/$app_name/app.conf" ]] || die "app.conf should exist on disk"
    [[ -f "$install_dir/$app_name/a/${nested_server_name}.container" ]] || die "a/${nested_server_name}.container should exist on disk"

    # Test quadlet list to verify all quadlets show the same app name
    run_podman quadlet list
    local frontend_line=$(echo "$output" | grep "${frontend_name}.container")
    local data_line=$(echo "$output" | grep "${data_name}.volume")
    local api_line=$(echo "$output" | grep "${api_name}.container")
    local cache_line=$(echo "$output" | grep "${cache_name}.volume")
    local network_line=$(echo "$output" | grep "${network_name}.network")
    local nested_server_line=$(echo "$output" | grep "${nested_server_name}.container")

    # Verify content of individual quadlet files
    run cat "$install_dir/$app_name/${frontend_name}.container"
    assert "$output" =~ "\\[Container\\]" "frontend container file should contain [Container] section"
    assert "$output" =~ "ContainerName=frontend-app-" "frontend container file should contain correct name prefix"

    run cat "$install_dir/$app_name/${api_name}.container"
    assert "$output" =~ "\\[Container\\]" "api-server container file should contain [Container] section"
    assert "$output" =~ "ContainerName=api-server-" "api-server container file should contain correct name prefix"

    run cat "$install_dir/$app_name/${network_name}.network"
    assert "$output" =~ "\\[Network\\]" "network file should contain [Network] section"
    assert "$output" =~ "Subnet=192.168.1.0/24" "network file should contain correct subnet"

    run cat "$install_dir/$app_name/a/${nested_server_name}.container"
    assert "$output" =~ "\\[Container\\]" "nested-server container file should contain [Container] section"
    assert "$output" =~ "ContainerName=nested-server-" "nested-server container file should contain correct name prefix"

    # Test that removing one quadlet removes the entire application
    run_podman quadlet rm $app_name --recursive

    # All quadlets should be removed since they're part of the same app
    run_podman quadlet list
    assert "$output" !~ "${frontend_name}.container" "${frontend_name}.container should be removed"
    assert "$output" !~ "${data_name}.volume" "${data_name}.volume should also be removed as part of same app"
    assert "$output" !~ "${api_name}.container" "${api_name}.container should also be removed as part of same app"
    assert "$output" !~ "${cache_name}.volume" "${cache_name}.volume should also be removed as part of same app"
    assert "$output" !~ "${network_name}.network" "${network_name}.network should also be removed as part of same app"
    assert "$output" !~ "${nested_server_name}.container" "${nested_server_name}.container should also be removed as part of same app"

    # All individual files should be removed
    [[ ! -f "$install_dir/$app_name/${frontend_name}.container" ]] || die "${frontend_name}.container should be removed"
    [[ ! -f "$install_dir/$app_name/${data_name}.volume" ]] || die "${data_name}.volume should be removed"
    [[ ! -f "$install_dir/$app_name/${api_name}.container" ]] || die "${api_name}.container should be removed"
    [[ ! -f "$install_dir/$app_name/${cache_name}.volume" ]] || die "${cache_name}.volume should be removed"
    [[ ! -f "$install_dir/$app_name/${network_name}.network" ]] || die "${network_name}.network should be removed"
    [[ ! -f "$install_dir/$app_name/app.conf" ]] || die "app.conf should be removed"
    [[ ! -f "$install_dir/$app_name/a/${nested_server_name}.container" ]] || die "${nested_server_name}.container should be removed"
}
