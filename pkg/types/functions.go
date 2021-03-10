package types

import (
	"sort"

	md5simd "github.com/minio/md5-simd"
)

// EquivalentTo Compares all fields except executable
// The binaries can live in different locations.
func (tc *Toolchain) EquivalentTo(other *Toolchain) bool {
	return tc.GetKind() == other.GetKind() &&
		tc.GetLang() == other.GetLang() &&
		tc.GetTargetArch() == other.GetTargetArch() &&
		tc.GetPicDefault() == other.GetPicDefault() &&
		tc.GetVersion() == other.GetVersion()
}

func (k *Key) Canonical() string {
	return k.Bucket + "." + k.Name
}

func (k *Key) ShortID() string {
	return FormatShortID(k.Bucket, 6, ElideCenter) + "." + k.Name
}

func (tc *Toolchain) Hash(hasher md5simd.Hasher) {
	hasher.Write([]byte(tc.TargetArch))
	hasher.Write([]byte(ToolchainKind_name[int32(tc.Kind)]))
	hasher.Write([]byte(ToolchainLang_name[int32(tc.Kind)]))
	hasher.Write([]byte(tc.Version))
	if tc.PicDefault {
		hasher.Write([]byte{1})
	} else {
		hasher.Write([]byte{0})
	}
}

func (req *CompileRequest) Hash(hasher md5simd.Hasher) {
	req.Toolchain.Hash(hasher)
	hasher.Write(req.PreprocessedSource)
	sortedArgs := append([]string{}, req.Args...)
	sort.Sort(sort.StringSlice(sortedArgs))
	for _, arg := range sortedArgs {
		hasher.Write([]byte(arg))
	}
}
