//go:build linux || freebsd

package integration

import (
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman artifact created timestamp", func() {
	createArtifactFile := func(size int) (string, error) {
		artifactFile := filepath.Join(podmanTest.TempDir, RandomString(12))
		f, err := os.Create(artifactFile)
		if err != nil {
			return "", err
		}
		defer f.Close()

		data := RandomString(size)
		_, err = f.WriteString(data)
		if err != nil {
			return "", err
		}
		return artifactFile, nil
	}

	It("podman artifact inspect shows created date in RFC3339 format", func() {
		artifactFile, err := createArtifactFile(1024)
		Expect(err).ToNot(HaveOccurred())

		artifactName := "localhost/test/artifact-created"

		// Record time before creation (with some buffer for slow systems)
		beforeCreate := time.Now().UTC().Add(-time.Second)

		// Add artifact
		podmanTest.PodmanExitCleanly("artifact", "add", artifactName, artifactFile)

		// Record time after creation
		afterCreate := time.Now().UTC().Add(time.Second)

		// Inspect artifact
		a := podmanTest.InspectArtifact(artifactName)
		Expect(a.Name).To(Equal(artifactName + ":latest"))

		// Check that created annotation exists and is in valid RFC3339 format
		createdStr, exists := a.Manifest.Annotations["org.opencontainers.image.created"]
		Expect(exists).To(BeTrue(), "Should have org.opencontainers.image.created annotation")

		// Parse the created timestamp as RFC3339Nano
		createdTime, err := time.Parse(time.RFC3339Nano, createdStr)
		Expect(err).ToNot(HaveOccurred(), "Created timestamp should be valid RFC3339Nano format")

		// Verify timestamp is reasonable (within our time window)
		Expect(createdTime).To(BeTemporally(">=", beforeCreate))
		Expect(createdTime).To(BeTemporally("<=", afterCreate))
	})

	It("podman artifact append preserves original created date", func() {
		artifactFile1, err := createArtifactFile(1024)
		Expect(err).ToNot(HaveOccurred())
		artifactFile2, err := createArtifactFile(2048)
		Expect(err).ToNot(HaveOccurred())

		artifactName := "localhost/test/artifact-append"

		// Add initial artifact
		podmanTest.PodmanExitCleanly("artifact", "add", artifactName, artifactFile1)

		// Get initial created timestamp
		a := podmanTest.InspectArtifact(artifactName)
		originalCreated := a.Manifest.Annotations["org.opencontainers.image.created"]
		Expect(originalCreated).ToNot(BeEmpty())

		// Wait a moment to ensure timestamps would be different
		time.Sleep(100 * time.Millisecond)

		// Append to the artifact
		podmanTest.PodmanExitCleanly("artifact", "add", "--append", artifactName, artifactFile2)

		// Check that created timestamp is unchanged
		a = podmanTest.InspectArtifact(artifactName)
		currentCreated := a.Manifest.Annotations["org.opencontainers.image.created"]
		Expect(currentCreated).To(Equal(originalCreated), "Created timestamp should not change when appending")

		// Verify we have 2 layers
		Expect(a.Manifest.Layers).To(HaveLen(2))
	})
})
