#!/usr/bin/env bats   -*- bats -*-
#
# Tests for podman artifact created date functionality
#

load helpers

# FIXME #27264: Artifact store does not seem to work properly with concurrent access. Do not the ci:parallel tags here!

# Create temporary artifact file for testing
function create_test_file() {
    local content="$(random_string 100)"
    local filename=$(random_string 12)
    local filepath="$PODMAN_TMPDIR/$filename.txt"
    echo "$content" > "$filepath"
    echo "$filepath"
}

function teardown() {
    run_podman artifact rm --all --ignore
    basic_teardown
}

@test "podman artifact inspect shows created date in RFC3339 format" {
    local testfile1=$(create_test_file)
    local artifact_name="localhost/test/created-test"
    local testfile2=$(create_test_file)

    # Record time before creation (in nanoseconds for comparison)
    local before_epoch=$(date +%s%N)

    # Create artifact
    run_podman artifact add $artifact_name "$testfile1"

    # Record time after creation
    local after_epoch=$(date +%s%N)

    # Inspect the artifact
    run_podman artifact inspect --format '{{index  .Manifest.Annotations "org.opencontainers.image.created" }}' $artifact_name
    local created_annotation="$output"
    assert "$created_annotation" != "" "Created annotation should not be empty"

    # Verify it's a valid RFC3339 timestamp by trying to parse it
    # Convert to epoch for comparison
    local created_epoch=$(date -d "$created_annotation" +%s%N 2>/dev/null)

    # Verify parsing succeeded
    assert "$?" -eq 0 "Created timestamp should be valid RFC3339 format"

    # Verify timestamp is within reasonable bounds
    assert "$created_epoch" -ge "$before_epoch" "Created time should be after before_epoch"
    assert "$created_epoch" -le "$after_epoch" "Created time should be before after_epoch"

    # Append to artifact
    run_podman artifact add --append $artifact_name "$testfile2"

    # Get the created timestamp after append
    run_podman artifact inspect --format '{{index  .Manifest.Annotations "org.opencontainers.image.created" }}\n{{len .Manifest.Layers}}' $artifact_name
    local current_created="${lines[0]}"
    local layer_count="${lines[1]}"

    # Verify the created timestamp is preserved
    assert "$current_created" = "$created_annotation" "Created timestamp should be preserved during append"

    # Verify we have 2 layers now
    assert "$layer_count" -eq 2 "Should have 2 layers after append"

    run_podman artifact rm "$artifact_name"
}


@test "podman artifact add --replace basic functionality" {
    local artifact_name="localhost/test/replace-artifact"
    local file1 file2

    file1=$(create_test_file)
    file2=$(create_test_file)

    # Add initial artifact
    run_podman artifact add "$artifact_name" "$file1"
    local first_digest="$output"

    # Verify initial artifact exists
    run_podman artifact inspect "$artifact_name"

    # Replace with different file
    run_podman artifact add --replace "$artifact_name" "$file2"
    local second_digest="$output"

    # Verify artifact was replaced (different digest)
    assert "$first_digest" != "$second_digest" "Replace should create different digest"

    # Verify artifact still exists and is accessible
    run_podman artifact inspect "$artifact_name"

    # Cleanup
    run_podman artifact rm "$artifact_name"
}

@test "podman artifact add --replace nonexistent artifact" {
    local artifact_name="localhost/test/nonexistent-artifact"
    local file1

    file1=$(create_test_file)

    # Using --replace on nonexistent artifact should succeed
    run_podman artifact add --replace "$artifact_name" "$file1"

    # Verify artifact was created
    run_podman artifact inspect "$artifact_name"

    # Cleanup
    run_podman artifact rm "$artifact_name"
}

@test "podman artifact add --replace and --append conflict" {
    local artifact_name="localhost/test/conflict-artifact"
    local file1

    file1=$(create_test_file)

    # Using --replace and --append together should fail
    run_podman 125 artifact add --replace --append "$artifact_name" "$file1"
    assert "$output" =~ "--append and --replace options cannot be used together"
}

@test "podman artifact add --replace with existing artifact" {
    local artifact_name="localhost/test/existing-artifact"
    local file1 file2

    file1=$(create_test_file)
    file2=$(create_test_file)

    # Create initial artifact
    run_podman artifact add "$artifact_name" "$file1"

    # Verify initial artifact exists
    run_podman artifact inspect "$artifact_name"

    # Adding same name without --replace should fail
    run_podman 125 artifact add "$artifact_name" "$file2"
    assert "$output" =~ "artifact already exists"

    # Replace should succeed
    run_podman artifact add --replace "$artifact_name" "$file2"

    # Verify artifact was replaced
    run_podman artifact inspect "$artifact_name"

    # Cleanup
    run_podman artifact rm "$artifact_name"
}

@test "podman artifact volume validation at creation" {
    # Issue #27747: Artifact volume validation should fail at creation, not start
    local artifact_name="localhost/test/nonexistent-artifact"

    # Creation should fail if the artifact does not exist
    run_podman 125 create --name test-artifact-fail --mount type=artifact,source=$artifact_name,target=/tmp $IMAGE
    assert "$output" = "Error: $artifact_name:latest: artifact does not exist" "creation should fail for nonexistent artifact"
}


# vim: filetype=sh
