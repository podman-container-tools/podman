package e2e_test

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var _ = Describe("podman machine list", func(ctx context.Context) {
	It("list machine", func() {
		list := new(listMachine)
		firstList, err := mb.setCmd(ctx, list).run(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(firstList).Should(Exit(0))
		Expect(firstList.outputToStringSlice()).To(HaveLen(1)) // just the header

		i := new(initMachine)
		session, err := mb.setCmd(ctx, i.withFakeImage(mb)).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		secondList, err := mb.setCmd(ctx, list).run(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(secondList).To(Exit(0))
		Expect(secondList.outputToStringSlice()).To(HaveLen(2)) // one machine and the header
	})

	It("list machines with quiet or noheading", func(ctx context.Context) {
		// Random names for machines to test list
		name1 := randomString()
		name2 := randomString()

		list := new(listMachine)
		firstList, err := mb.setCmd(ctx, list.withQuiet()).run(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(firstList).Should(Exit(0))
		Expect(firstList.outputToStringSlice()).To(BeEmpty()) // No header with quiet

		noheaderSession, err := mb.setCmd(ctx, list.withNoHeading()).run(ctx) // noheader
		Expect(err).NotTo(HaveOccurred())
		Expect(noheaderSession).Should(Exit(0))
		Expect(noheaderSession.outputToStringSlice()).To(BeEmpty())

		i := new(initMachine)
		session, err := mb.setName(name1).setCmd(ctx, i.withFakeImage(mb)).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		session2, err := mb.setName(name2).setCmd(ctx, i.withFakeImage(mb)).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(session2).To(Exit(0))

		secondList, err := mb.setCmd(ctx, list.withQuiet()).run(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(secondList).To(Exit(0))
		Expect(secondList.outputToStringSlice()).To(HaveLen(2)) // two machines, no header

		listNames := secondList.outputToStringSlice()
		stripAsterisk(listNames)
		Expect(slices.Contains(listNames, name1)).To(BeTrue())
		Expect(slices.Contains(listNames, name2)).To(BeTrue())
	})

	It("list machine: check if running while starting", func(ctx context.Context) {
		skipIfWSL("the below logic does not work on WSL.  #20978")
		i := new(initMachine)
		session, err := mb.setCmd(ctx, i.withImage(mb.imagePath)).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		l := new(listMachine)
		listSession, err := mb.setCmd(ctx, l.withFormat("{{.LastUp}}")).run(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(listSession).To(Exit(0))
		Expect(listSession.outputToString()).To(Equal("Never"))

		// The logic in this test stanza is seemingly invalid on WSL.
		// issue #20978 reflects this change
		s := new(startMachine)
		startSession, err := mb.setCmd(ctx, s).runWithoutWait(ctx)
		Expect(err).ToNot(HaveOccurred())
		wait := 3
		retries := int(mb.timeout/time.Second) / wait
		for range retries {
			listSession, err := mb.setCmd(ctx, l).run(ctx)
			Expect(listSession).To(Exit(0))
			Expect(err).ToNot(HaveOccurred())
			if startSession.ExitCode() == -1 {
				Expect(listSession.outputToString()).NotTo(ContainSubstring("Currently running"))
			} else {
				break
			}
			time.Sleep(time.Duration(wait) * time.Second)
		}
		Expect(startSession).To(Exit(0))
		listSession, err = mb.setCmd(ctx, l).run(ctx)
		Expect(listSession).To(Exit(0))
		Expect(err).ToNot(HaveOccurred())
		Expect(listSession.outputToString()).To(ContainSubstring("Currently running"))
		Expect(listSession.outputToString()).NotTo(ContainSubstring("Less than a second ago")) // check to make sure time created is accurate
	})

	It("list with --format", func(ctx context.Context) {
		// Random names for machines to test list
		name1 := randomString()

		i := new(initMachine)
		session, err := mb.setName(name1).setCmd(ctx, i.withFakeImage(mb)).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		// go format
		list := new(listMachine)
		listSession, err := mb.setCmd(ctx, list.withFormat("{{.Name}}")).run(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(listSession).To(Exit(0))
		Expect(listSession.outputToStringSlice()).To(HaveLen(1))

		listNames := listSession.outputToStringSlice()
		stripAsterisk(listNames)
		Expect(slices.Contains(listNames, name1)).To(BeTrue())

		// --format json
		list2 := new(listMachine)
		list2 = list2.withFormat("json")
		listSession2, err := mb.setCmd(ctx, list2).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(listSession2).To(Exit(0))
		Expect(listSession2.outputToString()).To(BeValidJSON())

		var listResponse []*entities.ListReporter
		err = jsoniter.Unmarshal(listSession2.Bytes(), &listResponse)
		Expect(err).ToNot(HaveOccurred())

		// table format includes the header
		list = new(listMachine)
		listSession3, err3 := mb.setCmd(ctx, list.withFormat("table {{.Name}}")).run(ctx)
		Expect(err3).NotTo(HaveOccurred())
		Expect(listSession3).To(Exit(0))
		listNames3 := listSession3.outputToStringSlice()
		Expect(listNames3).To(HaveLen(2))
	})
	It("list machine in machine-readable byte format", func(ctx context.Context) {
		i := new(initMachine)
		session, err := mb.setCmd(ctx, i.withFakeImage(mb)).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		list := new(listMachine)
		list = list.withFormat("json")
		listSession, err := mb.setCmd(ctx, list).run(ctx)
		Expect(err).NotTo(HaveOccurred())
		var listResponse []*entities.ListReporter
		err = jsoniter.Unmarshal(listSession.Bytes(), &listResponse)
		Expect(err).NotTo(HaveOccurred())
		for _, reporter := range listResponse {
			memory, err := strconv.Atoi(reporter.Memory)
			Expect(err).NotTo(HaveOccurred())
			Expect(memory).To(BeNumerically(">", 2000000000)) // 2GiB
			diskSize, err := strconv.Atoi(reporter.DiskSize)
			Expect(err).NotTo(HaveOccurred())
			Expect(diskSize).To(BeNumerically(">", 11000000000)) // 11GiB
		}
	})
	It("list machine in human-readable format", func(ctx context.Context) {
		i := new(initMachine)
		session, err := mb.setCmd(ctx, i.withFakeImage(mb)).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		list := new(listMachine)
		listSession, err := mb.setCmd(ctx, list.withFormat("{{.Memory}} {{.DiskSize}}")).run(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(listSession).To(Exit(0))
		Expect(listSession.outputToString()).To(Equal("2GiB 11GiB"))
	})
})

func stripAsterisk(sl []string) {
	for idx, val := range sl {
		sl[idx] = strings.TrimRight(val, "*")
	}
}
