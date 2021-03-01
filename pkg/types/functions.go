package types

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
