package util

import md5simd "github.com/minio/md5-simd"

type Hashable interface {
	Hash(md5simd.Hasher)
}

type HashServer struct {
	srv md5simd.Server
}

func NewHashServer() *HashServer {
	return &HashServer{
		srv: md5simd.NewServer(),
	}
}

func (hs *HashServer) Hash(obj Hashable) string {
	hasher := hs.srv.NewHash()
	defer hasher.Close()
	obj.Hash(hasher)
	return string(hasher.Sum(nil))
}
