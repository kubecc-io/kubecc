/*
Copyright 2021 The Kubecc Authors.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package util

import (
	"encoding/hex"

	md5simd "github.com/minio/md5-simd"
)

// Hashable represents an object which can be hashed using a md5simd.Hasher.
type Hashable interface {
	Hash(md5simd.Hasher)
}

// HashServer uses the md5-simd library to allow highly concurrent hashing.
type HashServer struct {
	srv md5simd.Server
}

// NewHashServer creates a new hash server.
func NewHashServer() *HashServer {
	return &HashServer{
		srv: md5simd.NewServer(),
	}
}

// Hash hashes the given object using the hash server. This can and should
// be called simultaneously from multiple goroutines for maximum performance.
func (hs *HashServer) Hash(obj Hashable) string {
	hasher := hs.srv.NewHash()
	defer hasher.Close()
	obj.Hash(hasher)
	return hex.EncodeToString(hasher.Sum(nil))
}
