package editor

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CompareVersions", func() {
	It("should compare versions correctly", func() {
		Expect(CompareVersions("2019.1.3f0", "2019.1.3f0")).To(Equal(0))
		Expect(CompareVersions("2019.1.3f0", "2019.1.3b20")).To(BeNumerically(">", 0))
		Expect(CompareVersions("2019.1.3a0", "2019.1.3b20")).To(BeNumerically("<", 0))
		Expect(CompareVersions("2019.4.9f1", "2019.4.14f1")).To(BeNumerically("<", 0))
	})
})