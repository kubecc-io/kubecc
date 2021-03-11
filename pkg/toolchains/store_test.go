package toolchains_test

import (
	"errors"
	"io/fs"
	"time"

	"github.com/cobalt77/kubecc/pkg/toolchains"
	"github.com/cobalt77/kubecc/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var executable = "/path/to/executable"

type querier struct {
	version    string
	targetArch string
	picDefault bool
	kind       types.ToolchainKind
	lang       types.ToolchainLang
	modTime    time.Time
}

func defaultQuerier() *querier {
	return &querier{
		version:    "0",
		targetArch: "testarch",
		picDefault: true,
		kind:       types.Gnu,
		lang:       types.CXX,
		modTime:    time.Now(),
	}
}

func (q *querier) Version(compiler string) (string, error) {
	if q.version == "" {
		return "", errors.New("test error")
	}
	return q.version, nil
}

func (q *querier) TargetArch(compiler string) (string, error) {
	if q.targetArch == "" {
		return "", errors.New("test error")
	}
	return q.targetArch, nil
}

func (q *querier) IsPicDefault(compiler string) (bool, error) {
	return q.picDefault, nil
}

func (q *querier) Kind(compiler string) (types.ToolchainKind, error) {
	return q.kind, nil
}

func (q *querier) Lang(compiler string) (types.ToolchainLang, error) {
	return q.lang, nil
}

func (q *querier) ModTime(compiler string) (time.Time, error) {
	if q.modTime == time.Unix(0, 0) {
		return time.Time{}, &fs.PathError{}
	}
	return q.modTime, nil
}

var _ = Describe("Store", func() {
	var store *toolchains.Store
	q := defaultQuerier()

	When("creating a new store", func() {
		It("should be empty", func() {
			store = toolchains.NewStore()
			Expect(store.Len()).To(Equal(0))
			Expect(store.Items()).NotTo(Receive())
			Expect(store.ItemsList()).To(HaveLen(0))
		})
	})

	When("adding a toolchain", func() {
		It("should be stored", func() {
			tc, err := store.Add(executable, q)
			Expect(err).NotTo(HaveOccurred())
			Expect(tc).NotTo(BeNil())
		})
		It("can be searched for", func() {
			Expect(store.Contains(executable)).To(BeTrue())
			items := store.Items()
			Expect(items).To(Receive(Not(Equal(nil))))
			Expect(items).NotTo(Receive())
			Expect(store.ItemsList()).To(HaveLen(1))
		})
		It("should disallow adding twice", func() {
			tc, err := store.Add(executable, q)
			Expect(err).To(HaveOccurred())
			Expect(tc).To(BeNil())
			Expect(store.Len()).To(Equal(1))
		})
	})
	It("should store multiple toolchains", func() {
		tc, err := store.Add("test1", q)
		Expect(err).NotTo(HaveOccurred())
		Expect(tc).NotTo(BeNil())
		Expect(store.Len()).To(Equal(2))
		tc, err = store.Add("test2", q)
		Expect(err).NotTo(HaveOccurred())
		Expect(tc).NotTo(BeNil())
		Expect(store.Len()).To(Equal(3))
		tc, err = store.Add("test3", q)
		Expect(err).NotTo(HaveOccurred())
		Expect(tc).NotTo(BeNil())
		Expect(store.Len()).To(Equal(4))
	})
	It("should update toolchains", func() {
		By("checking modification time")
		q.modTime = q.modTime.Add(1 * time.Second)
		for tc := range store.Items() {
			err, ok := store.UpdateIfNeeded(tc)
			Expect(err).To(BeNil())
			Expect(ok).To(BeTrue())
		}
		for tc := range store.Items() {
			err, ok := store.UpdateIfNeeded(tc)
			Expect(err).To(BeNil())
			Expect(ok).To(BeFalse())
		}
		By("checking version")
		q.version = ""
		q.modTime = q.modTime.Add(1 * time.Second)
		for tc := range store.Items() {
			err, ok := store.UpdateIfNeeded(tc)
			Expect(err).To(HaveOccurred())
			Expect(ok).To(BeTrue())
		}
		q.version = "0"
		By("deleting toolchains that no longer exist")
		q.modTime = time.Unix(0, 0)
		for tc := range store.Items() {
			err, ok := store.UpdateIfNeeded(tc)
			Expect(err).To(HaveOccurred())
			Expect(ok).To(BeTrue())
		}
		Expect(store.Len()).To(Equal(0))
	})
	It("should merge with another toolchain", func() {
		q := defaultQuerier()
		store1 := toolchains.NewStore()
		store2 := toolchains.NewStore()
		tc1, err := store1.Add("test1", q)
		Expect(err).NotTo(HaveOccurred())
		tc2, err := store2.Add("test2", q)
		Expect(err).NotTo(HaveOccurred())
		tc3, err := store1.Add("test3", q)
		Expect(err).NotTo(HaveOccurred())
		tc4, err := store2.Add("test3", q)
		Expect(err).NotTo(HaveOccurred())
		tc5, err := store2.Add("test4", q)
		Expect(err).NotTo(HaveOccurred())
		Expect(store1.ItemsList()).To(ContainElements(tc1, tc3))
		Expect(store2.ItemsList()).To(ContainElements(tc2, tc4, tc5))
		store1.Merge(store2)
		Expect(store1.ItemsList()).To(ContainElements(tc1, tc2, tc3, tc5))
		Expect(store2.ItemsList()).To(ContainElements(tc2, tc4, tc5))
		store2.Merge(store1)
		Expect(store1.ItemsList()).To(ContainElements(tc1, tc2, tc3, tc5))
		Expect(store2.ItemsList()).To(ContainElements(tc1, tc2, tc4, tc5))
	})
	It("should compute intersections", func() {
		q := defaultQuerier()
		store1 := toolchains.NewStore()
		store2 := toolchains.NewStore()
		tc1, err := store1.Add("test1", q)
		Expect(err).NotTo(HaveOccurred())
		tc2, err := store2.Add("test2", q)
		Expect(err).NotTo(HaveOccurred())
		tc3, err := store1.Add("test3", q)
		Expect(err).NotTo(HaveOccurred())
		tc4, err := store2.Add("test3", q)
		Expect(err).NotTo(HaveOccurred())
		tc5, err := store2.Add("test4", q)
		Expect(err).NotTo(HaveOccurred())

		Expect(store1.Intersection(store2)).To(ContainElements(tc3, tc4))
		store1.Merge(store2)
		Expect(store1.Intersection(store2)).To(ContainElements(tc2, tc3, tc4, tc5))
		store2.Merge(store1)
		Expect(store1.Intersection(store2)).To(ContainElements(tc1, tc2, tc3, tc4, tc5))
	})
})
